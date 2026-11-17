package order_test

import (
	"context"
	"testing"

	"study-go/order"
)

func BenchmarkMemoryStoreReplay(b *testing.B) {
	ctx := context.Background()
	store := order.NewMemoryStore()
	request := order.Request{ProductID: 1, Quantity: 1}
	first, created, err := store.Create(ctx, "bench-replay", request)
	if err != nil || !created {
		b.Fatalf("unable to seed one order: created=%t, err=%v", created, err)
	}

	// 两组都重放同一笔订单，避免库存变化或订单 Map 不断增长影响对比。
	b.Run("serial", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			replayed, created, err := store.Create(ctx, "bench-replay", request)
			if err != nil || created || replayed != first {
				b.Fatalf("unexpected replay: order=%+v, created=%t, err=%v", replayed, created, err)
			}
		}
	})

	b.Run("parallel", func(b *testing.B) {
		// RunParallel 在多个 goroutine 中分配总共 b.N 次操作，模拟并发访问同一把锁。
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				replayed, created, err := store.Create(ctx, "bench-replay", request)
				if err != nil || created || replayed != first {
					b.Errorf("unexpected replay: order=%+v, created=%t, err=%v", replayed, created, err)
					return
				}
			}
		})
	})
}
