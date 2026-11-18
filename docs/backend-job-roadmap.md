# Go × Java 后端招聘能力地图

下面选取 2026 年 11 月查到的公开职位。校招、社会招聘和资深岗位分别用来观察基础门槛、业务栈和进阶方向。

| 招聘样本 | 职位写明的能力 | 仓库里怎么验证 |
| --- | --- | --- |
| 小红书生态研发实习 | Java、Go 等语言，强调算法、软件工程、自动化测试和稳定性 | 先在 Go 和 Java 完成同一个订单契约，再用同一份自动化脚本验收。 [实习岗位](https://job.xiaohongshu.com/campus/position/18111?referer_code=QMT16MXVARJL) |
| 百度 Go 后端，J95446 | Go 服务开发、HTTP、MySQL 索引与锁、Linux 部署和问题定位。微服务和 Redis、Kafka 是加分方向 | 用 [HTTP 订单](02-http.md)、[查询计划与锁](10-sql-performance.md) 和 [探针日志](17-observability-and-probes.md) 各留一项可复现证据。 [百度招聘](https://talent.baidu.com/jobs/detail/SOCIAL/3c92f3ba-9347-4e16-9c2a-73a18bd2ee48) |
| 小红书电商 Java 后端 | Java、Spring Boot，以及 MySQL、Redis、RPC、消息队列等分布式组件 | 用 [Spring Boot + MySQL](../labs/spring-orders/README.md) 验证事务，再与 Go 的 PostgreSQL 订单实现对照。 [电商后端岗位](https://job.xiaohongshu.com/social/position/18216) |
| Wheely 资深 Go 后端 | Go 微服务、关系数据库、缓存和消息中间件。团队使用 gRPC、Kubernetes 和持续交付 | 先用 [通知链路和重投](12-messaging-reliability.md) 证明一个分布式行为，再按目标职位补 RPC 和容器编排。 [Wheely 招聘](https://job-boards.greenhouse.io/wheely/jobs/8016802003) |

## 推荐顺序

1. **能写接口。** 先用一种语言写请求校验、错误映射和测试，再对照另一种语言。应能从 HTTP 请求一路指出代码入口和数据库写入位置。 [订单接口](02-http.md)
2. **能证明数据正确。** 在真实数据库里复现并发抢库存、同键重试和写入中途失败。用订单行数、库存差和回滚后是否残留幂等键验收。 [Spring Boot 与 MySQL](../labs/spring-orders/README.md) · [Go 与 PostgreSQL](03-database.md)
3. **能查故障。** 先区分请求未到达、进程存活和依赖未就绪，再用同一个请求 ID 查日志。让 Redis 停机，观察缓存回源。暂停消费者，验证事件重投和最终一致。 [请求预算](09-service-reliability.md) · [缓存故障](04-cache.md) · [消息积压](12-messaging-reliability.md)
4. **按目标岗位深入。** Java 业务岗优先复盘 Spring MVC、依赖注入、事务、JDBC 和异常链。Go 平台岗优先练 `context`、连接预算、并发诊断与 RPC。Agent Infra 则从状态恢复、执行证据和发布回滚继续。 [Go 路线](go-roadmap.md) · [Agent Infra 路线](agent-infra-roadmap.md)

用一次小实验检查是否学会。先预测结果，再运行检查并写下实际证据。隔天闭卷画出失败时序，然后改变一个条件重跑。例如把「库存扣减后订单插入失败」换成「两个客户端拿同一个键重试」，分别说明哪个数据库约束在保护数据。 [学习工作台](study-system.md)
