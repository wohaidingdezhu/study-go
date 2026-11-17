# 开始学习和自测

要自动安排闭卷复习，可以从 [学习系统](study-system.md) 开始。这份手册提供主线之外的额外练习和查阅入口。

每次做一项，先预测结果，运行后留下一条证据，再合上文档回答复习题。空闲少时做 15 分钟，只跑通一个命令即可。有 30–45 分钟时，再改变一次输入或故障条件。每周做 3 次大约需要 6 周，按自己的进度完成。

## 第一次开始

在项目根目录检查工具。Java 使用 JDK 21 或以上，Go 入门练习使用 1.22 或以上。Mac 的安装和选用 JDK 命令放在 [README](../README.md#先跑一段语法对照)。

```bash
bash scripts/study-doctor.sh
```

准备运行 Docker 实验时，再检查引擎。如果你使用 Colima 且引擎停了，先运行 `colima start`。

```bash
bash scripts/study-doctor.sh --docker
```

今天只做一步也可以。从 [第 01 章](01-languages.md) 跑两版购物车，把第一件商品数量从 2 改到 3，预测总额后再运行一次。如果你已跑通商品接口，从表里的第 4 次继续。

## 18 次学习任务

| 次数 | 动手任务 | 留下的证据 |
| --- | --- | --- |
| 1 | [Go 和 Java 购物车](01-languages.md) 改数量后重跑 | 说出进入 `main` 后怎样到达小计函数，得到 3900 分 |
| 2 | [库存查询](../exercises/stock-lookup/README.md) 先看测试失败，再补齐 Go 或 Java | 三个用例通过，能解释为什么 0 件库存仍表示该商品存在 |
| 3 | [并发扣库存](01-languages.md#并发扣库存) 改成 1 件库存和 20 次尝试 | 两边各 1 次成功，指出锁保护的两个操作 |
| 4 | [HTTP](02-http.md) 查商品、创建和重放订单 | 写下 `200`、`201`、`404` 各是哪次请求 |
| 5 | [订单测试](08-tests.md) 故意改错一个预期，再恢复 | 找到报错中的实际值与预期值 |
| 6 | [超时和重试](09-service-reliability.md) 给下游增加延迟 | 解释为什么一次请求的两个尝试共用总预算 |
| 7 | [数据库事务](03-database.md) 观察订单、库存、outbox | 写出三次写入的事务边界 |
| 8 | [隔离和锁](10-sql-performance.md#用两个连接观察锁等待) 开两个会话 | 普通读取成功，第二个事务等锁超时 |
| 9 | [查询计划](10-sql-performance.md) 比较两种分页 | 记录实际扫描行数，并说明为什么结果相同 |
| 10 | [缓存](04-cache.md) 观察命中、过期或回源 | 读两次商品，在日志找到 MISS 和 HIT |
| 11 | [Redis 限流](11-cache-reliability.md) 执行四次 Lua 脚本 | 验证第四次拒绝、过期后计数重新开始 |
| 12 | [基本投递](05-messaging.md) 暂停消费者再恢复 | 找到一条 `messages_ready` 增加后被处理的事件 |
| 13 | [消息路由](12-messaging-reliability.md) 发往不存在的队列 | 用 `routed:false` 解释 broker 确认和路由的区别 |
| 14 | [查请求日志](17-observability-and-probes.md) 后再 [跟踪订单](07-trace-an-order.md) | 先用请求 ID 找访问日志，再用订单 ID 找到事件 |
| 15 | [应用实例和故障](13-microservices-in-practice.md) 让两个 API 重放同一订单 | 两次响应的订单 ID 相同，库存只减一 |
| 16 | [Go 性能排查](14-go-profiling.md) 对比串行和并行 | 找出基准中的 ns/op，再用 mutex profile 解释结果 |
| 17 | [综合练习](15-capstone.md) 在独立栈验证订单和通知 | 留下接口响应和 SQL 证据 |
| 18 | [跑通订单分页](16-order-history-api.md)，再做 [商品筛选扩展](15-capstone.md#任务二给订单历史加商品筛选) | 能使用游标翻页，写出筛选和非法参数测试 |

可以把这张卡复制到自己的笔记，一项任务只写几行。

```text
第几次 / 我跑了哪条命令
我先预测
实际结果
我用哪段代码或日志解释
隔一天复习时，我能不能重新说一遍
```

## 合上代码能回答吗

<details>
<summary>1. 库存为 0 与商品不存在要返回什么？</summary>

商品存在时返回商品信息和 0 库存。查无商品时返回 404。Go map 读取缺失键也会给零值，可以用第二个返回值区分。
</details>

<details>
<summary>2. 客户端下单超时，为什么应该复用原来的幂等键？</summary>

超时发生时数据库可能已提交，客户端只是没收到响应。服务用原键找到已存订单并返回，避免再次扣库存。
</details>

<details>
<summary>3. 有索引时，为什么大 OFFSET 仍会变慢？</summary>

数据库仍得定位并越过前面的合格行。用稳定排序和上页末尾的游标，可从范围起点继续读。看实际计划的扫描行数确认。
</details>

<details>
<summary>4. 写库后也删了 Redis，为什么仍可能读到旧库存？</summary>

先读到旧数据的请求可能在删除之后才回填。TTL 限制旧记录的存活时间。下单的库存检查继续由数据库的条件更新保证。
</details>

<details>
<summary>5. 为什么 relay 收到确认，Java 还可能收到两条？</summary>

relay 确认后可能来不及更新 outbox。重启后再发同一事件。Java 用 event_id 的唯一约束去重，写成功后才 ACK。
</details>

<details>
<summary>6. 两个 API 进程为什么不能只用各自的 Mutex 防超卖？</summary>

Mutex 只能协调同一进程里的共享内存。多个进程需要共同遵守数据库里的条件更新、事务和唯一约束。
</details>

<details>
<summary>7. 只知道接口变慢，下一步看什么？</summary>

先观察请求量、错误率、延迟，再按链路判断卡在网络、等待数据库连接、SQL 执行、缓存还是消息。若热点在 Go 进程里，用 CPU、堆或锁 profile 对应定位。
</details>
