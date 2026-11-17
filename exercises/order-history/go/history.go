package history

type Page struct {
	Items      []int64
	NextCursor *int64
}

// PageOrders 按 ID 倒序分页。before 为 0 表示第一页。
func PageOrders(orderIDs []int64, limit int, before int64) (Page, error) {
	// TODO: 校验参数，取符合条件的订单并排序，再决定是否有下一页。
	return Page{Items: []int64{}}, nil
}
