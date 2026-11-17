package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"study-go/order"
)

type orderStore struct {
	db    *pgxpool.Pool
	cache *redis.Client
}

func (s *orderStore) Ping(ctx context.Context) error {
	// PostgreSQL 是核心依赖。Redis 故障时，商品查询仍能回源数据库。
	return s.db.Ping(ctx)
}

func productKey(id int64) string {
	return fmt.Sprintf("product:%d", id)
}

func (s *orderStore) Product(ctx context.Context, id int64) (order.Product, error) {
	// Cache-Aside 的读路径。先找 Redis，命中直接返回，未命中或缓存异常时回源 PostgreSQL。
	key := productKey(id)
	requestID := order.RequestID(ctx)
	var product order.Product
	cached, cacheErr := s.cache.Get(ctx, key).Bytes()
	switch {
	case cacheErr == nil && json.Unmarshal(cached, &product) == nil && product.ID == id:
		log.Printf("cache HIT %s request_id=%s", key, requestID)
		return product, nil
	case errors.Is(cacheErr, redis.Nil):
		log.Printf("cache MISS %s request_id=%s", key, requestID)
	default:
		log.Printf("cache BYPASS %s request_id=%s", key, requestID)
	}

	// $1 是参数占位符，Scan 按 SELECT 的列顺序把结果写到变量里。
	err := s.db.QueryRow(ctx, "SELECT id, name, stock FROM products WHERE id = $1", id).
		Scan(&product.ID, &product.Name, &product.Stock)
	if errors.Is(err, pgx.ErrNoRows) {
		return order.Product{}, order.ErrNotFound
	}
	if err != nil {
		return order.Product{}, err
	}

	data, err := json.Marshal(product)
	if err != nil {
		return order.Product{}, err
	}
	// TTL 是这个键的有效期。缓存回填失败也可以返回刚从数据库查到的商品。
	if err := s.cache.Set(ctx, key, data, 20*time.Second).Err(); err != nil {
		log.Printf("cache fill %s request_id=%s: %v", key, requestID, err)
	}
	return product, nil
}

func (s *orderStore) Order(ctx context.Context, id int64) (order.Order, error) {
	var value order.Order
	err := s.db.QueryRow(ctx, "SELECT id, product_id, quantity FROM orders WHERE id = $1", id).
		Scan(&value.ID, &value.ProductID, &value.Quantity)
	if errors.Is(err, pgx.ErrNoRows) {
		return order.Order{}, order.ErrNotFound
	}
	return value, err
}

func (s *orderStore) ListOrders(ctx context.Context, maxRows int, before int64) ([]order.Order, error) {
	query := "SELECT id, product_id, quantity FROM orders"
	args := []any{maxRows}
	if before > 0 {
		// 只拼接固定的 SQL，游标仍通过参数绑定传递。按主键范围继续查询。
		query += " WHERE id < $2"
		args = append(args, before)
	}
	rows, err := s.db.Query(ctx, query+" ORDER BY id DESC LIMIT $1", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]order.Order, 0, maxRows)
	for rows.Next() {
		var value order.Order
		if err := rows.Scan(&value.ID, &value.ProductID, &value.Quantity); err != nil {
			return nil, err
		}
		items = append(items, value)
	}
	return items, rows.Err()
}

func (s *orderStore) Create(ctx context.Context, key string, request order.Request) (order.Order, bool, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return order.Order{}, false, err
	}
	// 提前返回时回滚未提交的写入。已经 Commit 的事务不会再被回滚。
	defer tx.Rollback(ctx)

	// 第一步，在商品存在时尝试插入订单。数据库的 UNIQUE 约束协调跨进程的同键请求。
	value := order.Order{ProductID: request.ProductID, Quantity: request.Quantity}
	err = tx.QueryRow(ctx, `
		INSERT INTO orders (idempotency_key, product_id, quantity)
		SELECT $1, id, $3 FROM products WHERE id = $2
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING id`, key, value.ProductID, value.Quantity).Scan(&value.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		// 插入没有返回 ID，可能是商品不存在，也可能是旧请求已保存。查一次幂等键区分两者。
		var previous order.Order
		err = tx.QueryRow(ctx, "SELECT id, product_id, quantity FROM orders WHERE idempotency_key = $1", key).
			Scan(&previous.ID, &previous.ProductID, &previous.Quantity)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return order.Order{}, false, order.ErrNotFound
		case err != nil:
			return order.Order{}, false, err
		case previous.ProductID != request.ProductID || previous.Quantity != request.Quantity:
			return order.Order{}, false, order.ErrKeyConflict
		default:
			return previous, false, nil
		}
	}
	if err != nil {
		return order.Order{}, false, err
	}

	// 第二步，把库存检查和扣减放在同一条 UPDATE 中。并发更新会在数据库里协调。
	// 修改 0 行表示库存不足，前面的订单插入也会随本次事务回滚。
	updated, err := tx.Exec(ctx, `
		UPDATE products SET stock = stock - $2
		WHERE id = $1 AND stock >= $2`, value.ProductID, value.Quantity)
	if err != nil {
		return order.Order{}, false, err
	}
	if updated.RowsAffected() == 0 {
		return order.Order{}, false, order.ErrOutOfStock
	}
	// 第三步，把待发布事件和订单一起提交。RabbitMQ 停机时，relay 恢复后仍可找到这条记录。
	if _, err := tx.Exec(ctx, "INSERT INTO outbox (order_id) VALUES ($1)", value.ID); err != nil {
		return order.Order{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return order.Order{}, false, err
	}

	// 提交后才删除缓存，让下一次查询回填。删除失败时，旧值会按 TTL 过期。
	if err := s.cache.Del(ctx, productKey(value.ProductID)).Err(); err != nil {
		log.Printf("cache invalidation request_id=%s: %v", order.RequestID(ctx), err)
	}
	return value, true, nil
}
