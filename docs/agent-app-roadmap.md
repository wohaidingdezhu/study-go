# Go × Java × AI Agent 合并学习路线

用已有订单系统继续学习 AI Agent。你写过的 HTTP 接口可以成为工具，订单手册可以成为检索资料，请求超时可以成为 Agent 的运行预算，数据库与消息队列则支撑长任务和故障恢复。先跑通一个短循环，再逐步加上下文、评估和可靠性。

## 后端开发与单独做一个 Agent 应用的区别

| 对照点 | Go / Java 后端练习 | Agent 应用增加的工作 | 在本项目如何一起练 |
| --- | --- | --- | --- |
| 执行顺序 | handler 按代码规定的分支调用业务逻辑 | 模型根据输入与已有结果提出下一步，执行器检查后调用工具 | 对照订单 handler 与 `Model.Next → Engine.Advance` |
| 输入输出 | 路径、JSON、状态码、数据库约束 | 还要处理工具参数、模型返回格式、证据和终止条件 | 保留业务接口契约，另行校验模型提出的工具请求 |
| 正确性 | 用确定的样例断言订单、库存、错误码 | 还要检查工具选择、资料是否相关、回答是否有依据 | 同时记录工具轨迹、检索结果和最终回答 |
| 状态 | 一次请求或数据库中的业务记录 | 多轮对话、待调用工具、工具结果、检查点 | 从单次离线循环过渡到 PostgreSQL 检查点 |
| 失败恢复 | HTTP 超时、事务回滚、幂等、消息重投 | 模型超时、工具失败、上下文过长、重复执行 | 用请求预算和稳定幂等键保护工具副作用 |
| 权限 | 用户身份决定能访问哪些接口 | 工具调用也必须服从用户权限，检索内容可能夹带恶意指令 | 工具白名单、参数校验、调用者授权在服务端执行 |
| 发布 | 测试、部署、日志和回滚 | 还要固定模型、提示词、工具和资料版本，比较质量与成本 | 复用 Agent 版本评估、发布门禁和回滚 |

单独开发 Agent 应用通常先围绕一个使用场景，把模型、工具和回答串起来。合并学习把同一个场景从业务接口继续做到数据正确性、失败恢复和可观测性。Agent 应用开发与 Agent Infra 的工作也有侧重，前者关心任务是否完成，后者还关心大量任务怎样调度、恢复、隔离和发布。

工作流的步骤主要由代码预先规定，Agent 的下一步由模型参与决定。可以把模型决策限制在少数工具和明确的结束条件内，再根据评估结果扩大能力。 [Anthropic 的工作流与 Agent 工程说明](https://www.anthropic.com/engineering/building-effective-agents)

## 按能力接入同一个项目

下面是练习依赖顺序。完成一次 HTTP 对照后，就能穿插第一项 Agent 练习。数据库、缓存和队列继续沿订单主线推进。

| 阶段 | 同时练的后端内容 | Agent 内容 | 能留下的证据 |
| --- | --- | --- | --- |
| 1. 走通请求 | Go / Java 语法、HTTP 校验、错误码 | 输入、工具请求、工具结果、答复组成的短循环 | 一次正常轨迹和一次无匹配轨迹 |
| 2. 理解上下文 | JSON、集合、接口、错误处理 | 工具结果回填、检索排序、引用、长度限制 | 同一个问题在 v1 和 v2 中得到两条和三条证据 |
| 3. 判断能否交付 | 自动测试、状态码和业务断言 | 固定评估集、失败样本、工具权限、引用检查 | 合格版本通过，回归版本被拦截 |
| 4. 支撑长任务 | 事务、锁、并发、取消、幂等 | 检查点、租约代次、工具重复执行 | worker 崩溃后原任务被接手，旧 worker 的提交被拒绝 |
| 5. 跨服务协作 | 缓存、outbox、消息消费、探针和日志 | 工具超时、排队、任务历史、调用链 | 按任务 ID 找到工具请求与结果，说明副作用的去重位置 |
| 6. 接入真实模型 | HTTP 客户端、配置、连接预算 | 模型适配、工具调用格式、上下文预算和评估 | 真实模型按问题选择工具，结果回填后输出可核验答案 |
| 7. 按岗位深入 | RPC、容器、性能与交付 | MCP、混合检索、长期记忆、隔离执行、发布控制面 | 根据目标岗位补一个可以复现的工程场景 |

阶段 1–3 使用现有 Go 确定性模型适配器，可以离线练控制循环。它按固定规则选择手册工具。真实模型接入、向量检索、MCP 服务、长期记忆与操作系统级工具隔离列在后续里程碑，当前实现的能力边界见下文。

## 现在就能做的三项练习

### 1. 跟踪一个自选问题

在仓库根目录运行，需要 Go 1.26+。首次构建会下载 Go 模块，依赖准备好以后可离线演练。

```bash
(cd labs/stack && go run ./cmd/agent-runtime trace ops-runbook-v1 'Redis 下线后商品查询怎么办？')
```

先预测下一步，再看输出。`events` 应依次出现 `tool_requested`、`tool_completed`、`run_completed`。第一步给出工具名和查询，第二步保存手册片段，第三步给出回答和 `cache-fallback` 引用。数据在内存中推进，数据库检查点练习见第 4 阶段。

继续换一个输入，观察同一个工具返回零条证据时怎样结束。

```bash
(cd labs/stack && go run ./cmd/agent-runtime trace ops-runbook-v1 '如何烤面包？')
```

预期状态为 `succeeded`，答复说明手册无匹配，引用列表为空。这里的成功表示执行正常结束，业务上仍然需要用户补充相关问题。闭卷画出“决策 → 工具请求 → 结果回填 → 再决策”的顺序，指出输入和观察结果各在哪一步传给模型。

[Model、工具与执行循环](../labs/stack/agentruntime/engine.go) · [命令入口](../labs/stack/cmd/agent-runtime/main.go) · [轨迹采集](../labs/stack/agentruntime/replay.go)

### 2. 比较检索与上下文

用完全相同的问题比较两个版本。

```bash
(cd labs/stack && go run ./cmd/agent-runtime trace ops-runbook-v1 'Redis、PostgreSQL 和 RabbitMQ 如何排查？')
(cd labs/stack && go run ./cmd/agent-runtime trace ops-runbook-v2 'Redis、PostgreSQL 和 RabbitMQ 如何排查？')
```

v1 的检索上限是两条，预期引用 `cache-fallback`、`database-readiness`。v2 的上限是三条，还会得到 `outbox-backlog`。解释缺失原因时，先区分手册是否存在、检索是否命中、命中结果是否被截断、回答是否引用了结果。

当前实现用关键词计分检索，并把片段放回 `Checkpoint.Observations`。后续做 RAG 时，先练文档来源、切块、元数据过滤、关键词与向量召回、重排、上下文组装、答案验证。手册检索上限用于理解“选择哪些证据”，真实模型的上下文窗口还要计入指令、问题、历史、工具定义、工具结果和输出预留。

下一次刻意练习可给现有手册补一条与库存超卖有关的记录，先写预期引用 ID，再改 `runbooks` 并运行轨迹。Go 里比较 `map`、`slice` 和 `error`，Java 里用 `Map`、`List` 和异常表达同样的业务规则。

[检索与结果回填](../labs/stack/agentruntime/engine.go) · [v1 / v2 配置](../labs/stack/agentruntime/releases.go)

### 3. 用评估发现缺陷

```bash
(cd labs/stack && go run ./cmd/agent-runtime eval ops-runbook-v2)
(cd labs/stack && go run ./cmd/agent-runtime eval ops-runbook-regression)
```

第一条预期 `passed: true`。第二条预期 `passed: false`，进程非零退出。回归版本没有手册工具权限，可以用以下命令单独定位拒绝发生的步骤。

```bash
(cd labs/stack && go run ./cmd/agent-runtime trace ops-runbook-regression 'Redis 下线怎么办？')
```

预期 `tool_requested → tool_denied`，状态 `failed`，错误为 `tool_not_allowed`，进程非零退出。比较代码中的“工具已注册”和“当前版本允许调用”两项判断，并说明为什么模型提出调用不能直接等价于获得权限。

现有四个评估项覆盖 Redis 引用、队列引用、无匹配与工具越权。引用检查验证 ID 来自本次检索，仍需人工或额外评估检查回答是否真的得到证据支持。接真实模型后，保存代表性问题、预期行为、实际工具请求、答案、模型与提示词版本，并统计任务完成率、证据支持率、调用次数、耗时和使用量。对有波动的样本重复运行，单独记录检索失败、工具失败与回答失败。

[当前评估实现](../labs/stack/agentruntime/eval.go) · [后续发布与回滚](../labs/agent-runtime/README.md#发布与回滚)

## 用 Go 和 Java 各练什么

| 能力 | Go 入口 | Java 对照 |
| --- | --- | --- |
| HTTP 与 JSON 契约 | 订单 `handler` 与 `http.Client` | `OrderServer` 与 `HttpClient` |
| 工具接口 | `Model`、`Tool`、`ToolCall` 和 `Evidence` | 能用接口、record、List 表达相同契约，业务校验沿用订单规则 |
| 取消与预算 | `context` 向模型和工具传递取消 | 请求超时、线程中断和取消传播，核查调用库是否响应取消 |
| Agent API 调用 | `agentruntime` 的任务历史接口 | 运行已有 `AgentHistoryProbe` 验证分页和 ID 精度 |
| 真实模型与框架 | 先让适配器满足已有 `Model.Next` 契约 | 再用 Spring AI 对照工具声明、调用结果和应用授权 |

先选一边完成一个能力，再用同一输入验证另一边。需要学习框架时，Java 可从 Spring AI 的工具调用入口对照，应用仍负责工具权限与业务行为。 [Spring AI 工具调用](https://docs.spring.io/spring-ai/reference/api/tools.html)

## 真实模型接入的验收单

当前模型由 `RunbookModel` 的固定规则实现。下一步替换 `Model.Next` 时，应逐项验收这些行为。

1. 从问题、历史观察和工具定义构建请求，正确关联模型返回的调用 ID 与对应的工具结果。
2. 工具参数解析失败、工具不存在或越权时返回可追踪的失败，业务服务继续检查用户授权与参数。
3. 处理模型调用超时、取消、限流和服务错误，为总轮数、工具次数、时间、输出和费用设置预算。
4. 覆盖无证据、错误证据、上下文截断和恶意检索片段。来源文本中的指令按资料处理，权限由应用检查。
5. 固定测试集比较接入前后的结果，记录模型、提示词、工具、资料版本和实际使用量。费用按所选服务当前价格计算。
6. 写操作工具复用业务幂等契约，恢复执行前能查询已完成的外部操作。

会话历史、任务检查点和长期记忆有不同用途。会话历史帮助理解对话，检查点帮助恢复执行，长期记忆跨会话保存经筛选的信息，需要用户归属、读写权限、更新、删除与过期机制。资料检索与这三者相互配合，练习时分别说明保存了什么、何时读取、谁可以访问。

MCP 用统一协议连接宿主应用、客户端与工具或资源服务器。先掌握本地工具契约，后续再把现有只读查询做成 MCP 服务，对照发现、调用、传输和授权边界。 [MCP 官方架构](https://modelcontextprotocol.io/docs/learn/architecture)

## 记录与复习

在 React 工作台「Agent 应用与上下文」中打开任务，或从命令行查看。

```bash
python3 scripts/study.py practice agent-loop
python3 scripts/study.py practice agent-context
python3 scripts/study.py practice agent-evaluation
```

每项做完后用对应的 `done` 命令记录实际事件顺序、引用和失败原因。首次配合注释走一遍，下一次合上源码预测，再换问题或版本验证。复习仍使用现有的 `1、3、7、14、30` 天间隔。

完成短循环后，接着做 [持久化、取消与恢复](../labs/agent-runtime/README.md#验收)。已有的 `agent-runtime` 和 `agent-release` 任务用于第 4、5 阶段及发布验收。岗位取向与更深入的基础设施能力见 [Agent Infra 路线](agent-infra-roadmap.md)。
