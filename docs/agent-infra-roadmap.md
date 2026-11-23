# AI Agent Infra 岗位路线

先从 [Go × Java × AI Agent 合并路线](agent-app-roadmap.md) 掌握工具调用、检索、上下文和评估，再练本页的持久化与平台能力。完成 HTTP 入门后就能用 `trace` 跟踪自选问题，随后把订单服务的超时、幂等和事务知识用于 Agent 工具及任务恢复。

以小红书 RAI Studio 的 2027 校招岗位为主参照。该职位覆盖执行内核、托管运行时、上下文与记忆、工具权限和隔离、控制面、评估与迭代。另一条企业 Agent 岗位也强调会话、状态持久化和可观测性。项目优先沿着执行、恢复、评估、发布和回滚做出可复现的验证。 [RAI Studio AI Agent Infra](https://job.xiaohongshu.com/campus/position/22062?referer_code=BSWH0FUOCMNF) · [企业 Agent 基础设施岗位](https://job.xiaohongshu.com/campus/position/22074?referer_code=1HFWERNHLB1B)

Go 作为运行时主语言，练接口、`context`、并发、PostgreSQL 事务和故障诊断。Java 用同一业务练 HTTP 契约、JUnit、JDBC 和 RabbitMQ 消费，再用 Java 客户端验证 Agent API 契约。这是本项目的语言分工。涉及 Java 应用落地时，可对照招聘说明中对 Java、计算机基础、Agent 框架和工程交付的要求。 [Java 方向 AI Agent 岗位](https://job.xiaohongshu.com/campus/position/21969)

## 按验证产出推进

| 优先级 | 岗位能力 | 仓库入口与验收 |
| --- | --- | --- |
| 1 | 后端基础与分布式正确性 | [学习系统](study-system.md)。用同一用例验证 Go 和 Java 接口，在两个进程竞争库存时说明数据库条件更新如何保护库存，并复现消息重投去重。 |
| 2 | Harness 与持久化运行时 | [Agent Runtime](../labs/agent-runtime/README.md)。沿一个任务 ID 读检查点和事件，强制终止 worker 后证明能接手，恢复旧进程后看到代次校验拒绝旧提交。 |
| 3 | 评估、控制面与回滚 | [版本演练](../labs/agent-runtime/README.md#发布与回滚)。让带回归的候选无法发布，切换合格版本，再回滚。用两个同输入任务的不同结果证明任务绑定了各自版本。 |
| 4 | 模型适配、上下文与评估集 | 已有 [离线入门练习](agent-app-roadmap.md#现在就能做的三项练习) 对照工具循环、关键词检索上限、四项评估和引用检查。真实模型适配、工具选择与上下文截断评估继续按合并路线的验收单补齐。 |
| 5 | 工具隔离和跨服务运行 | 后续扩展。将工具放到受限进程，验证越权文件、网络和凭证访问被拦截。让 Java 客户端验证 Go API 契约和失败响应，再贯通跨服务追踪。 |

质量目标以实验为准。能说清 Go 的 `context` 会怎样取消下游，PostgreSQL 的租约为什么需要代次，以及 RabbitMQ ACK 前后崩溃的差别。上述三种场景分别用慢网络、暂停进程和重投消息验证。之后把复现代码、评估报告和失败案例一起保留，便于复盘。

## 复习入口

```bash
python3 scripts/study.py practice agent-runtime
python3 scripts/study.py practice agent-release
python3 labs/agent-runtime/check.py
```

完成一次练习后使用 `done agent-runtime` 或 `done agent-release` 保存你实际观测到的记录，然后用 `review` 从记忆重画时序。画完把故障时机提前或推迟一步，再预测状态、事件和重试会怎么变。
