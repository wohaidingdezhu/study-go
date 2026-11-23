package order

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"sync"
)

// 为常见业务失败定义固定的 error，HTTP 层可以用 errors.Is 识别并返回对应状态。
var (
	ErrNotFound    = errors.New("not found")
	ErrOutOfStock  = errors.New("insufficient stock")
	ErrKeyConflict = errors.New("idempotency key already used for another request")
)

// json 标签指定传输给客户端的字段名，例如 Go 的 ID 会输出为 JSON 的 id。
type Product struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Stock int    `json:"stock"`
}

type Request struct {
	ProductID int64 `json:"productId"`
	Quantity  int   `json:"quantity"`
}

type Order struct {
	ID        int64 `json:"id"`
	ProductID int64 `json:"productId"`
	Quantity  int   `json:"quantity"`
}

// Store 描述 HTTP 层需要的存储操作。内存版和 PostgreSQL 版提供同样的方法。
// context 会把请求取消和超时传给数据库等外部依赖。
// HTTP 层依赖这组接口，可复用相同路由来切换存储实现，这就是此处的依赖抽象。
type Store interface {
	Ping(context.Context) error
	Product(context.Context, int64) (Product, error)
	Order(context.Context, int64) (Order, error)
	// ListOrders 返回 ID 倒序的一段订单。before 为 0 时从最新订单开始。
	ListOrders(ctx context.Context, maxRows int, before int64) ([]Order, error)
	// Create 返回订单和是否为本次新建。重试命中旧订单时，第二个值是 false。
	Create(context.Context, string, Request) (Order, bool, error)
}

// MemoryStore 是进程内存储，状态随进程退出而丢失。两次独立启动各有自己的库存和锁。
// Mutex 是互斥锁，让访问同一份共享状态的 goroutine 在临界区内依次执行。
// Java 版用同一实例的 synchronized 方法协调并发，可沿 MemoryStore.java 对照。
type MemoryStore struct {
	mu      sync.Mutex
	product Product
	orders  map[int64]Order
	keys    map[string]Order
	nextID  int64
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		product: Product{ID: 1, Name: "Notebook", Stock: 5},
		orders:  make(map[int64]Order),
		keys:    make(map[string]Order),
		nextID:  1,
	}
}

func (s *MemoryStore) Ping(ctx context.Context) error {
	return ctx.Err()
}

func (s *MemoryStore) Product(_ context.Context, id int64) (Product, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id != s.product.ID {
		return Product{}, ErrNotFound
	}
	return s.product, nil
}

func (s *MemoryStore) Order(_ context.Context, id int64) (Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// map 查询的第二个返回值表明键是否存在，避免把“缺失”误认成字段全为零的订单。
	value, ok := s.orders[id]
	if !ok {
		return Order{}, ErrNotFound
	}
	return value, nil
}

func (s *MemoryStore) ListOrders(_ context.Context, maxRows int, before int64) ([]Order, error) {
	// 锁只保护读取共享的 map，排序在取出快照后执行。
	// 这里复制的是由整数构成的 Order 值，解锁后处理自己的切片，缩短持锁时间。
	s.mu.Lock()
	items := make([]Order, 0, len(s.orders))
	for _, value := range s.orders {
		if before == 0 || value.ID < before {
			items = append(items, value)
		}
	}
	s.mu.Unlock()

	slices.SortFunc(items, func(a, b Order) int {
		return cmp.Compare(b.ID, a.ID)
	})
	return items[:min(len(items), maxRows)], nil
}

func (s *MemoryStore) Create(_ context.Context, key string, request Request) (Order, bool, error) {
	// 整段操作持有同一把锁，防止两个请求基于同一份库存或同一个幂等键同时创建订单。
	// 临界区包含“检查后修改”整个过程。只锁住扣减时，两个请求可能已在锁外读到相同库存。
	s.mu.Lock()
	defer s.mu.Unlock()

	// 先找旧订单。请求重试时，即使现在已经卖完，也要返回第一次创建的结果。
	// 同一键的参数必须相同，才能认定是同一业务请求。键相同而数量改变属于冲突。
	if existing, ok := s.keys[key]; ok {
		if existing.ProductID != request.ProductID || existing.Quantity != request.Quantity {
			return Order{}, false, ErrKeyConflict
		}
		return existing, false, nil
	}
	if request.ProductID != s.product.ID {
		return Order{}, false, ErrNotFound
	}
	if s.product.Stock < request.Quantity {
		return Order{}, false, ErrOutOfStock
	}

	value := Order{ID: s.nextID, ProductID: request.ProductID, Quantity: request.Quantity}
	s.product.Stock -= request.Quantity
	s.orders[value.ID] = value
	s.keys[key] = value
	s.nextID++
	return value, true, nil
}
