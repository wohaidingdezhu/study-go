# 12 · 消息投递语义、积压与 Kafka

第 05 章做过暂停消费者和事件重放。本章把一次消息分成几段，判断每段失败后应该从哪里恢复。

| 故障发生在 | 可能看到什么 | 恢复依据 |
| --- | --- | --- |
| 订单已提交，relay 还没发出 | outbox 有记录，`published_at` 为空 | relay 恢复后扫描待发记录 |
| broker 收到消息，relay 在写标记前进程退出 | broker 可能收到相同的 `eventId` 两次 | 稳定事件 ID 和消费端唯一键 |
| 通知写入成功，ACK 还没送达 | broker 重新投递 | Java 以 `event_id` 去重，再确认本次投递 |
| 通知数据库暂时故障 | 消费者重试，消息等待时间增长 | 观察重试次数和剩余容量，恢复后补处理 |
| 消息无法解析或持续失败 | 进入死信队列 | 查明原因、修复后按原事件身份重放 |

至少一次允许重投，业务去重让同一事件重复到达时只写一条通知。这个保证依赖副作用和去重在同一个原子步骤中完成。项目通过一条带唯一约束的 SQL 达成。若副作用是调用外部邮件或支付接口，需考虑对方的幂等键或自己的可恢复发送记录。 [RabbitMQ 确认机制](https://www.rabbitmq.com/docs/confirms) · [Transactional outbox](https://docs.aws.amazon.com/prescriptive-guidance/latest/cloud-design-patterns/transactional-outbox.html)

## 实验：确认消息找得到队列

RabbitMQ 的默认 exchange 按 routing key 查找同名队列。下面使用一个没有配置的队列名，观察管理 API 的路由结果。请在已启动的本地实验栈执行。

```bash
curl -fsS -u study:study \
  http://127.0.0.1:15672/api/exchanges/%2F/amq.default/publish \
  -H 'Content-Type: application/json' \
  -d '{"properties":{},"routing_key":"study.no-such-queue","payload":"routing-probe","payload_encoding":"string"}'
```

预期是 `{"routed":false}`。对照第 05 章投向正确队列的例子，再打开 [Go relay](../labs/stack/relay.go)。AMQP 发布时，无法路由的消息也可能收到 publisher confirm。relay 还启用了 `mandatory` 并检查 return，确认消息有目标队列后才标记 outbox。 [RabbitMQ 确认与无法路由的消息](https://www.rabbitmq.com/docs/confirms)

## 怎么判断是哪一段积压

```bash
docker compose -f labs/stack/compose.yaml exec -T rabbitmq \
  rabbitmqctl list_queues name consumers messages_ready messages_unacknowledged
docker compose -f labs/stack/compose.yaml exec -T postgres psql -U study -d orders \
  -c 'SELECT count(*) AS pending, now() - min(created_at) AS oldest_wait FROM outbox WHERE published_at IS NULL;'
```

- outbox 的 `oldest_wait` 增长，先检查发布进程、broker 连通性和发布确认延迟。没有待发记录时这列为空。
- `messages_ready` 一直上涨，先确认 `consumers` 是否为零，再比较单位时间流入与处理成功的数量。持续每秒到 120 条、只能处理 100 条，积压约每秒增加 20 条。
- `messages_unacknowledged` 长时间偏高，检查消费耗时、数据库等待、未完成任务以及 prefetch 约束。本项目每个消费者最多 1 条未确认投递。提高 prefetch 可以增加在途消息，处理速度仍受执行方式和数据库容量约束。
- 死信增长时先识别是输入格式、权限还是依赖持续故障，再决定重放。重放时保留事件 ID，并确认处理状态和副作用。

## 一条订单事件放到 Kafka 会怎样

| 问题 | 本项目的 RabbitMQ | Kafka |
| --- | --- | --- |
| 发到哪里 | exchange 根据规则投递到队列 | 生产者写 topic 的某个 partition |
| 消费进度 | 每次成功处理后确认投递 | consumer group 记录各分区的 offset |
| 回放 | 本项目通过 outbox 重发或从死信重放 | 日志在保留期内时，可以调整 offset 重新读取 |
| 有序范围 | 重投和多个消费者会改变实际处理顺序 | 追加顺序在单个 partition 内，处理及外部写入顺序还取决于消费端 |

若只需要同一个订单内的事件有序，可以用 `orderId` 作为 Kafka 分区键，并在事件中记录版本，处理端按版本决定跳过旧事件或等待缺失版本。Kafka 的生产者幂等和事务有自己的保障范围，通知落到外部数据库时仍要设计去重。 [Apache Kafka 概念](https://kafka.apache.org/43/getting-started/introduction/)

### 练习

把第 07 章跑过的一笔订单画成「提交 → 发布 → 消费 → 确认」。标出两个可能产生重复的位置，并解释怎样用 SQL 和两条日志判断恢复成功。
