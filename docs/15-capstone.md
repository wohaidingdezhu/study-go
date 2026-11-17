# 15 · 综合练习 · 订单历史与通知恢复

用这个任务复习 API、事务、缓存、消息消费和故障排查。先能证明已有链路正常，再给 Go 的订单历史加上商品筛选。可以分两次做，每次只检查当前一步。

## 启动独立练习环境

从项目根目录执行。练习环境的 Compose 项目名是 `study-go-practice`，入口用 18082，RabbitMQ 管理页用 25672。它有自己的容器、网络和数据卷，适合重复练习。 [Docker Compose 配置合并](https://docs.docker.com/reference/compose-file/merge/)

```bash
bash scripts/practice-stack.sh up --build --wait
bash scripts/practice-stack.sh ps
curl --retry 10 --retry-connrefused --retry-delay 1 -fsS http://127.0.0.1:18082/products/1
```

首次商品库存应为 5。练习结束后执行 `bash scripts/practice-stack.sh down`，下次启动可以继续使用练习数据。需要重来时，`bash scripts/practice-stack.sh down -v` 会清空这套练习的订单和消息数据，再执行上面的启动命令。

## 任务一：验证已有链路

从一套刚初始化的练习数据开始。暂停 Java 通知服务，提交订单并原样重试。下面的循环会把同一个请求发两次。

```bash
bash scripts/practice-stack.sh stop notifications
for attempt in 1 2; do
  curl -si http://127.0.0.1:18082/orders \
    -H 'Content-Type: application/json' \
    -H 'Idempotency-Key: capstone-order-1' \
    -d '{"productId":1,"quantity":1}'
done
curl -fsS http://127.0.0.1:18082/products/1
```

应看到第一次 `201`、第二次 `200`，订单 ID 相同，商品库存从 5 变成 4。记下返回的订单编号，下面以初次新建的订单 1 为例。

```bash
bash scripts/practice-stack.sh exec -T postgres psql -U study -d orders -c "
  SELECT o.id AS order_id, e.id AS event_id, e.published_at
  FROM orders AS o JOIN outbox AS e ON e.order_id = o.id
  WHERE o.idempotency_key = 'capstone-order-1';
"
bash scripts/practice-stack.sh exec -T rabbitmq \
  rabbitmqctl list_queues name consumers messages_ready messages_unacknowledged
```

看到 outbox 的 `published_at` 和队列的待消费消息后，再启动消费者。如果仍处于待发布，先看 `bash scripts/practice-stack.sh logs --tail=20 relay`。若已发布而 `messages_ready` 刚查询还是 0，等几秒再查一次。暂停期间应看到 `order.created` 的 `consumers` 为 0、`messages_ready` 为 1。然后查询通知数据库。

```bash
bash scripts/practice-stack.sh start notifications
bash scripts/practice-stack.sh logs --tail=20 relay notifications
bash scripts/practice-stack.sh exec -T postgres psql -U study -d notifications \
  -c 'SELECT event_id, order_id, received_at FROM order_notifications WHERE order_id = 1;'
```

如果启动后立刻查询还没有记录，等消费者日志出现 `Processed event` 再查询一次。通过条件是订单只新增一笔、outbox 事件 ID 与通知表匹配，通知表只有一行。沿着「HTTP 返回 → PostgreSQL 提交 → relay 发布 → Java 写入」解释每一条证据。

### 用脚本核对跨服务结果

用一套初始库存为 5、订单和通知为空的练习库运行。脚本会用完这 5 件库存，一共创建 4 笔订单。若先做了手动练习，保存需要的记录，再用新练习数据运行检查。

```bash
bash scripts/practice-stack.sh up --build --wait
python3 scripts/check-orders.py --practice-stack
```

脚本依次验证并发下单、订单分页，再按订单 ID 查询 outbox 与通知数据库。两边的订单 ID 和事件 ID 匹配时，会打印「跨服务检查通过」。如果发生超时，输出里会列出两边当时查到的记录。

## 任务二：给订单历史加商品筛选

先做 [订单分页对照题](../exercises/order-history/README.md)，再跑 [现有分页 API](16-order-history-api.md)。在这个基础上给 Go 的 `GET /orders` 增加可选的 `productId`，练习让内存和数据库共用一份查询约定。

### 验收约定

- `productId` 省略时查询全部订单，传入正整数时只列出该商品的订单。非法值返回 `400 invalid_pagination`。
- 筛选和游标都在取 `limit+1` 条之前生效，所以游标对应的是该商品的下一页。继续翻页时要同时带上 `before` 和相同的 `productId`。
- 无匹配时，响应是 `{"items":[],"nextCursor":null}`。分页仍按订单 ID 倒序。
- 重跑原有 [接口检查](../scripts/check-orders.py)，再加筛选相关的用例。

### 实现与自测

1. 在 [Go HTTP 层](../go/order/http.go) 读取和校验新参数。把筛选条件传进 `Store`。
2. [内存版](../go/order/store.go) 在快照里应用商品筛选，[数据库版](../labs/stack/store.go) 将商品编号作为 SQL 参数绑定。不能先取满一页再在响应里筛掉其他商品，否则页长与游标会出错。
3. 先用现有的商品 1 检查 `productId=1` 返回订单，`productId=999` 返回空页，`productId=0` 返回 400。
4. 在独立练习库加入下方的商品 2，用下单接口分别创建涉及两种商品的订单。请求 `/orders?productId=2&limit=1` 并翻页，核对每项都属于商品 2。最后留下一组有、无筛选的响应和新增测试结果。

```bash
bash scripts/practice-stack.sh exec -T postgres psql -U study -d orders \
  -c "INSERT INTO products (id, name, stock) VALUES (2, 'Pencil', 3) ON CONFLICT (id) DO NOTHING;"
```

进阶复盘。数据量大时可以比较现有主键和 `(product_id, id DESC)` 复合索引。用查询计划说明哪种能更快找到该商品的订单，以及新索引给写入带来的开销。
