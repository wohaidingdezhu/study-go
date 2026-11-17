package main

import (
	"errors"
	"fmt"
)

// Line 用 struct 把一个订单明细的字段放在一起，对照 Java 的 Line record。
// 单价用整数分保存，1200 表示 12 元，例子里可以直接用整数相乘。
type Line struct {
	name      string
	unitCents int
	quantity  int
}

// (line Line) 是方法的接收者，相当于本次调用的那个明细。
// 两个返回值分别是结果和错误。nil 表示没有错误。
func (line Line) subtotal() (int, error) {
	if line.quantity <= 0 {
		return 0, errors.New("quantity must be positive")
	}
	return line.unitCents * line.quantity, nil
}

// 只要方法签名匹配，Line 就能赋给 Priced。对照 Java 的 implements。
type Priced interface {
	subtotal() (int, error)
}

func total(items []Priced) (int, error) {
	sum := 0
	// range 遍历切片时给出下标和元素。这里用 _ 忽略下标。
	for _, item := range items {
		value, err := item.subtotal()
		if err != nil {
			return 0, err
		}
		sum += value
	}
	return sum, nil
}

func main() {
	// := 让编译器根据右边推断局部变量的类型。这里分别是切片和 map。
	lines := []Priced{Line{"Notebook", 1200, 2}, Line{"Pen", 300, 1}}
	categories := map[string]string{"Notebook": "Stationery"}

	sum, err := total(lines)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("total: %d cents\n", sum)
	fmt.Println("category:", categories["Notebook"])

	// 缺失键的值也是空字符串，用 ok 才能区分缺失和显式保存的空字符串。
	if _, ok := categories["Pen"]; !ok {
		fmt.Println("category missing: Pen")
	}
	if _, err := total([]Priced{Line{"Pen", 300, 0}}); err != nil {
		fmt.Println("invalid item:", err)
	}
}
