# 05 · RabbitMQ 消息队列

下单成功后，Java 通知服务可以稍后处理事件。数据库里的 outbox 记录和订单一起提交。独立的 Go relay 轮询未发布的记录，发送 `order.created.v1` 事件，收到 broker 确认后才写入 `published_at`。

| 概念 | 本实验怎么用 |
| --- | --- |
| 生产者 | [Go relay](../labs/stack/relay.go) 把待发送事件转成 JSON，带上稳定的 `eventId` |
| Exchange 与路由 | 默认 exchange 按 routing key 路由到同名队列 `order.created` |
| 队列 | 持久化 quorum queue 保存待消费的消息，实验中只有一个 RabbitMQ 节点 |
| Publisher confirm | broker 确认接收发布，relay 才标记 outbox。设置 mandatory 并检查无法路由的返回 |
| Consumer ACK | [Java 消费者](../java/src/main/java/study/NotificationWorker.java) 把通知写入数据库后确认，可将这次投递从队列移除 |
| Prefetch | 消费者最多保留 1 条未确认投递，便于观察处理顺序 |
| 重试与死信 | 消费数据库出错时等待一秒再拒绝并重入队。quorum queue 超出投递限制后转送到死信队列，无效事件直接拒绝并进入死信队列 |

Publisher confirm 和消费 ACK 分别确认两段传递。relay 在 broker 确认后、数据库标记前崩溃，恢复时会再次发布同一个 `eventId`。消费者可能在提交后、ACK 前断开，此时 broker 也可以重新投递。通知表用 `event_id` 主键和 `ON CONFLICT DO NOTHING` 吸收两种重复。 [RabbitMQ 确认机制](https://www.rabbitmq.com/docs/confirms) · [Quorum queues](https://www.rabbitmq.com/docs/4.2/quorum-queues) · [Transactional outbox](https://docs.aws.amazon.com/prescriptive-guidance/latest/cloud-design-patterns/transactional-outbox.html)

## 暂停消费者，观察消息积压

```bash
docker compose -f labs/stack/compose.yaml stop notifications
curl -i http://127.0.0.1:8082/orders \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: mq-lesson-1' \
  -d '{"productId":1,"quantity":1}'
docker compose -f labs/stack/compose.yaml exec rabbitmq \
  rabbitmqctl list_queues name messages_ready messages_unacknowledged
```

等待 relay 发出事件，`order.created` 的 `messages_ready` 会增加。也可在 [RabbitMQ 管理页](http://127.0.0.1:15672) 用 `study` / `study` 观察队列。重新启动消费者并查通知数据库。

```bash
docker compose -f labs/stack/compose.yaml start notifications
docker compose -f labs/stack/compose.yaml logs --tail=15 relay notifications
docker compose -f labs/stack/compose.yaml exec postgres psql -U study -d notifications \
  -c 'SELECT event_id, order_id, received_at FROM order_notifications ORDER BY event_id;'
```

## 重放一条已消费的消息

先确认上一步出现了 `Processed event`。下面的 SQL 将刚才的事件重新标记为待发送，模拟 relay 确认后崩溃、恢复时重发。

```bash
docker compose -f labs/stack/compose.yaml exec postgres psql -U study -d orders \
  -c "UPDATE outbox SET published_at = NULL WHERE order_id = (SELECT id FROM orders WHERE idempotency_key = 'mq-lesson-1');"
docker compose -f labs/stack/compose.yaml logs --tail=15 relay notifications
```

等待 relay 再次发布。消费者日志出现 `Ignored duplicate`，订单对应的通知仍只有一行。

## 练习

往队列发一条无法解析的消息，观察 `order.created.dead`。这个命令用本地管理 API 模拟传错格式的生产者。

```bash
curl -fsS -u study:study http://127.0.0.1:15672/api/exchanges/%2F/amq.default/publish \
  -H 'Content-Type: application/json' \
  -d '{"properties":{"delivery_mode":2},"routing_key":"order.created","payload":"invalid-event","payload_encoding":"string"}'
```

预期发布返回 `"routed":true`，消费者记录 `Rejecting invalid event`，死信队列增加一条消息。思考如何定位坏消息、修正格式、复核副作用并重新投递。通知实验以数据库的一行表示完成，实际邮件或短信需要由外部通道支持幂等，或为该副作用再设计记录与重试。

要进一步理解队列积压和重复投递，从 [第 12 章](12-messaging-reliability.md) 的故障位置表开始，然后比较同一个事件在 RabbitMQ 与 Kafka 中的处理方式。
