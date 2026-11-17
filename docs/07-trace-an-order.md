# 07 · 跟踪一笔订单

用一个订单编号串起 HTTP、PostgreSQL 和 Java 消费者，练习回答「下单成功了，通知现在走到哪一步」。这章可以复用 [第 03 章](03-database.md) 里已经创建的订单，下面的查询只读取状态。还没有订单时，可以先做第 03 章的「跟一笔订单」。

## 1. 确认 API 能查到订单

从创建订单的响应头 `Location` 里找到路径。假设是 `/orders/1`，把下面的 `1` 换成你实际拿到的编号。

```bash
curl -i http://127.0.0.1:8082/orders/1
```

先看状态码，再看正文的 `id`、`productId` 和 `quantity`。如果收到 `404`，先核对请求中的端口和订单编号。Go 与 Java 的内存服务各有自己的订单，进阶实验的订单在端口 `8082`。

## 2. 用 SQL 找到同一个订单的事件

在项目根目录执行。把查询里的 `1` 换成实际订单编号。

```bash
docker compose -f labs/stack/compose.yaml exec -T postgres psql -U study -d orders -c '
  SELECT o.id AS order_id, o.quantity, e.id AS event_id, e.published_at
  FROM orders AS o
  LEFT JOIN outbox AS e ON e.order_id = o.id
  WHERE o.id = 1;
'
```

`LEFT JOIN` 先保留找到的订单，再用 `order_id` 配对 outbox 事件。如果事件缺失，右侧列会显示为空，便于看出问题在哪一层。正常下单的两条记录在同一个事务中提交，应该能看到 `event_id`。刚下单时 `published_at` 可能还是空，表示 relay 尚未把这条事件标记为发布完成。这个时间只记录发布侧的状态。

## 3. 看 Java 消费者是否写入通知

```bash
docker compose -f labs/stack/compose.yaml exec -T postgres psql -U study -d notifications -c '
  SELECT event_id, order_id, received_at
  FROM order_notifications
  WHERE order_id = 1;
'
docker compose -f labs/stack/compose.yaml logs --tail=30 relay notifications
```

成功时，通知里的 `event_id` 和上一步查到的值一致。日志中的 `published event X for order Y` 和 `Processed event X for order Y` 让你能分别确认发布与消费。两个编号有时刚好相等，查询时仍要认清它们是两列。

| 观察到什么 | 下一步检查什么 |
| --- | --- |
| `curl` 连接失败 | 用 `docker compose -f labs/stack/compose.yaml ps` 检查 API 是否在运行，核对端口 |
| 有订单，`published_at` 持续为空 | 查 relay 日志，再查 PostgreSQL 和 RabbitMQ 是否可用 |
| 有发布时间，通知表迟迟无记录 | 查消费者日志和 RabbitMQ 队列，确认消息是在等待、尚未确认，还是进了死信队列 |
| 消费器出现 `Ignored duplicate` | 用相同 `event_id` 看通知表，确认仍只有一行 |

### 回顾

1. 一次 `201 Created` 能证明哪一步做完了？通知落库要在哪里确认？
2. 用 [第 05 章](05-messaging.md) 的暂停消费者实验验证自己的判断。暂停期间查一次，启动消费者后再查一次，记录两次的 `published_at` 和通知行数。
