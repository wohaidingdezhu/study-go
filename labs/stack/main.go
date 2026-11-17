package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"study-go/order"
)

func main() {
	// 同一个编译产物有两种启动方式。Compose 分别用 api 和 relay 创建两个容器。
	if len(os.Args) != 2 {
		log.Fatal("usage: stack api|relay")
	}
	if err := run(os.Args[1]); err != nil {
		log.Fatal(err)
	}
}

func run(mode string) error {
	// 连接地址由运行环境提供。在 Compose 里，容器可以直接用 postgres 等服务名寻址。
	dbURL := os.Getenv("ORDERS_DATABASE_URL")
	if dbURL == "" {
		return fmt.Errorf("ORDERS_DATABASE_URL is required")
	}
	ctx := context.Background()
	// 连接池复用数据库连接，避免每次 HTTP 请求都重新建连。
	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return err
	}
	defer db.Close()

	switch mode {
	case "api":
		address := os.Getenv("REDIS_ADDR")
		if address == "" {
			return fmt.Errorf("REDIS_ADDR is required")
		}
		// 缓存读写只等很短的时间。Redis 慢或不可用时，请求还能用剩余时间查询数据库。
		cache := redis.NewClient(&redis.Options{
			Addr:                  address,
			DialTimeout:           250 * time.Millisecond,
			ReadTimeout:           250 * time.Millisecond,
			WriteTimeout:          250 * time.Millisecond,
			ContextTimeoutEnabled: true,
			MaxRetries:            -1,
			DialerRetries:         1,
		})
		defer cache.Close()
		listen := os.Getenv("ORDERS_LISTEN")
		if listen == "" {
			listen = "127.0.0.1:8082"
		}
		log.Printf("PostgreSQL-backed order API listening at %s", listen)
		return order.HTTPServer(listen, &orderStore{db, cache}).ListenAndServe()
	case "relay":
		brokerURL := os.Getenv("RABBITMQ_URL")
		if brokerURL == "" {
			return fmt.Errorf("RABBITMQ_URL is required")
		}
		return relay(db, brokerURL)
	default:
		return fmt.Errorf("unknown mode %q; expected api or relay", mode)
	}
}
