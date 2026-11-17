package history

import (
	"slices"
	"testing"
)

func TestPageOrders(t *testing.T) {
	// 输入故意打乱顺序，两种语言连翻三页应得到同一组 ID。
	orderIDs := []int64{3, 5, 1, 4, 2}
	before := int64(0)
	for _, test := range []struct {
		ids  []int64
		next int64
	}{
		{ids: []int64{5, 4}, next: 4},
		{ids: []int64{3, 2}, next: 2},
		{ids: []int64{1}, next: 0},
	} {
		page, err := PageOrders(orderIDs, 2, before)
		if err != nil || !slices.Equal(page.Items, test.ids) {
			t.Fatalf("before=%d: 得到 %v，错误 %v，预期 %v", before, page.Items, err, test.ids)
		}
		if test.next == 0 {
			if page.NextCursor != nil {
				t.Fatalf("最后一页不应返回游标")
			}
		} else if page.NextCursor == nil || *page.NextCursor != test.next {
			t.Fatalf("before=%d: 有下一页时，游标应为本页末尾的 %d", before, test.next)
		}
		before = test.next
	}
	if !slices.Equal(orderIDs, []int64{3, 5, 1, 4, 2}) {
		t.Fatalf("分页修改了输入顺序: %v", orderIDs)
	}
}

func TestPageOrdersEmptyAndInvalid(t *testing.T) {
	page, err := PageOrders(nil, 2, 0)
	if err != nil || page.Items == nil || len(page.Items) != 0 || page.NextCursor != nil {
		t.Fatalf("空列表应该得到空切片和空游标，实际 %+v，错误 %v", page, err)
	}

	// 第二页的存在性取决于剩余订单数是否超过 limit。
	page, err = PageOrders([]int64{1, 2}, 2, 0)
	if err != nil || !slices.Equal(page.Items, []int64{2, 1}) || page.NextCursor != nil {
		t.Fatalf("正好一页时应返回两个订单和空游标，实际 %+v，错误 %v", page, err)
	}
	for _, input := range []struct {
		limit  int
		before int64
	}{{limit: 0}, {limit: 101}, {limit: 2, before: -1}} {
		if _, err := PageOrders(nil, input.limit, input.before); err == nil {
			t.Fatalf("limit=%d、before=%d 应返回错误", input.limit, input.before)
		}
	}
}
