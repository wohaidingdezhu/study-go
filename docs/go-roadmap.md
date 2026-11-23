# Go 后端学习路线

用同一个订单项目练 Go 后端。Java 对照帮助理解数据类型、接口、错误处理和并发，第 05 章以后还可以跟着 Java 消费者看跨语言协作。每个学习阶段做一个小实验，再用自己的话解释结果。可以按 [18 次学习任务](study-workbook.md) 从今天开始。

AI Agent 可以从 HTTP 阶段穿插学习。先在同一套订单手册上练工具循环、检索与评估，再用事务、取消和幂等知识完成长任务与恢复。每一项都能在工作台保存证据并加入间隔复习。 [Go × Java × AI Agent 合并路线](agent-app-roadmap.md)

## 招聘样本提供了哪些方向

下面是 2026 年 11 月 16 日查阅的雇主公开职位样本，覆盖国内业务岗位和海外后端岗位。课程据此安排技术顺序。工作年限、岗位是否仍开放和地点以招聘页为准。

| 职位样本 | 页面提到的技术能力 | 对应的练习 |
| --- | --- | --- |
| 百度 Go 后端开发工程师，AI 生态方向，北京 | Go/C++、数据结构和算法、数据库、消息队列、缓存、分布式开发 | 写 Go 接口，跟一次事务、缓存和通知事件。 [职位 J97847](https://talent.baidu.com/jobs/detail/SOCIAL/c026b4e1-5d61-4337-8338-ef563c6a2c38) |
| 百度后端开发工程师，上海 | Go 服务端开发、REST/HTTP/JSON、MySQL 索引和锁、Linux 部署与问题定位。微服务和 Redis/Kafka 等是加分方向 | 发请求、看日志，做查询计划和锁等待实验，再比较 PostgreSQL 与 InnoDB。 [职位 J95446](https://talent.baidu.com/jobs/detail/SOCIAL/3c92f3ba-9347-4e16-9c2a-73a18bd2ee48) |
| Avantos AI Backend Engineer | Go/Python、PostgreSQL 模型与查询、Kafka 事件处理，另列 Docker/Kubernetes 与微服务经验 | 做分页和消息重放，解释 Kafka 分区与消费进度。 [职位原文](https://jobs.ashbyhq.com/avantos/9a8a7a1f-53ca-454e-8703-095ea366a14e) |
| Roadie Senior Backend Engineer | Go/Ruby、API 设计和分布式系统，团队使用 PostgreSQL、Redis、Kafka、Docker、Kubernetes | 从单个进程扩展到两个 API，找出共享数据和故障边界。 [职位原文](https://job-boards.greenhouse.io/roadie/jobs/8786983002) |

这组样本里反复出现的是服务开发、数据存取和问题定位。PostgreSQL、MySQL、RabbitMQ、Kafka 等产品随团队而异。先在项目里做出可复现的证据，再针对常见栈迁移原理。岗位常提到的数据结构与算法可以并行复习：先用 map 查幂等键，用队列理解 FIFO 和重投，用排序和二分解释分页范围，再写出各自的时间复杂度。

## 按能完成的任务推进

| 阶段 | 小课程 | 动手任务 | 通过条件 |
| --- | --- | --- | --- |
| 1 · Go 和 HTTP | [01 语法对照](01-languages.md)、[02 HTTP](02-http.md) | 比较 Go/Java 的空值和错误处理，亲手下单并重试 | 能指出 Go handler、请求体、状态码和库存各在哪里。能用 `go run`、`go test` 和 `curl` 独立跑一轮 |
| 2 · 测试和网络边界 | [08 自动测试](08-tests.md)、[09 超时与重试](09-service-reliability.md) | 改错一个断言并恢复，跑正常、503 和慢响应三种场景 | 能解释 goroutine、锁和 context 各负责什么，说明为什么只重试一次 |
| 3 · 数据库 | [03 事务](03-database.md)、[10 查询计划、分页与锁](10-sql-performance.md) | 验证并发扣减，比较复合索引和两种分页，观察锁等待 | 能指出 SQL 实际扫了多少行，解释隔离级别、唯一键和条件更新怎样协作 |
| 4 · 缓存和消息 | [04 缓存](04-cache.md)、[11 缓存限流](11-cache-reliability.md)、[05 队列](05-messaging.md)、[12 投递语义](12-messaging-reliability.md) | 跑一次缓存回源，看到第 4 次请求被限流，暂停消费者再重放订单事件 | 能用日志与 SQL 区分缓存故障、发布阻塞、消费者积压和重复投递 |
| 5 · 微服务和分布式 | [06 故障恢复](06-distributed.md)、[07 跟踪订单](07-trace-an-order.md)、[13 多实例](13-microservices-in-practice.md) | 用两个 API 发送同键请求，跟踪订单到通知，推演支付失败补偿 | 能解释共享数据库的角色，画出事务边界、同步或异步链路，以及组件停止后的结果 |

每次只做一行里的一个小任务。只有 15 分钟时，用 5 分钟读原理、8 分钟跑一个命令、2 分钟记一条结果。能投入 40 分钟时，再改一个变量做第二次对照。复习时先合上文档回答「之前看到什么」「为什么发生」「线上会用什么数据确认」。

## 跟着目标岗位增加复习范围

| 方向 | 先掌握什么 | 下一步可以怎样迁移 |
| --- | --- | --- |
| 业务 Go 后端 | HTTP、参数校验、分层、SQL、Redis、测试 | 把一个 `net/http` handler 用团队框架重写，保留请求/响应约定、业务规则和测试 |
| 高并发后端 | `context`、锁、`-race`、重试预算、查询计划 | 做 [第 14 章的基准实验](14-go-profiling.md)，用 Go 的 profile 定位 CPU 或锁等待，再尝试分析内存。复习 goroutine 调度、GC、逃逸和连接池，形成可验证的解释 |
| 云原生与基础设施 | Linux 进程、端口、DNS 和容器日志，服务发现与健康检查 | 阅读 Kubernetes Deployment、Service 和 probes。再根据岗位补 gRPC、发布回滚或一致性协议 |
| 数据和事件平台 | SQL 调优、消息偏移量、积压、重放和幂等 | 把订单事件按 `orderId` 映射到 Kafka 分区，设计 consumer group、offset 提交与回放步骤 |

学完一阶段，选一条证据记在自己的笔记里，例如「同键跨两个 API 实例只生成一笔订单」「同一页查询分别从索引读取了 820 行和 20 行」。面试复盘时，从业务约束和实验结果说起，再解释适用范围和代价。

最后用 [综合练习](15-capstone.md) 验收。先证明已有的订单和通知链路，再为 Go 的订单分页实现商品筛选。
