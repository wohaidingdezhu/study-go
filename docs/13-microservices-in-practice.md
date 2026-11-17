# 13 · 微服务边界、多实例与分布式故障

这章用同一套订单服务回答三个问题：怎样决定边界，两台 API 怎么处理同一个请求，以及哪个组件失败会让哪部分业务停下来。

## 从业务规则决定服务边界

| 决策 | 对照本项目 |
| --- | --- |
| 哪些修改必须一起成功 | 扣库存、创建订单、登记 outbox 在同一数据库事务里 |
| 哪部分允许晚一点发生 | Java 通知服务通过消息消费订单事件，有单独的数据和处理节奏 |
| 用同步还是异步 | 客户端同步得知订单写入结果，通知通过事件传递，失败可稍后恢复 |
| 怎样减少耦合 | 消费者依赖事件含义和字段契约，保留旧版本兼容窗口，新增必填字段前先检查消费者 |

拆分后，每个服务可以独立部署、扩展和排障，同时多了网络超时和跨服务数据一致性问题。订单和库存规则强相关时，可以先做同一个服务里的独立模块，再根据数据所有权和部署需求判断是否拆开。

扩容时，入口负载均衡负责选择实例，服务发现负责找可用目标。这个栈用 Compose DNS 找依赖。Kubernetes 中可以用 Service 提供稳定地址，readiness 探针判断 Pod 能否接流量，liveness 判断是否需要重启，startup 适合给慢启动留时间。 [Kubernetes 探针](https://kubernetes.io/docs/concepts/configuration/liveness-readiness-startup-probes/)

## 实验：两个 API 实例提交同一个幂等键

先启动 [Docker 实验栈](../README.md#启动进阶实验) 并确认商品还有至少 1 件。会在进阶实验库实际创建一笔订单。以下命令从项目根目录执行，保持在同一个终端。

```bash
curl -fsS http://127.0.0.1:8082/products/1
docker compose -f labs/stack/compose.yaml run --detach --rm --no-deps \
  --name study-go-api-replica -p 127.0.0.1:8083:8082 api
curl --retry 10 --retry-connrefused --retry-delay 1 -fsS http://127.0.0.1:8083/products/1
```

`8082` 是原实例，`8083` 是刚创建的副本，两者连到相同的 PostgreSQL 和 Redis。用同一个键分别请求两个端口。

```bash
key="replica-lesson-$(date +%s)"
for port in 8082 8083; do
  curl -si "http://127.0.0.1:$port/orders" \
    -H 'Content-Type: application/json' \
    -H "Idempotency-Key: $key" \
    -d '{"productId":1,"quantity":1}'
done
curl -fsS http://127.0.0.1:8082/products/1
docker stop study-go-api-replica
```

预期先得到 `201`，再得到 `200`，两次的订单 ID 一样，库存只减 1。第二个进程能重放订单，是因为两个实例共用数据库的唯一约束和订单记录。然后用 [第 07 章](07-trace-an-order.md) 查这个订单的 outbox 与通知。 [Docker Compose run](https://docs.docker.com/reference/cli/docker/compose/run/)

## 读懂故障边界

| 发生什么 | 下单能否成功 | 用什么观测恢复 |
| --- | --- | --- |
| 一个 API 进程退出 | 有其他健康实例接请求时仍可下单 | 实例就绪、入口错误率、重试是否返回原订单 |
| Redis 不可用 | 数据库可用且容量足够时仍可下单 | 缓存回源率、数据库池等待和接口延迟 |
| RabbitMQ 不可用 | 数据库仍能提交订单和待发事件 | 待发数量、最老事件年龄、恢复后的通知记录 |
| PostgreSQL 无法写入 | 当前架构无法接受新订单 | 实例能否重新写入、是否存在已提交的幂等记录 |
| 读副本落后 | 取决于读路径，本项目没有读副本 | 复制延迟、提交后立即读是否读到新值 |

多 API 实例主要解决应用进程的可用性与容量。实验的 PostgreSQL 和 RabbitMQ 仍是单节点。实际部署还要判断数据库复制、故障转移和消息副本是否能跨不同故障域。三副本的多数派通常需要两份同意，允许一个副本不可用。在网络分区期间，如果隔离一侧无法确认最新状态，需要针对读写分别决定哪些操作要等待、失败或允许读旧值。对库存写入要优先保护业务约束。

跨服务写入时，两阶段提交让参与方先准备，再由协调者决定统一提交或回滚。准备好的事务可能保留锁并等待最终决定。Saga 把流程分成多个可提交的步骤，失败时通过业务动作补偿，比如释放已经预留的库存。Outbox 解决本地写入与事件发布衔接的问题，可作为 Saga 的通信机制之一。 [PostgreSQL 两阶段事务](https://www.postgresql.org/docs/16/two-phase.html) · [Saga 编排](https://docs.aws.amazon.com/prescriptive-guidance/latest/cloud-design-patterns/saga-orchestration.html)

## 用指标验证设计

- API 层统计请求量、错误率、p95 延迟。一个可讨论的服务目标是「在一个窗口内，99% 的有效下单请求在 300ms 内完成」，这里的数字是练习示例，需按实际测量和业务要求选定。
- 异步链路单独看「从订单提交到通知落库用了多久」。可以用 outbox 和通知时间做实验，积压时需要看最老年龄与处理速度。
- 日志适合用 `orderId`、`eventId` 搜具体业务过程。Trace 用 span 串起调用步骤，Metrics 用于看趋势。把请求 ID、订单号用作高基数指标标签会让时间序列快速膨胀。 [OpenTelemetry 信号](https://opentelemetry.io/docs/concepts/signals/)

### 练习

设计「订单预留库存 → 支付 → 确认订单」。写下订单状态、支付超时后怎样查询结果、什么时候释放库存，以及重复收到一个支付结果会怎样。把每一步标成一个服务的本地事务或一段远程调用，最后说明补偿失败怎样重试。 [Saga 编排](https://docs.aws.amazon.com/prescriptive-guidance/latest/cloud-design-patterns/saga-orchestration.html)

先做 [第 17 章的探针实验](17-observability-and-probes.md)，响应 503 时观察实例为何仍在运行。之后做 [第 15 章的综合练习](15-capstone.md)，用独立环境复现一笔订单的幂等请求和通知恢复。
