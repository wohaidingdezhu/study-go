package main

import (
	"fmt"
	"maps"
	"unicode/utf8"
)

func showStock(stockByID map[int]int, id int) {
	// 查询缺失的键也会得到 int 的零值。用 exists 区分已售罄和商品不存在。
	stock, exists := stockByID[id]
	if !exists {
		fmt.Printf("product %d: not found\n", id)
		return
	}
	fmt.Printf("product %d: stock=%d\n", id, stock)
}

func main() {
	stockByID := map[int]int{1: 5, 2: 0}
	for _, id := range []int{1, 2, 999} {
		showStock(stockByID, id)
	}

	// 给另一个变量赋值后，两者仍访问同一个 map。
	sharedStock := stockByID
	sharedStock[1]--
	fmt.Printf("original after shared update: %d\n", stockByID[1])

	// Clone 复制 map 的条目。此处值都是 int，修改副本中的库存不会影响原表。
	copiedStock := maps.Clone(stockByID)
	copiedStock[1]--
	fmt.Printf("original after copy update: %d\n", stockByID[1])
	fmt.Printf("copy after copy update: %d\n", copiedStock[1])

	// Go 的字符串以字节索引。这里是 UTF-8 文本，按码点数时要用 utf8 包。
	name := "笔记本📒"
	fmt.Printf("UTF-8 bytes: %d\n", len(name))
	fmt.Printf("Unicode code points: %d\n", utf8.RuneCountInString(name))
}
