# 练习 · 查库存时如何区分「卖完了」

API 查到商品 2 时应能显示库存 0，查一个没有录入的商品则应走缺失分支。先在一个函数里表达这个差别，再去 [第 02 章](../../docs/02-http.md) 看服务怎么把它变成响应。

| 商品编号 | 测试数据 | Go 预期 | Java 预期 |
| --- | --- | --- | --- |
| 1 | 5 件 | `5, true` | `OptionalInt.of(5)` |
| 2 | 0 件 | `0, true` | `OptionalInt.of(0)` |
| 999 | 缺失 | `0, false` | `OptionalInt.empty()` |

从项目根目录执行。编辑 [Go 函数](go/stock.go) 或 [Java 方法](java/StockLookup.java) 里的 TODO，再重跑对应检查。

```bash
bash scripts/check-exercise.sh stock-lookup go
bash scripts/check-exercise.sh stock-lookup java
```

第一次 Go 会指出商品 1、2 失败，Java 会在第一条失败的断言停下。通过时 Go 显示 `PASS`，Java 打印「库存查询用例通过」。

提示。Go 从 map 可以同时读出值和键是否存在。Java 版本的 map 保存有效整数，先用 `Integer` 接收结果，再决定是否有值。

<details>
<summary>参考解</summary>

Go 的 `exercises/stock-lookup/go/stock.go` 可以写成这样。

```go
package stock

func LookupStock(stockByID map[int]int, productID int) (int, bool) {
	stock, exists := stockByID[productID]
	return stock, exists
}
```

Java 的 `exercises/stock-lookup/java/StockLookup.java` 可以写成这样。

```java
import java.util.Map;
import java.util.OptionalInt;

final class StockLookup {
    static OptionalInt lookupStock(Map<Integer, Integer> stockById, int productId) {
        Integer stock = stockById.get(productId);
        if (stock == null) {
            return OptionalInt.empty();
        }
        return OptionalInt.of(stock);
    }
}
```

</details>

通过后把测试中的商品 2 删掉，预测它会走哪一条分支。再回到 [库存对照课](../../docs/01-languages.md#库存为零和查无商品)，用自己的话解释两个实现。
