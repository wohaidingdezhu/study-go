package stock

import "testing"

func TestLookupStock(t *testing.T) {
	stockByID := map[int]int{1: 5, 2: 0}
	// 两个返回值都要检查，才能区分已售罄和查无商品。
	tests := []struct {
		name   string
		id     int
		stock  int
		exists bool
	}{
		{name: "有库存", id: 1, stock: 5, exists: true},
		{name: "已售罄", id: 2, stock: 0, exists: true},
		{name: "商品不存在", id: 999, stock: 0, exists: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stock, exists := LookupStock(stockByID, test.id)
			if stock != test.stock || exists != test.exists {
				t.Fatalf("商品 %d 得到 (%d, %t)，预期 (%d, %t)", test.id, stock, exists, test.stock, test.exists)
			}
		})
	}
}
