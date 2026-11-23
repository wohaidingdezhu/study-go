# Go / Java 代码知识加练

沿“查库存、保存快照、传递业务错误、结束一次调用”学习四组代码概念。每组先预测输出，再运行两种语言，最后沿订单服务找到同一种机制。

Day 4 先做值、引用、集合与接口部分，Day 10 再做错误和资源释放。每次可以只阅读一个函数，对应安排见 [逐日计划](day-by-day-study-plan.md)。

## 先运行，再对照注释

需要 Go 1.22+、JDK 21+，这两份示例只使用标准库。从仓库根目录运行。

```bash
(cd go && go run ./cmd/language-lab)
java java/src/main/java/study/LanguageLab.java
```

两边的预期输出相同，逐段阅读 [Go 示例](../go/cmd/language-lab/main.go) 和 [Java 示例](../java/src/main/java/study/LanguageLab.java)，比较实现方式。

```text
value copy: original=5 copy=4
shared reference: original=4
shallow snapshot: original=3
independent snapshot: original=3 copy=2
interface stock: 3
error direct: false
error cause: true
resource: use
resource: close
resource: returned
```

## 1. 值复制、指针与对象引用

| 操作 | Go | Java |
| --- | --- | --- |
| 得到独立的整数库存 | 结构体赋值 `valueCopy := original` 复制字段值 | 用 `new Inventory(original.stock)` 显式构造新对象 |
| 访问同一份库存 | `shared := &original` 保存变量地址 | `var shared = original` 复制对象引用 |
| 修改共享库存 | 通过指针更新原变量的字段 | 通过引用更新同一对象的字段 |

Go 的 `inventory` 在本例里只有整数，复制后各自修改互不影响。如果结构体里有切片、map 或指针，还要继续检查这些字段指向的数据。Java 把引用赋给另一个变量时，两者会指向同一对象。Java 的参数传递按值复制，传对象时复制的是引用值，方法通过它仍能访问同一对象。

接口把“能做什么”写成方法约定。Go 的 `inventory` 有 `available()`，就能赋给 `stockReader`；Java 的 `Inventory` 通过 `implements StockReader` 声明实现。调用者按接口使用读取能力，具体存储可以有不同实现。

**回到项目。** [Go Store](../go/order/store.go) 让内存和 PostgreSQL 提供相同方法；[Java Basics](../java/src/main/java/study/Basics.java) 的不同计价明细可实现 `Priced`。比较“按约定调用”与“创建具体对象”的位置。

**动手。** 把原始库存改成 8，先写出五行库存输出，再运行核对。合上代码解释为什么第一次副本扣减后原值仍为 8，而共享引用扣减后原值会变化。

## 2. 集合复制与快照

浅拷贝会复制集合里的元素。如果元素本身是指针或对象引用，新旧集合仍可能访问同一对象。

| 示例中的动作 | 会复制什么 | 后续修改的结果 |
| --- | --- | --- |
| Go 的 `slices.Clone(source)` | 切片元素里的指针值 | 通过新切片修改库存，原库存也变化 |
| Java 的 `new ArrayList<>(source)` | 列表元素里的对象引用 | 通过新列表修改库存，原对象也变化 |
| 逐个复制本例的库存对象 | 整数库存字段 | 修改副本后，原库存保持原值 |

示例中的独立快照针对只有整数的 `Inventory`。更复杂的嵌套对象，需要明确哪些字段共享、哪些字段独立复制。Java 的 `List.of` 限制列表结构修改，列表里已有的可变对象仍能被更新。

**回到项目。** [Go ListOrders](../go/order/store.go) 在锁内复制整数构成的 `Order` 值，然后解锁排序；[Java listOrders](../java/src/main/java/study/MemoryStore.java) 取得 record 列表快照后再筛选排序。解释本例这些订单字段为什么适合这种处理方式，再推演给订单加上可变明细列表后需要检查什么。

**动手。** 先预测，再暂时把独立复制改为直接复用已有元素。观察修改副本是否影响原库存，记录影响出现在哪一个引用上，最后恢复独立复制。

## 3. 业务错误、包装与原因链

底层返回“库存不足”，调用层还希望记录“预留商品 1 失败”。包装错误可以补充当前操作，并保留底层失败的原因。

| 要判断什么 | Go 示例 | Java 示例 |
| --- | --- | --- |
| 直接检查最外层 | 比较包装错误与原错误，结果为 `false` | 判断包装异常是否为 `Rejected`，结果为 `false` |
| 保留原因后识别业务失败 | `%w` 包装，`errors.Is` 沿错误链匹配 | 构造异常时传入 cause，本例用 `getCause()` 取得一层原因并检查类型 |

两边第一行的检查方式不同，分别是错误值比较与异常类型判断。它们共同说明，要处理被包装的业务失败，需要关注原因链。Java 示例只包装了一层，`getCause()` 也只返回直接原因；增加多层包装时，应继续沿原因链定位。

**回到项目。** [Go HTTP 层](../go/order/http.go) 用 `errors.Is` 映射业务错误；[Java HTTP 层](../java/src/main/java/study/OrderServer.java) 捕获当前实现抛出的 `Rejected`。给调用链增加包装时，要同步检查上层仍能识别原业务失败，以及最终 HTTP 状态是否符合契约。

**动手。** 给示例多加一层包装。Go 的 `errors.Is` 应仍能匹配原错误；Java 当前的一层 `getCause()` 检查会变化。解释原因，再指出怎样完整遍历。比较异常时关注错误类型与业务码，错误文字主要帮助诊断。

## 4. defer 与 try-with-resources

本例用打印模拟资源的使用和关闭，顺序应为“使用 → 关闭 → 调用者继续”。

| 机制 | 生命周期边界 | 阅读时检查什么 |
| --- | --- | --- |
| Go `defer resource.Close()` | 当前函数退出时执行 | 是否在成功取得资源后安排关闭，函数里长循环是否会积累过多待关闭资源 |
| Java `try (var resource = ...)` | 离开 try 代码块时调用 `close()` | 资源是否实现 `AutoCloseable`，代码块和异常处理覆盖了哪些操作 |

Go 的 `defer` 与普通花括号块的结束有不同的生命周期边界。Java 的 try-with-resources 在退出该 try 块时清理资源，即使后面还有其他语句。提前返回和异常路径也要考虑清理；关闭自身可能失败的真实资源还需要相应的错误处理。

**回到项目。** [Go 数据库查询](../labs/stack/store.go) 的 `defer rows.Close()`、[Java 通知消费者](../java/src/main/java/study/NotificationWorker.java) 的 JDBC 资源块，以及 [Java Agent 客户端](../java/src/main/java/study/AgentHistoryProbe.java) 的 `HttpClient` 生命周期，都是同一类问题。数据库连接、查询结果和客户端各有自己的关闭协议。

**动手。** 在 `useResource` 使用资源后增加提前返回，预测关闭行是否仍会出现，再运行。继续推演资源使用中途抛错的情况，画出关闭与错误返回的顺序。

## 读后端代码时的检查表

| 看到的代码 | 可以追问什么 |
| --- | --- |
| `:=` / `var` / `new` | 变量是什么类型，得到的是值、地址还是对象引用？ |
| `nil` / `null` | 这里代表缺失、可选结果还是未初始化？调用前怎样检查？ |
| map 查询 | 怎样区分零值与缺失键？是否允许保存空值？ |
| `append` / `add` / 集合复制 | 哪部分可以修改，哪些元素仍然共享？ |
| `return err` / `throw` | 谁处理这个错误，原因是否保留，怎样映射业务响应？ |
| 锁与快照 | 共享数据在哪里，检查与修改是否在同一个保护范围？ |
| `defer` / `try` | 资源在哪取得、在哪归还，失败路径怎样收尾？ |
| 接口调用 | 哪些能力由契约约定，具体实现从哪里传入？ |

## 记录与复习

完成四组实验后，保存两份相同输出与一个自己改变输入的结果，再闭卷回答每段变化的原因。

```bash
python3 scripts/study.py practice code-foundations
python3 scripts/study.py done code-foundations
python3 scripts/study.py review --limit 2
```

第一次只完成前两组时，可以先留笔记，Day 10 结合错误与资源部分完成整项验收。下一次复习先预测输出，再打开注释核对。
