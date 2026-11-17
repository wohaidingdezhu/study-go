# 09 · Go 服务的超时、取消与重试

在前面的订单 API 里，请求可能要等 Redis 或数据库。再加一个同步 HTTP 下游，就多了网络、连接池和下游处理时间。这一章先跑一个只有标准库的实验，观察一个慢依赖如何耗尽整次请求的时间预算。

## 跑三种结果

从项目根目录执行。实验会临时启动一个本机 HTTP 服务，在程序结束时关闭。

```bash
(cd go && go run ./cmd/service-lab)
(cd go && go run ./cmd/service-lab -fail-first)
(cd go && go run ./cmd/service-lab -budget 180ms -delay 600ms)
```

| 场景 | 预期输出 | 从哪里看出来 |
| --- | --- | --- |
| 依赖正常 | `stock=5, attempts=1` | 一次 GET 得到 JSON |
| 第一次返回 503 | `stock=5, attempts=2` | 等 50ms 后只重试一次 |
| 请求预算耗尽 | `deadline exceeded, attempts=1` | HTTP 请求跟随 context 取消，服务端也能停止等待 |

打开 [实验代码](../go/cmd/service-lab/main.go)，沿着 `main → readWithRetry → readStock` 看。两次尝试和中间等待共用一个 context。把 `-fail-first`、`-budget 120ms`、`-delay 600ms` 一起传入，观察为什么第二次请求也会在原预算用完时结束。 [Go context](https://go.dev/blog/context)

## 时间花在哪里

| 层 | 读什么 | 能防什么 |
| --- | --- | --- |
| 客户端到 API | DNS、建立 TCP/TLS 连接、上传、等首字节、读响应 | 连接慢、网络或服务器处理慢 |
| API 入口 | [HTTPServer](../go/order/http.go) 的 `ReadHeaderTimeout`、`ReadTimeout`、`WriteTimeout` | 慢客户端长时间占用连接 |
| 业务调用链 | API 创建 3 秒 context 并向 Redis、数据库传递 | 所有依赖操作共享等待预算 |
| 数据库 | 等连接池、执行 SQL、等行锁 | 分开定位连接耗尽和 SQL 执行慢 |

在已有订单服务上观察首字节和总耗时。 `-o /dev/null` 丢弃正文，`-w` 输出时间和状态。

```bash
curl -sS --max-time 5 -o /dev/null \
  -w 'status=%{http_code} connect=%{time_connect}s first_byte=%{time_starttransfer}s total=%{time_total}s\n' \
  http://127.0.0.1:8082/products/1
```

这个结果是一笔请求，适合定位单次耗时。评估服务还要看一段时间的请求数、错误率和延迟分布。p95 表示约 95% 的样本在该耗时以内，适合发现平均值掩盖的慢请求。

## 接口、业务和存储各负责什么

订单 API 的 handler 解析和校验请求、选择状态码。存储实现负责把一次业务操作变为带事务的数据库写入。跨进程后还要明确调用约定。HTTP/JSON 便于浏览器、curl 和不同语言联调。内部服务也可以定义 Protobuf 契约并使用 gRPC，调用方用生成的客户端传递参数、期限和错误状态。两种调用都要应对超时、发布兼容和部分失败。 [gRPC 官方介绍](https://grpc.io/docs/what-is-grpc/introduction/)

如果给项目增加用户体系，先认证调用方是谁，再授权它能否读取或修改这笔订单。幂等键也随业务身份隔离，例如按用户和键组成唯一约束。网关常用于 TLS、路由和公共流量控制，业务服务仍要检查资源权限。

## 什么时候重试，什么时候停

- 读取类请求遇到暂时性失败时，可以在剩余预算内有限重试。参数校验失败和库存不足需要先处理业务原因。支付或下单在超时后可能已经提交，重试前要确定服务具备幂等保障。
- 把重试放在最了解业务的一层。三层服务每层最多尝试 3 次，最坏可能把底层一次操作放大到 27 次。生产系统可用带抖动的指数退避，分散大量请求同时回来的压力。实验用固定等待，便于比较。 [AWS 超时、重试与退避](https://aws.amazon.com/builders-library/timeouts-retries-and-backoff-with-jitter/)
- 限流给入口设置速率上限，并发限制给在途工作设置上限。下游连续失败时，熔断可以暂时减少调用。商品展示可采用缓存回退，接受订单时需要保留数据库的库存约束。
- Go 中用 `go test -race` 查共享状态的竞态，再用 CPU、堆或 goroutine profile 判断慢在哪里。追查 goroutine 堆积时先看它们在等锁、等 channel、等网络，还是在等连接池。 [Go 性能诊断](https://go.dev/doc/diagnostics)

### 练习

画一个上限 300ms 的请求预算，把时间分给入口验证、一次 Redis 读取和一次数据库查询，再留一点给编码响应。假设 Redis 已用去 150ms，说明数据库最后能分到多少时间。然后用 `-budget`、`-delay` 预测和核对一次超时。

先按 [第 17 章](17-observability-and-probes.md) 用请求 ID 定位访问日志。发现瓶颈在 Go 进程里时，可以按 [第 14 章](14-go-profiling.md) 采集 CPU 和锁的 profile。
