package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"time"
)

var errUnavailable = errors.New("downstream returned HTTP 503")

func main() {
	budget := flag.Duration("budget", time.Second, "total request budget")
	delay := flag.Duration("delay", 20*time.Millisecond, "downstream response delay")
	failFirst := flag.Bool("fail-first", false, "return 503 on the first request")
	flag.Parse()
	if *budget <= 0 || *delay < 0 {
		log.Fatal("budget must be positive and delay must be nonnegative")
	}

	// 用本机 HTTP 测试服务器模拟库存服务，不需要启动订单栈。
	var attempts atomic.Int32
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := attempts.Add(1)
		if *failFirst && attempt == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		// 客户端超时并断开时，服务端可以提前结束仍在等待的工作。
		if err := wait(r.Context(), *delay); err != nil {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"stock":5}`)
	}))
	defer downstream.Close()

	ctx, cancel := context.WithTimeout(context.Background(), *budget)
	defer cancel()
	stock, err := readWithRetry(ctx, downstream.Client(), downstream.URL)
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		fmt.Printf("deadline exceeded, attempts=%d\n", attempts.Load())
	case err != nil:
		log.Fatal(err)
	default:
		fmt.Printf("stock=%d, attempts=%d\n", stock, attempts.Load())
	}
}

func readWithRetry(ctx context.Context, client *http.Client, url string) (int, error) {
	stock, err := readStock(ctx, client, url)
	if !errors.Is(err, errUnavailable) {
		return stock, err
	}
	// 只给 GET 的 503 一次重试。等待和第二次 HTTP 调用都沿用原请求的期限。
	if err := wait(ctx, 50*time.Millisecond); err != nil {
		return 0, err
	}
	return readStock(ctx, client, url)
}

func readStock(ctx context.Context, client *http.Client, url string) (int, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	response, err := client.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusServiceUnavailable {
		return 0, errUnavailable
	}
	if response.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected HTTP status %d", response.StatusCode)
	}
	var result struct {
		Stock int `json:"stock"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return 0, err
	}
	return result.Stock, nil
}

func wait(ctx context.Context, interval time.Duration) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
