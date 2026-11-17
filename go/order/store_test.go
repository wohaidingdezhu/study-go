package order_test

import (
	"context"
	"errors"
	"testing"

	"study-go/order"
)

func TestCreateReplaysAtZeroStock(t *testing.T) {
	// 准备一份刚初始化的内存存储，每个测试独立拥有 5 件库存。
	ctx := context.Background()
	store := order.NewMemoryStore()
	request := order.Request{ProductID: 1, Quantity: 5}

	first, created, err := store.Create(ctx, "buy-all", request)
	if err != nil || !created {
		t.Fatalf("first request: created=%t, err=%v; want created=true and no error", created, err)
	}

	// 第一次买光库存。相同请求重试时，仍应拿到原来的订单。
	replayed, created, err := store.Create(ctx, "buy-all", request)
	if err != nil || created || replayed != first {
		t.Fatalf("replay: order=%+v, created=%t, err=%v; want order=%+v, created=false and no error",
			replayed, created, err, first)
	}
	product, err := store.Product(ctx, 1)
	if err != nil || product.Stock != 0 {
		t.Fatalf("stock after replay: stock=%d, err=%v; want stock=0 and no error", product.Stock, err)
	}
}

func TestCreateCanRetryAfterOutOfStock(t *testing.T) {
	ctx := context.Background()
	store := order.NewMemoryStore()

	_, created, err := store.Create(ctx, "try-again", order.Request{ProductID: 1, Quantity: 6})
	if created || !errors.Is(err, order.ErrOutOfStock) {
		t.Fatalf("failed order: created=%t, err=%v; want created=false and ErrOutOfStock", created, err)
	}
	product, err := store.Product(ctx, 1)
	if err != nil || product.Stock != 5 {
		t.Fatalf("stock after failure: stock=%d, err=%v; want stock=5 and no error", product.Stock, err)
	}

	// 失败的请求没有占用幂等键。减少数量后，可以用原键创建订单。
	value, created, err := store.Create(ctx, "try-again", order.Request{ProductID: 1, Quantity: 1})
	if err != nil || !created || value.Quantity != 1 {
		t.Fatalf("retry after failure: quantity=%d, created=%t, err=%v; want quantity=1, created=true and no error",
			value.Quantity, created, err)
	}
	product, err = store.Product(ctx, 1)
	if err != nil || product.Stock != 4 {
		t.Fatalf("stock after retry: stock=%d, err=%v; want stock=4 and no error", product.Stock, err)
	}
}
