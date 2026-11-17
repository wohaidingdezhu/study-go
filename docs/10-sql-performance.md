# 10 · 数据库查询计划、分页与锁

第 03 章用约束和事务保护订单。本章练查询慢和写入阻塞怎么定位。两种问题都可能表现为接口超时，排查的证据各不相同。

## 用 5 万行观察索引和分页

启动 [Docker 实验栈](../README.md#启动进阶实验)，在项目根目录执行。脚本在一个数据库会话中用临时表造数据，会话结束后自动清理。

```bash
docker compose -f labs/stack/compose.yaml exec -T postgres \
  psql -U study -d orders < labs/postgres/query-plans.sql
```

读 [SQL 脚本](../labs/postgres/query-plans.sql) 和输出里的三处信息。

| 比较 | 看哪里 | 应该学到什么 |
| --- | --- | --- |
| 只有主键索引和加入 `(customer_id, id DESC) INCLUDE (quantity)` | `Index Cond`、`Filter`、`Rows Removed by Filter` | 先定位一个客户的索引范围，减少扫描其他客户的行 |
| `OFFSET 800` 和 `id < 10006` | `Limit` 下方节点的实际行数 | OFFSET 需要跳过前面的合格行，游标可以直接从上页末尾之后找 |
| 两种分页的结果 | 最后一列 `same_page` 显示 `t` | 同一份固定数据、同一排序下，两边取到相同 ID |

本地 PostgreSQL 16 的一次运行中，OFFSET 让索引节点返回了 820 行，游标查询只返回了 20 行。两条都用了 `Index Only Scan`，`Heap Fetches` 均为 0。先观察你机器上的执行计划，再用实际行数解释差异。

`EXPLAIN` 显示估计的计划，`EXPLAIN ANALYZE` 会执行语句并显示实际行数和时间。再看 `BUFFERS` 可了解访问的页。本机时间受缓存、环境和数据分布影响，优先解释计划和实际扫描行数。对写语句用 `ANALYZE` 时，语句也会执行。 [PostgreSQL EXPLAIN](https://www.postgresql.org/docs/16/using-explain.html)

游标在接口里可以是上一页最后一个 ID。同一个字段值可能重复时，用诸如 `(created_at, id)` 的稳定排序，把两列都带进游标。跨请求翻页期间若数据继续新增或变化，是否还需要固定快照由业务决定。

B-tree 可以把索引键按顺序组织起来。数据库先沿树定位范围起点，再顺着叶子页读符合条件的一段。这个实验先按 `customer_id` 等值定位，再在命中的范围中按 `id` 倒序读。索引还通过 `INCLUDE` 存下要返回的 `quantity`，这叫覆盖查询。若数据页的可见性状态允许，`Index Only Scan` 可以直接返回结果。试着去掉 `INCLUDE (quantity)`，观察优化器是否改用主键并过滤客户。多建一个索引，插入订单时也要维护它，判断值得与否要结合实际读写比例。 [PostgreSQL B-tree 实现](https://www.postgresql.org/docs/16/btree-implementation.html) · [索引仅扫描](https://www.postgresql.org/docs/16/indexes-index-only-scans.html)

## 用两个连接观察锁等待

在两个终端各开一次 `docker compose -f labs/stack/compose.yaml exec postgres psql -U study -d orders`。这是行锁实验，选择无人用订单栈时练。两个会话都用基础商品行，无需改库存。

终端 A 持有行锁。

```sql
BEGIN;
SELECT id, stock FROM products WHERE id = 1 FOR UPDATE;
```

终端 B 尝试锁同一行。先用普通 `SELECT` 读取，再尝试加锁。

```sql
BEGIN;
SET LOCAL lock_timeout = '1s';
SELECT id, stock FROM products WHERE id = 1;
SELECT id, stock FROM products WHERE id = 1 FOR UPDATE;
ROLLBACK;
```

普通查询能读到它可见的版本。第二次加锁约一秒后超时。回到 A 执行 `ROLLBACK;` 释放锁，再在 B 执行同样四条语句，此时可以加锁。结束后用 `\q` 退出两个会话。行锁冲突是等待，两个事务互相持锁并都在等待对方才构成死锁。数据库检测到死锁会中止其中一个事务，应用需根据操作的幂等性和剩余预算决定是否重试整个事务。 [PostgreSQL 显式锁](https://www.postgresql.org/docs/16/explicit-locking.html)

## 把原理迁移到 MySQL

| 对比 | PostgreSQL 本项目 | MySQL InnoDB 学习点 |
| --- | --- | --- |
| 多版本并发控制 MVCC | 普通读取基于快照。默认 Read Committed 下，每条语句可看到新的提交 | 默认隔离级别是 Repeatable Read，普通一致性读的快照规则需要按 InnoDB 理解 |
| 锁定读取 | `FOR UPDATE` 对命中的行加锁 | 查看是否为唯一键等值或索引范围查询，理解记录锁和 next-key 锁 |
| 数据和索引的关系 | 普通索引与表数据分开存放，读取未覆盖列通常还要访问表 | 聚簇索引组织行数据，二级索引记录携带主键值，读未覆盖列可能按主键再查一次 |
| 优化方法 | 看实际行数、扫描方式和过滤条件 | 同样先看查询计划，再讨论索引列顺序、扫描范围和代价 |

索引、事务、MVCC、连接池都能迁移，默认行为及具体锁规则要按数据库和隔离级别核对。连接池过大也可能制造数据库竞争。接口变慢时，分开量获取连接耗时与实际执行 SQL 耗时。 [PostgreSQL 隔离级别](https://www.postgresql.org/docs/16/transaction-iso.html) · [MySQL InnoDB 隔离级别](https://dev.mysql.com/doc/refman/8.4/en/innodb-transaction-isolation-levels.html) · [InnoDB 索引类型](https://dev.mysql.com/doc/refman/8.4/en/innodb-index-types.html)

提交后的崩溃恢复还涉及 WAL。PostgreSQL 会先为数据修改记录日志，恢复时可以根据日志重做尚未写入数据页的修改。MVCC 解决读能看到哪个版本，锁协调冲突写入，WAL 为恢复提供依据。 [PostgreSQL WAL](https://www.postgresql.org/docs/16/wal-intro.html)

### 练习

试着改脚本中的 OFFSET 和游标值，先算出新的上一页末尾 ID，再验证 `same_page`。最后用一句话解释「有索引的深翻页为什么仍会慢」。
