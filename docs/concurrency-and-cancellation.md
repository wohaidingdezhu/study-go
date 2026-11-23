# Go / Java 并发、等待与取消

用两组代码串起“启动任务 → 等待结果 → 超时或失败 → 收尾”。第一组复用已有的并发库存示例，第二组用计时器和线程等待模拟一个依赖，观察取消怎样传到工作代码。两组均使用标准库，代码关键位置有中文注释。

Day 11 先看任务启动和结果汇总，Day 38 再练取消与预算。相关安排见 [逐日计划](day-by-day-study-plan.md)。

## 先把几个词对应到代码

| 概念 | 在本项目里怎样观察 |
| --- | --- |
| 并发 | 多个任务在同一时间段内推进，可以交替运行，也可以等待 I/O |
| 并行 | 多个任务在同一时刻执行，是否发生与线程调度及可用执行资源有关 |
| 阻塞等待 | 调用者在接收 channel、调用 `Future.get` 或 `await` 时等待结果 |
| 异步提交 | 先启动或提交任务，调用者随后通过 channel 或 Future 获取结果 |
| 互斥 | 同一份库存的检查与修改由锁保护，一次只允许一个执行者进入临界区 |
| 协作式取消 | 执行代码检查 context、响应线程中断或使用支持取消的下游调用 |

异步提交之后仍然可以阻塞等待结果。阅读代码时，分别标出“何时提交”“何时开始等待”“谁修改共享状态”。

## 1. 复用并发库存实验

从仓库根目录运行，需要 Go 1.22+、JDK 21+。

```bash
(cd go && go run ./cmd/concurrency)
java java/src/main/java/study/Concurrency.java
```

两边默认各启动 10 次库存预留，初始库存为 3，预期输出如下。

```text
succeeded: 3, rejected: 7, stock: 0
```

按下表阅读 [Go Concurrency](../go/cmd/concurrency/main.go) 与 [Java Concurrency](../java/src/main/java/study/Concurrency.java)。

| 动作 | Go | Java |
| --- | --- | --- |
| 共享库存 | 所有 goroutine 使用同一份 `inventory` | 所有任务引用同一 `Inventory` 对象 |
| 启动任务 | `go func()` | `executor.submit()` |
| 等待完成 | `WaitGroup.Wait()` | `Future.get()`，执行器退出时也会等待关闭流程 |
| 取得结果 | 从结果 channel 读取 | 从 Future 读取返回值 |
| 保护库存 | `Mutex` | `synchronized` |

Go 的 WaitGroup 负责等待任务结束，结果由 channel 传递，锁负责共享数据访问。Java 把返回结果和异常交给 Future，库存的原子检查与修改仍由 `synchronized` 保护。把工作交给虚拟线程或 goroutine 后，业务约束依然需要明确的并发保护。

### 为什么取结果的循环可以逐个等待

两边都先把所有任务提交出去，再汇总结果。Java 后面的循环逐个 `get()`，任务可以早已在同时推进。若把“提交任务”和“等待该任务”写在同一个循环里，再开始下一个任务，才可能让整个流程变成逐个推进。

### Go 的缓冲区与发送完成信号

本例先 `Wait()`，后读 channel，每个任务发送一次结果。channel 的容量等于任务数，所有结果都能放入缓冲区，发送者才可以结束并调用 `Done()`。

如果把 channel 容量改成 0，而保持这个等待顺序，发送者会等待接收者，主 goroutine 又在等待发送者结束，程序会卡住。可通过并发接收、在发送者全部结束后关闭 channel 等方式重新安排汇总；修改时同时核对发送次数、缓冲容量与关闭时机。

**动手。** 把两版初始库存改成 1、尝试次数改成 20，预测后运行。预期恰好一次成功，最终库存为 0。画出锁、任务结束与结果汇总的位置，再恢复原始参数。

## 2. 观察一个任务的成功、超时与失败

运行新示例，先从成功路径开始。

```bash
(cd go && go run ./cmd/cancellation-lab)
java java/src/main/java/study/CancellationLab.java
```

两边在当前演示参数下预期输出相同。

```text
result: stock=5
worker cleanup: finished
```

再让依赖等待 2 秒，而调用方的演示预算为 200 毫秒。

```bash
(cd go && go run ./cmd/cancellation-lab -mode timeout)
java java/src/main/java/study/CancellationLab.java timeout
```

| 程序 | 常规运行下的预期输出 | 代码中的关键步骤 |
| --- | --- | --- |
| Go | `result: timeout`，随后 `worker cleanup: finished` | context 到期，工作函数的 `select` 响应取消并返回 |
| Java | `result: timeout`、`cancel requested: true`，随后 `worker cleanup: finished` | `get` 等待超时，调用方请求取消，工作线程响应中断并执行 `finally` |

最后比较依赖主动失败。

```bash
(cd go && go run ./cmd/cancellation-lab -mode failure)
java java/src/main/java/study/CancellationLab.java failure
```

两边预期输出如下。这里的失败由模拟依赖主动产生，与等候超时具有不同来源。

```text
result: dependency_failed
worker cleanup: finished
```

[Go 取消示例](../go/cmd/cancellation-lab/main.go) · [Java 取消示例](../java/src/main/java/study/CancellationLab.java)

## 3. 取消怎样传到工作代码

### Go 的 context

`context.WithTimeout` 创建截止时间，`ctx.Done()` 在到期或显式取消后变为可读取。工作函数通过 `select` 接收取消，返回 `ctx.Err()`，调用方据此识别超时。context 的信号需要下游主动响应，执行中的任意函数都有自己的退出路径。

示例中的结果 channel 交付一次结果，`finished` channel 单独表示工作函数已经走完本例的收尾。接收结果与等待收尾分别写出，便于观察两种生命周期。

**接真实 HTTP。** [已有 Go service-lab](../go/cmd/service-lab/main.go) 把 context 传给 `http.NewRequestWithContext`，并在模拟服务端用请求 context 结束等待。按 [第 09 章](09-service-reliability.md) 验证正常、503 重试与超时。

### Java 的 Future 与线程中断

`future.get(200, ...)` 限制调用方这一次等待的时长。它抛出 `TimeoutException` 后，工作任务仍可能继续执行，调用方需要根据业务决定是否请求取消。

示例使用 `future.cancel(true)` 请求取消并允许中断，`Thread.sleep` 响应中断抛出 `InterruptedException`，工作函数随后执行 `finally`。取消请求返回 `true` 表示 Future 接受了取消，工作代码的收尾仍要分别确认。`Future.isDone()` 也包含取消状态。

示例用 `started` 确认任务已启动后，再开始 `get` 的等待预算，用 `finished` 确认 `finally` 已执行。整个调用的排队、启动和后续等待时间，还需要放入统一的业务预算。执行器的 try-with-resources 退出时会等待终止流程，工作任务怎样响应中断会影响退出耗时。

**动手。** 在纸上推演工作函数长时间忽略取消时，两边会停在哪一步，调用者等待与工作任务的状态分别是什么。再到 [Go Agent worker](../labs/stack/agentruntime/worker.go) 找到单步预算、检查点提交和旧租约拒绝的位置。

## 4. 多个任务怎样共用总预算

假设一次请求总预算 500ms，前一段已耗时 400ms，后一段的等待预算要从剩余约 100ms 中分配。Go 可传递同一个带截止时间的 context，Java 可使用单调时钟记录绝对截止时间，再计算剩余时间。

下面是 Java 中“同一截止时间、剩余等待”的写法。deadline 在整次调用开始时建立，每次等待前重新计算。

```java
// budgetNanos 是整次调用的预算，deadline 只在开始时计算一次。
long deadline = System.nanoTime() + budgetNanos;
// 后续等待前计算剩余时间，避免每个阶段重新得到一整份预算。
long remaining = Math.max(0L, deadline - System.nanoTime());
var value = future.get(remaining, TimeUnit.NANOSECONDS);
```

这段写法控制等待时间，任务执行的取消与副作用处理仍需按具体 API 设计。Go 的真实 HTTP 练习已经把首次请求、重试间隔和第二次请求放入同一 context，可直接观察预算怎样耗尽。

## 5. 超时之后，业务结果怎样确认

客户端没拿到结果时，服务器可能已经提交订单、发布事件或完成扣款。停止等待、取消本地任务与撤销已经发生的业务动作有各自的边界。

| 项目场景 | 还需要检查什么 |
| --- | --- |
| 下单响应超时 | 用原幂等键与原参数重试或查询原订单，核对库存只扣一次 |
| 消费者写库后断线 | 消息可能重投，按事件 ID 与唯一约束去重 |
| Agent 工具完成但检查点未提交 | 接手后可能再次调用工具，写操作需要稳定幂等键并能查回结果 |
| Agent 取消与结果提交竞争 | 按持久化终态与租约条件拒绝迟到提交，已经发出的外部操作需要自己的结果确认 |

[订单幂等](02-http.md) · [消息确认与重投](05-messaging.md) · [Agent 取消与恢复](../labs/agent-runtime/README.md)

## 记录与复习

先预测三种模式，再运行 Go 和 Java。保存正常、超时与主动失败的输出，画出等待结束和工作代码收尾的位置。最后到真实 HTTP 或 Agent worker 中标出对应机制。

```bash
python3 scripts/study.py practice async-cancellation
python3 scripts/study.py done async-cancellation
python3 scripts/study.py review --limit 2
```

闭卷时选一个变式。把两次等待各重开 200ms 会怎样，去掉工作函数的取消响应会怎样，取消后已经完成的数据库提交又该怎样确认。每次只改变一个条件，用时间线解释结果。
