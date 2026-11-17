package main

import (
	"log"
	"os"

	"study-go/order"
)

func main() {
	// 127.0.0.1 只接受本机连接。8080 是 Go 入门服务的端口。
	address := "127.0.0.1:8080"
	if len(os.Args) > 1 {
		address = os.Args[1]
	}
	// 入门先用内存存订单，重启就恢复初始库存。进阶实验用同一个 HTTPServer 接 PostgreSQL。
	server := order.HTTPServer(address, order.NewMemoryStore())
	log.Printf("Go order API: http://%s", address)
	log.Fatal(server.ListenAndServe())
}
