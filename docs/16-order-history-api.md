# 16 · 在真实 API 里翻两页订单

[分页对照题](../exercises/order-history/README.md) 练的是内存中的一组 ID。本章用同一个 HTTP 接口读取实际下单结果，比较 Go、Java 和 PostgreSQL 怎样产生同样的响应。

## 先跑起来

按照 [README](../README.md#对比两个-http-服务) 启动任一个内存服务。Go 用 8080，Java 用 8081。要练数据库版，可用 [独立练习栈](15-capstone.md#启动独立练习环境) 构建新镜像，入口是 18082。以下示例使用一个刚启动的 Go 内存服务，库存至少需要 3 件。

```bash
base_url=http://127.0.0.1:8080
curl -fsS "$base_url/orders"
```

空订单库会返回 `{"items":[],"nextCursor":null}`。在同一个终端提交三笔订单，每笔使用自己的幂等键。

```bash
for n in 1 2 3; do
  curl -fsS "$base_url/orders" \
    -H 'Content-Type: application/json' \
    -H "Idempotency-Key: history-$n" \
    -d '{"productId":1,"quantity":1}'
  printf '\n'
done
curl -fsS "$base_url/orders?limit=2"
```

在新内存服务里，第一页会返回 ID 为 3 和 2 的订单，`nextCursor` 为 2。数据库的 ID 可能有间隔。翻页时复制响应里的实际值，使用相同的 `limit`。

```bash
curl -fsS "$base_url/orders?limit=2&before=2"
curl -si "$base_url/orders?before=0"
```

这里第二页只剩 ID 1，游标为 `null`。最后一个请求返回 `400` 和 `{"error":"invalid_pagination"}`。

## API 约定

| 项目 | 行为 |
| --- | --- |
| `GET /orders` | 从最新的订单 ID 开始倒序列出练习库中的记录 |
| `limit` | 省略时为 20，显式传入时接受 1–100 |
| `before` | 可省略。传入正整数后，只读取 ID 严格小于它的订单 |
| `items` | 每项含 `id`、`productId`、`quantity`。无结果时为 `[]` |
| `nextCursor` | 有下一页时是本页最后一个 ID，数据已读完时为 `null` |
| 校验 | 参数名只接受 `limit` 和 `before`，其值使用十进制数字。重复、未知、空值、超界和溢出都返回 `400 invalid_pagination` |

ID 提供唯一的排序顺序，游标给下一次查询提供范围起点。这套用法适合顺序浏览。页面之间如果发生写入或删除，后续请求看到的内容也可能变化。在需要固定结果集时，应另定义查询快照或业务上的截止条件。 [PostgreSQL 排序与 LIMIT](https://www.postgresql.org/docs/16/queries-limit.html)

## 从 HTTP 跟到存储

| 位置 | Go | Java |
| --- | --- | --- |
| 解析参数 | [http.go](../go/order/http.go) 使用 `url.ParseQuery` 读取原始查询串 | [OrderServer.java](../java/src/main/java/study/OrderServer.java) 拆分参数后用 `URLDecoder` 解码键和值 |
| 内存筛选和排序 | [store.go](../go/order/store.go) 在锁内读取数据，在锁外对切片按 ID 排序 | [MemoryStore.java](../java/src/main/java/study/MemoryStore.java) 复制 record 列表，在锁外筛选、排序 |
| 表示空游标 | Go 用 `*int64` 的 `nil` | Java 用 `Long` 的 `null` |

两种实现都最多向存储请求 `limit+1` 条。多出来的一条只用来判断下一页是否存在。把它的 ID 当作游标会漏掉那一条，因此游标仍取响应里最后一条的 ID。

PostgreSQL 版复用 Go 的 HTTP 层。 [SQL 存储](../labs/stack/store.go) 使用 `ORDER BY id DESC LIMIT`，下一页增加 `WHERE id < before` 的参数条件。主键 B-tree 能按 ID 倒序提供数据。具体计划由优化器根据数据量和代价选择，可以在 [第 10 章](10-sql-performance.md) 用 5 万行对照。 [PostgreSQL 索引排序](https://www.postgresql.org/docs/16/indexes-ordering.html)

### 练习

用另一种语言的内存服务重跑相同命令。每页改成 1 条，记录三个响应的游标。接着在新的空练习库运行 [同一组接口检查](../scripts/check-orders.py)，看看它如何确认结果不重不漏。准备修改接口时，去做 [综合练习的商品筛选](15-capstone.md#任务二给订单历史加商品筛选)。
