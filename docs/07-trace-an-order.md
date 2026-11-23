# 07 · 跟踪一笔订单

用一个订单编号串起 HTTP、PostgreSQL 和 Java 消费者，练习回答「下单成功了，通知现在走到哪一步」。这章可以复用 [第 03 章](03-database.md) 里已经创建的订单，下面的查询只读取状态。还没有订单时，可以先做第 03 章的「跟一笔订单」。

## 从概念找到源码

先沿一笔订单读下表中的源码注释。每次只看一行，合上代码预测结果，再按对应章节验证。使用独立练习栈时，HTTP 地址改为 `18082`，容器操作用 `bash scripts/practice-stack.sh`，启动方法见 [独立环境](15-capstone.md#启动独立练习环境)。

| 当前要理解的概念 | 注释与代码入口 | 可以验证什么 |
| --- | --- | --- |
| handler、JSON、业务校验 | [Go HTTP](../go/order/http.go) · [Java HTTP](../java/src/main/java/study/OrderServer.java) | 同一数量为 0 的请求在两边都返回 400，尚未进入库存写入 |
| 互斥锁、临界区、内存快照 | [Go MemoryStore](../go/order/store.go) · [Java MemoryStore](../java/src/main/java/study/MemoryStore.java) | 按 [并发练习](01-languages.md) 观察同一进程内共享状态的保护范围，再解释第二个进程为何有自己的锁 |
| 主键、唯一约束、外键、CHECK | [PostgreSQL 初始化](../labs/stack/init.sql) | 在独立实验环境中尝试无效库存、重复幂等键和不存在的商品引用，说明分别被哪项规则拒绝 |
| 事务、条件更新、回滚 | [Go PostgreSQL 存储](../labs/stack/store.go) · [Java Spring Service](../labs/spring-orders/src/main/java/study/springorders/OrderService.java) | 用现有 [Go 并发验收](03-database.md) 与 [Java 隔离验收](../labs/spring-orders/README.md) 核对库存、订单行数和回滚 |
| Cache-Aside、回源、TTL | [Go 商品读取与写后删除](../labs/stack/store.go) | 连续读、写订单后读、停止 Redis 后读，按 [缓存练习](04-cache.md) 对照 MISS、HIT 与 BYPASS |
| outbox、发布确认、路由 | [Go 写入 outbox](../labs/stack/store.go) · [Go relay](../labs/stack/relay.go) | 先查订单与事件是否一起提交，再检查 `published_at` 和发布日志 |
| 消费幂等、ACK、重投 | [Java NotificationWorker](../java/src/main/java/study/NotificationWorker.java) | 暂停与恢复消费者，按 [队列练习](05-messaging.md) 核对事件 ID、通知行数和确认时机 |

同一条业务链有多处提交点。订单接口成功表示业务事务已提交，relay 的发布确认对应 broker 接收发布，消费者 ACK 则在通知写库之后。下面三步分别验证这些位置。复习时把进程崩溃放到任意相邻两步之间，先预测哪一步可能重复、哪个约束会去重，再回到注释核对。

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
