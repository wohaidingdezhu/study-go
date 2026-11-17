# 03 · PostgreSQL 数据库与事务

启动 [进阶实验](../README.md#启动进阶实验)，本章 API 使用端口 8082。先看 [建表脚本](../labs/stack/init.sql) 和 [Go 数据库实现](../labs/stack/store.go)。

| 概念 | 在订单项目里的用途 |
| --- | --- |
| 主键 / 外键 | 订单有自己的 ID，通过 `product_id` 引用商品 |
| 唯一约束 | `idempotency_key` 保证并发的相同键最多产生一条已提交订单 |
| CHECK | 商品库存不能为负，订单数量必须在 1–100 内 |
| 索引 | 主键和唯一约束对应索引，待发布事件使用只覆盖未发布记录的部分索引 |
| 连接池 | Go 复用数据库连接，为请求提供等待连接的时间预算 |
| 事务 | 插入订单、扣减库存和插入待发送事件一起提交，任一步失败便回滚 |

```bash
docker compose -f labs/stack/compose.yaml exec postgres psql -U study -d orders
```

在 `psql` 中执行。

```sql
\d orders
\d outbox
SELECT id, name, stock FROM products;
EXPLAIN SELECT id FROM outbox WHERE published_at IS NULL ORDER BY id LIMIT 1;
```

PostgreSQL 会按代价选择查询计划。几行数据时看到顺序扫描很正常。索引可以减少读取，也会增加写入和存储成本。

## 跟一笔订单

```bash
curl -i http://127.0.0.1:8082/orders \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: db-lesson-1' \
  -d '{"productId":1,"quantity":1}'
```

再次查询数据库，可以看到一笔订单和对应的事件。原样重发请求后，这两张表都只多过一条记录。

```sql
SELECT id, idempotency_key, product_id, quantity FROM orders;
SELECT id, order_id, created_at, published_at FROM outbox;
SELECT id, stock FROM products;
```

并发扣减用的是 `UPDATE products SET stock = stock - $2 WHERE id = $1 AND stock >= $2`。在 PostgreSQL 的 Read Committed 隔离级别下，若目标行被另一个更新锁住，语句会等待，并在该更新提交后重新评估条件。影响行数为零时，服务回滚订单插入并返回库存不足。读库存后在应用里判断、再写入一个算好的绝对值，会让并发请求基于同一份旧库存计算。 [PostgreSQL 事务隔离](https://www.postgresql.org/docs/16/transaction-iso.html)

事务的原子性覆盖三次写入，约束保护数据库内的规则，隔离级别决定并发事务能看到什么，持久性保证提交记录能按数据库配置恢复。ACID 的一致性需要应用写对业务规则，消息稍后送到 Java 仍然是跨服务的最终一致。

## 从 Go 对照到 Java JDBC

| 操作 | Go pgx | Java JDBC |
| --- | --- | --- |
| 绑定参数 | SQL 写 `$1`，调用 `QueryRow(ctx, sql, id)` | SQL 写 `?`，用 `PreparedStatement.setLong(1, id)` |
| 查询结果 | `Scan(&value.ID, ...)` 把列写入变量 | 用 `ResultSet.next()` 移动游标，再调用 `getLong(...)` |
| 多语句事务 | `BeginTx` 后调用 `Commit` 或 `Rollback` | `setAutoCommit(false)` 后调用 `commit()` 或 `rollback()` |
| 释放连接 | 关闭查询结果，事务结束时连接返回池 | 用 `try-with-resources` 关闭语句和连接 |

[Java 通知消费者](../java/src/main/java/study/NotificationWorker.java) 只有一条写入语句，所以使用 JDBC 的自动提交。把它和 Go 下单的三步事务放在一起读，可以看到为什么事务边界由业务操作决定。

### 练习

1. 用数量 100 下单，预期返回 `409`。查数据库，确认订单表和 outbox 没有为这次失败增加行。
2. 单独用一套新实验卷演练并发，执行 `python3 scripts/check-orders.py http://127.0.0.1:8082`。它先验证八次并发重试只扣一次，再用不同请求同时抢库存。练习结束时库存为 0。
3. 在两次不同事务中读同一行。让另一个会话在两次读取中间更新并提交，比较 Read Committed 和 Repeatable Read 下第二次读取的值。

接着做 [第 10 章](10-sql-performance.md)，用有规模的数据看执行计划，再开两个会话观察行锁等待。
