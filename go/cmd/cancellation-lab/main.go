package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"
)

type result struct {
	stock int
	err   error
}

// readStock 模拟一个可响应取消的依赖，便于先观察超时和收尾，再阅读真实 HTTP 调用。
func readStock(ctx context.Context, mode string) result {
	delay := 20 * time.Millisecond
	if mode == "timeout" {
		delay = 2 * time.Second
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	// select 等待结果可用或取消信号。下游主动响应 ctx，才能在预算耗尽时及时结束。
	select {
	case <-ctx.Done():
		return result{err: ctx.Err()}
	case <-timer.C:
	}
	if err := ctx.Err(); err != nil {
		return result{err: err}
	}
	if mode == "failure" {
		return result{err: errors.New("dependency_failed")}
	}
	return result{stock: 5}
}

func main() {
	mode := flag.String("mode", "success", "success, timeout or failure")
	flag.Parse()
	if (*mode != "success" && *mode != "timeout" && *mode != "failure") || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: cancellation-lab -mode success|timeout|failure")
		os.Exit(2)
	}

	// 预算从创建 context 开始。将同一个 ctx 向后传递，子任务才能共用截止时间。
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	// 一个任务最多交付一个结果。容量为 1，发送者可以先交付，接收者随后读取。
	results := make(chan result, 1)
	finished := make(chan struct{})
	go func() {
		// 结果交付和函数收尾分开观察，由工作 goroutine 负责发送结束信号。
		defer close(finished)
		results <- readStock(ctx, *mode)
	}()

	value := <-results
	switch {
	case errors.Is(value.err, context.DeadlineExceeded):
		fmt.Println("result: timeout")
	case value.err != nil:
		fmt.Println("result:", value.err)
	default:
		fmt.Printf("result: stock=%d\n", value.stock)
	}
	<-finished
	fmt.Println("worker cleanup: finished")
}
