# 练习 · 用订单编号做游标分页

先在内存里练筛选、排序和计算下一页，再把规则迁移到 [真实订单 API](../../docs/16-order-history-api.md)。这道题只处理订单编号，输入可以是任意顺序的正整数 ID，每个 ID 唯一。

| 情况 | 规则 |
| --- | --- |
| 第一页 | 函数内 `before=0` 表示没有游标。对外的 HTTP 接口把未传参转换为 0 |
| 下一页 | 只取严格小于 `before` 的订单 ID |
| 顺序与数量 | 倒序排列，最多返回 `limit` 条，保持调用方输入的顺序 |
| 游标 | 只有后面还有记录时才返回本页最后一个 ID。最后一页与空页用 Go 的 `nil` 或 Java 的 `null` |
| 无效参数 | 数量超出 1–100 或 `before < 0` 时，Go 返回错误，Java 抛出 `IllegalArgumentException` |

用 `[3,5,1,4,2]` 和每页 2 条验算。首页返回 `[5,4]`，游标 4。第二页返回 `[3,2]`，游标 2。末页返回 `[1]`，游标为空。输入列表为空时，返回一个空列表。

从项目根目录执行。编辑 [Go 函数](go/history.go) 或 [Java 方法](java/OrderHistory.java) 里的 TODO。

```bash
bash scripts/check-exercise.sh order-history go
bash scripts/check-exercise.sh order-history java
```

提示。先复制符合条件的 ID，再排序。结果条数正好等于 `limit` 时，这页已经是最后一页。只有比 `limit` 多时，才需要返回游标。

<details>
<summary>参考解</summary>

Go 的 `exercises/order-history/go/history.go` 可以写成这样。

```go
package history

import (
	"errors"
	"slices"
)

type Page struct {
	Items      []int64
	NextCursor *int64
}

func PageOrders(orderIDs []int64, limit int, before int64) (Page, error) {
	if limit < 1 || limit > 100 || before < 0 {
		return Page{}, errors.New("invalid pagination")
	}

	matching := make([]int64, 0, len(orderIDs))
	for _, id := range orderIDs {
		if before == 0 || id < before {
			matching = append(matching, id)
		}
	}
	slices.Sort(matching)
	slices.Reverse(matching)

	if len(matching) <= limit {
		return Page{Items: matching}, nil
	}
	cursor := matching[limit-1]
	return Page{Items: matching[:limit], NextCursor: &cursor}, nil
}
```

Java 的 `exercises/order-history/java/OrderHistory.java` 可以写成这样。

```java
import java.util.Comparator;
import java.util.List;

final class OrderHistory {
    record Page(List<Long> items, Long nextCursor) {}

    static Page pageOrders(List<Long> orderIds, int limit, long before) {
        if (limit < 1 || limit > 100 || before < 0) {
            throw new IllegalArgumentException("invalid pagination");
        }

        var matching = orderIds.stream()
                .filter(id -> before == 0 || id < before)
                .sorted(Comparator.reverseOrder())
                .toList();

        if (matching.size() <= limit) {
            return new Page(matching, null);
        }
        return new Page(matching.subList(0, limit), matching.get(limit - 1));
    }
}
```

</details>

通过后用 `before=1` 单独测一次，解释为什么末尾没有更多记录。回到 [SQL 分页实验](../../docs/10-sql-performance.md)，比较内存排序与数据库按索引顺序读取的区别。
