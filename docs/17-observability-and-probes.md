# 17 · 用请求 ID 和探针排查服务

这一章练两件事。先从响应里的 ID 找到一次请求的访问日志，再分别停 Redis 和 PostgreSQL，看看进程状态与服务能否接请求怎样变化。普通读取和健康检查都可以反复运行。

## 从响应找到日志

从项目根目录启动一套 [独立练习栈](15-capstone.md#启动独立练习环境)。如果练的是 Go 或 Java 内存版，分别把端口换成 8080 或 8081，日志在启动服务的终端里。

```bash
bash scripts/practice-stack.sh up --build --wait
curl -si -H 'X-Request-ID: read-product-01' http://127.0.0.1:18082/products/1
bash scripts/practice-stack.sh logs --tail=30 api | grep -F 'read-product-01'
```

响应头应有 `X-Request-Id: read-product-01`，HTTP 头名大小写等价。日志会有一条 `msg` 为 `http_request` 的 JSON，能看到以下字段。

| 字段 | 用来查什么 |
| --- | --- |
| `request_id` | 按响应头回查这次请求 |
| `method`、`path` | 确认访问了哪个接口 |
| `status` | 看到服务返回给客户端的状态码 |
| `duration_ms` | 服务处理这次请求的毫秒数，不足 1ms 显示为 0 |

Go 进阶版还会在商品缓存的 `HIT`、`MISS` 或 `BYPASS` 日志里带同一个 ID。请求头可省略，服务会生成 ID。传入 `bad/id` 这类不合规则的值也会换成新 ID。可以再查一次 `/products/999`，核对 404 和日志的状态是否一致。 [Go 结构化日志](https://go.dev/blog/slog)

`X-Request-ID` 标识一次 HTTP 请求。同一笔下单重试时可以用不同的请求 ID，同时复用同一个 `Idempotency-Key`。查发布和消费时用 [第 07 章](07-trace-an-order.md) 的订单 ID 和事件 ID 配对。

## 观察两种健康检查

| 路径 | 健康时 | 表示什么 |
| --- | --- | --- |
| `GET /livez` | `200 {"status":"live"}` | HTTP 进程能响应 |
| `GET /readyz` | `200 {"status":"ready"}` | 订单存储可连接。Go 和 Java 内存版启动后可用，进阶版在 500ms 的时间预算内探测 PostgreSQL |
| `GET /readyz` 在 PostgreSQL 停机时 | `503 {"status":"not_ready"}` | 订单服务暂时无法使用核心存储 |

先查两条基线，然后只停 Redis。

```bash
curl -si http://127.0.0.1:18082/livez
curl -si http://127.0.0.1:18082/readyz
bash scripts/practice-stack.sh stop redis
curl -si http://127.0.0.1:18082/readyz
curl -si -H 'X-Request-ID: redis-down-01' http://127.0.0.1:18082/products/1
bash scripts/practice-stack.sh logs --tail=30 api | grep -F 'redis-down-01'
bash scripts/practice-stack.sh start redis
```

预期 `readyz` 仍为 200，商品请求能查数据库。日志使用 `BYPASS` 标出回源路径。Redis 在这里承担读缓存，PostgreSQL 有订单和商品的持久化数据。

接着只停 PostgreSQL。先预测三条 curl 会分别返回什么，再运行。

```bash
bash scripts/practice-stack.sh stop postgres
curl -si http://127.0.0.1:18082/livez
curl -si http://127.0.0.1:18082/readyz
curl -si -H 'X-Request-ID: pg-down-01' http://127.0.0.1:18082/orders
bash scripts/practice-stack.sh logs --tail=30 api | grep -F 'pg-down-01'
bash scripts/practice-stack.sh ps api
bash scripts/practice-stack.sh start postgres
```

这一轮分别返回 200、503、500。订单列表直接查询 PostgreSQL，日志里能按 `pg-down-01` 找到存储错误和访问结果。Compose 定期调用 `/readyz`。连续失败后，`ps` 才会显示 `unhealthy`，因此立即查询时状态可能还未更新。重新启动 PostgreSQL 后，重复查询 `/readyz`，等它恢复 200，再打开订单列表。 [Docker 健康检查](https://docs.docker.com/reference/dockerfile/#healthcheck)

在 Kubernetes 中，readiness 决定实例是否加入可接流量的集合，liveness 连续失败可以触发重启。依赖数据库的探测适合放在 readiness，这样数据库故障时可以先停止接入流量，同时保留 API 进程用于响应探针和查看日志。 [Kubernetes 探针](https://kubernetes.io/docs/concepts/workloads/pods/probes/)

## 对照源码和自测

Go 的 [日志中间件](../go/order/request_log.go) 包一层 `ResponseWriter` 来记录实际状态，把请求 ID 放进 context。Java 的 [OrderServer](../java/src/main/java/study/OrderServer.java) 从 `HttpExchange` 读取响应状态。两边都在请求结束时记录日志。Go 的 [探针入口](../go/order/http.go) 通过存储的 `Ping` 方法选择就绪状态，PostgreSQL 版在 [数据库适配层](../labs/stack/store.go) 检查连接。

把两次故障的响应和日志各留一条。复习时试着回答「如果只有 503，下一步先查进程、缓存还是数据库」「为什么 Redis 和 PostgreSQL 的故障结果不同」。在新练习数据上跑 [跨服务检查](15-capstone.md#用脚本核对跨服务结果)，还能验证恢复后的幂等下单和通知。
