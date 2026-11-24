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

## 自动验证分页边界

Day 27 可用下面的命令验证两种语言的真实 HTTP 入口。测试各自准备独立内存数据，无需启动常用端口或 Docker，不会消耗你正在使用的练习库存。

```bash
(cd go && go test -v ./order -run '^TestHTTPPagination')
mvn -f java/pom.xml '-Dtest=OrderHttpTest#rejectsInvalidPagination+paginationCursorContract' test
```

Go 有两个顶层测试，参数拒绝部分包含 14 个子用例；Java 本次选择运行 15 个测试，预期失败和错误均为 0。测试的准备、断言和资源清理方式见 [第 08 章](08-tests.md#两种语言怎样组织隔离测试)。[Go 测试源码](../go/order/http_test.go) · [Java 测试源码](../java/src/test/java/study/OrderHttpTest.java)

### 一次只改变一个查询条件

| 输入类型 | 例子 | 预期 |
| --- | --- | --- |
| 数量范围 | `limit=0`、`limit=101` | `400 invalid_pagination` |
| 数值表达 | `limit=-1`、`limit=1.5`、`limit=` | 同上 |
| 游标边界 | `before=0`、`before=`、大于有符号 64 位整数上限 | 同上 |
| 参数结构 | 未知参数、空参数名、缺少等号、重复 `limit` | 同上 |
| 编码后的重复键 | `limit=1&%6cimit=2` | `%6c` 解码为 `l`，两个键都叫 `limit`，仍要拒绝 |
| 编码后的分隔符 | `limit=2%26before%3D3` | 解码后的值是 `2&before=3`，不是整数，应拒绝 |

先按原始 `&` 划分参数边界，再分别解码键和值。把整个查询串解码后再拆分，会把某个值里的编码分隔符误当成新参数。Go 入口交给 `url.ParseQuery`，Java 入口先拆分再 `URLDecoder.decode`，可沿源码注释对照。 [Go ParseQuery](https://pkg.go.dev/net/url#ParseQuery) · [Java URLDecoder](https://docs.oracle.com/en/java/javase/21/docs/api/java.base/java/net/URLDecoder.html)

### 先手算，再运行插入场景

独立内存服务的前四笔订单 ID 为 1、2、3、4，每笔买一件。测试读取每次创建实际返回的 ID，不把连续 ID 当成数据库通用约定。

| 步骤 | 请求或操作 | 预期 ID 顺序 | nextCursor |
| --- | --- | --- | --- |
| 空库 | `GET /orders` | `[]`，不能用 `null` 代替数组 | `null` |
| 第一页 | `limit=2` | `[4,3]` | `3` |
| 翻页前插入 | 再创建订单 5 | 第一页响应保持原样 | 使用已有游标 3 |
| 第二页 | `limit=2&before=3` | `[2,1]` | `null` |
| 满页但读完 | `limit=1&before=2` | `[1]` | `null` |
| 最小 ID 之后 | `before=1` | `[]` | `null` |
| 重新从头读 | `limit=1` | `[5]` | `5`，后面仍有旧订单 |

第一页为了判断还有数据，会向存储多要一条，内部读到 `[4,3,2]`。响应返回 `[4,3]`，游标必须为 3。若误用多读那条的 ID 2，下一页按 `id < 2` 只返回 `[1]`，订单 2 被漏掉，虽然两页仍然“没有重复”。因此测试同时断言完整 ID、排序与游标，不能只比较条数或检查集合无交集。

刚好返回 `limit` 条也不代表还有下一页。例如只剩订单 1、`limit=1`，多读一条没有拿到额外数据，游标应为空。

这个插入实验说明当前接口的范围条件是 `id < before`：新插入的大 ID 不进入旧游标后续的页，需要从头浏览才能看到。本实验没有建立固定查询快照，也没有证明并发删除或字段更新时能保留旧结果。对账、导出等需要固定集合的业务，应另行定义一致性需求和查询方案。

### 做一次能暴露遗漏的变式

1. 合上源码，先画出错误游标取 2 时的两页结果，说出哪一项被漏掉。
2. 在本机 Go `TestHTTPPaginationCursorContract` 或 Java `paginationCursorContract` 的第一页游标断言中，暂时把期望改成多读的那条 ID。重跑测试，找到实际与期望，再恢复。
3. 改为每页一条，预测新增订单发生在两页之间时，旧游标后续返回哪些旧 ID；再补断言核对。每次只改一个条件。

保留一次失败与修复后的输出，用 `python3 scripts/study.py practice pagination` 对照验收。新复习卡会沿用已有任务与进度。
