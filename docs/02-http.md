# 02 · HTTP 后端服务

从浏览器里的商品 JSON 开始。客户端用 HTTP 方法和路径表达要做的事，服务端解析请求、验证数据、执行业务规则，再用状态码和 JSON 返回结果。

## 先读懂浏览器里的 JSON

如果现有 Docker 实验已启动，在终端访问同一个地址。只做读取，这一步约 3 分钟。

```bash
curl -i http://127.0.0.1:8082/products/1
curl -i http://127.0.0.1:8082/products/999
```

- URL 里的 `127.0.0.1` 指本机，`8082` 是服务端口，`/products/1` 是路径，`1` 是要查的商品编号。
- `curl` 这里只给了 URL，因此发出 `GET`。加 `-i` 可以查看响应状态行、响应头和正文。
- 第一条预期是 `200 OK`，响应头的 `Content-Type` 表示正文是 JSON，空行下面的正文含 `id`、`name` 和 `stock`。库存取决于之前有没有下过单。
- 第二条预期是 `404 Not Found`，正文是 `{"error":"not_found"}`。这个请求已经到达服务，服务没有找到编号为 999 的商品。若终端直接报连接失败，先检查端口和进程。 [HTTP 状态码](https://www.rfc-editor.org/rfc/rfc9110.html#name-status-codes)

只启动了 Go 入门服务时，可把这两条命令的端口改成 `8080`。练过读取后，按 [README](../README.md#对比两个-http-服务) 启动 Go、Java 内存服务，再用它们练习写入。内存服务重启会回到库存 5。

## 亲手创建订单并重试

| 请求 | 请求数据 | 成功 |
| --- | --- | --- |
| `GET /products/1` | 路径里的商品编号 | `200`，返回 `id`、`name`、`stock` |
| `POST /orders` | JSON 含 `productId`、`quantity`。请求头带 `Idempotency-Key` | 新建 `201`，重放 `200`，都返回订单及 `Location` |
| `GET /orders/{id}` | 路径里的订单编号 | `200`，返回 `id`、`productId`、`quantity` |
| [`GET /orders`](16-order-history-api.md) | 可选的 `limit`、`before` 查询参数 | `200`，返回订单列表和下一页游标 |

下面先练 Go 入门服务。`-H` 添加请求头，`-d` 把 JSON 放进请求体并使用 `POST`。幂等键由你给出，用来让服务识别同一笔下单的重试。本实验的幂等键允许 1–64 位 ASCII 字母、数字、下划线和连字符。

```bash
curl -i http://127.0.0.1:8080/orders \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: http-lesson-1' \
  -d '{"productId":1,"quantity":2}'
```

1. 第一次返回 `201 Created`，响应头的 `Location` 给出查询路径。复制该路径到同一端口，例如 `curl -i http://127.0.0.1:8080/orders/1`，核对正文里的 `id` 和 `quantity`。
2. 原样执行 POST 第二次。预期返回 `200` 和原订单 ID。再访问 `/products/1`，初始库存为 5 时，现在应是 3。
3. 保留原来的键，改成 `"quantity":1` 再发一次。预期得到 `409` 和 `idempotency_conflict`。
4. 把请求端口改成 `8081`，用 Java 服务重复以上步骤。两个进程各有一份初始库存和订单，重启后可以重新练习。

### 看一次请求为什么失败

复制同一条 POST 命令，每次只改一项。不同请求用新的幂等键，原键冲突这一行除外。

| 改动 | 预期状态和正文里的 `error` | 拒绝原因 |
| --- | --- | --- |
| 数量设为 `0` | `400 invalid_order` | 请求数据不满足数量 1–100 的规则 |
| 把请求头换成 `Content-Type: text/plain` | `415 expected_json` | 请求声明的类型和接口接收的 JSON 不匹配 |
| 把 `productId` 改成 `999` | `404 not_found` | 订单引用的商品不存在 |
| 把数量设为 `100` | `409 out_of_stock` | 数量在合法范围内，当前库存不足 |
| 复用成功订单的键并改变数量 | `409 idempotency_conflict` | 同一个键已关联到另一组参数 |

两个 `409` 要结合正文区分。一个表示库存状态和请求冲突，一个表示同一个幂等键和现有订单发生冲突。完成后再查询库存，确认这些失败请求没有多扣库存。

## 顺着请求读源码

| 请求经过哪里 | Java | Go |
| --- | --- | --- |
| 接收、路由和响应 | [OrderServer.java](../java/src/main/java/study/OrderServer.java) 中的 HttpServer 和 Jackson | [http.go](../go/order/http.go) 中的 `net/http` 和 `encoding/json` |
| 业务判断和进程内存储 | [MemoryStore.java](../java/src/main/java/study/MemoryStore.java) 用 record、Map 和 `synchronized` | [store.go](../go/order/store.go) 用 struct、map 和 Mutex |
| 业务失败 | 抛出 `Rejected`，由 HTTP 层生成响应 | 返回 `error`，HTTP 层用 `errors.Is` 选择响应 |

幂等键让客户端在请求超时、结果未知时能用同一组参数重试。它和业务结果一起保存。这个实验只为成功创建的订单保留键，因此库存不足等失败回滚后可以重新尝试。实际多用户服务应按用户或租户隔离键，并设计键的生命周期。

### 练习

给订单查询加入一个 `formattedQuantity` 字段，在 Go 的 JSON tag 和 Java record 上各实现一次。用 `curl` 查看两边的响应，再思考客户端是否需要这个衍生字段。

手动确认状态码和库存后，可以接着做 [第 08 章的自动测试](08-tests.md)，把同样的业务预期写成可重复执行的检查。
想观察一次请求如何跨网络等待，用 [第 09 章](09-service-reliability.md) 的 Go 实验模拟慢响应和重试。
