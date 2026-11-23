package main

import (
	"errors"
	"fmt"
	"slices"
)

// inventory 用一个整数表示库存。复制这个结构体时，整数值也会被复制。
type inventory struct {
	stock int
}

// stockReader 约定读取能力，inventory 具有同名同签名的方法后就满足接口。
// 可对照 Java 的 StockReader 与 implements 声明。
type stockReader interface {
	available() int
}

func (item inventory) available() int {
	return item.stock
}

func valueAndCollectionDemo() {
	original := inventory{stock: 5}
	valueCopy := original
	valueCopy.stock--
	fmt.Printf("value copy: original=%d copy=%d\n", original.stock, valueCopy.stock)

	// 指针保存原变量的地址，通过这个指针修改字段会影响原变量。
	shared := &original
	shared.stock--
	fmt.Printf("shared reference: original=%d\n", original.stock)

	source := []*inventory{&original}
	// Clone 复制切片元素。这里元素是指针，两个切片的元素仍指向同一份库存。
	shallow := slices.Clone(source)
	shallow[0].stock--
	fmt.Printf("shallow snapshot: original=%d\n", original.stock)

	// 逐个取出结构体值，得到独立的库存副本。本例的字段全部是整数。
	independent := make([]inventory, len(source))
	for index, item := range source {
		independent[index] = *item
	}
	independent[0].stock--
	fmt.Printf("independent snapshot: original=%d copy=%d\n", original.stock, independent[0].stock)

	var reader stockReader = original
	fmt.Printf("interface stock: %d\n", reader.available())
}

func errorDemo() {
	outOfStock := errors.New("out_of_stock")
	// %w 保留原错误作为原因，在补充操作信息后，调用者仍能沿错误链识别业务错误。
	wrapped := fmt.Errorf("reserve product 1: %w", outOfStock)
	fmt.Printf("error direct: %t\n", wrapped == outOfStock)
	fmt.Printf("error cause: %t\n", errors.Is(wrapped, outOfStock))
}

// traceResource 用打印模拟资源的使用与关闭，便于观察顺序。
// 实际业务可在取得文件、响应体或查询结果后安排对应的关闭操作。
type traceResource struct{}

func (traceResource) Close() error {
	fmt.Println("resource: close")
	return nil
}

func useResource() {
	resource := traceResource{}
	// defer 在当前函数退出时执行，调用者恢复执行前，资源已完成本例的关闭动作。
	defer resource.Close()
	fmt.Println("resource: use")
}

func main() {
	valueAndCollectionDemo()
	errorDemo()
	useResource()
	fmt.Println("resource: returned")
}
