package main

import (
	"fmt"
	"sync"
)

type Inventory struct {
	mu    sync.Mutex
	stock int
}

// 指针接收者让所有 goroutine 更新同一份库存。锁把检查和扣减放进同一个临界区。
func (inventory *Inventory) reserve() bool {
	inventory.mu.Lock()
	// defer 在函数退出时执行。有库存和库存为零两条路径都会解锁。
	defer inventory.mu.Unlock()
	if inventory.stock == 0 {
		return false
	}
	inventory.stock--
	return true
}

func main() {
	const attempts = 10
	inventory := Inventory{stock: 3}
	// 每个任务发送一次。缓冲区能装下所有结果，所以可以先等待，再统一读取。
	results := make(chan bool, attempts)
	var workers sync.WaitGroup

	for i := 0; i < attempts; i++ {
		// 启动 goroutine 之前登记任务，确保 Wait 知道要等几个。
		workers.Add(1)
		go func() {
			defer workers.Done()
			results <- inventory.reserve()
		}()
	}
	workers.Wait()
	// 所有发送者结束后由主 goroutine 关闭 channel，下面的 range 读完就会退出。
	close(results)

	succeeded := 0
	for reserved := range results {
		if reserved {
			succeeded++
		}
	}
	fmt.Printf("succeeded: %d, rejected: %d, stock: %d\n",
		succeeded, attempts-succeeded, inventory.stock)
}
