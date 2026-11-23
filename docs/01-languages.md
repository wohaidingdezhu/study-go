# 01 · Java / Go 语法对照

先从同一条规则开始：每件商品的小计是单价乘数量，数量必须大于零。单价用整数分表示，避免入门例子受浮点舍入影响。

```bash
java java/src/main/java/study/Basics.java
(cd go && go run ./cmd/basics)
```

```text
total: 2700 cents
category: Stationery
category missing: Pen
invalid item: quantity must be positive
```

| 要写什么 | Java | Go |
| --- | --- | --- |
| 局部变量 | `int sum = 0` 或 `var sum = 0` | `var sum int = 0` 或 `sum := 0` |
| 分支 | `if (quantity <= 0) { ... }` | `if quantity <= 0 { ... }` |
| 遍历集合 | `for (Priced item : items)` | `for _, item := range items` |
| 定义数据 | `record Line(String name, int unitCents, int quantity)` | `type Line struct { ... }` |
| 方法 | 定义在类或 record 中 | 给函数声明接收者，例如 `func (line Line) subtotal()` |
| 接口 | 声明 `implements Priced` | 方法集匹配后隐式满足接口 |
| 列表 | `List<Priced>`，可用 `ArrayList` 增删 | `[]Priced` 是切片，通过 `append` 增长 |
| 字典 | `Map<String, String>`，用 `containsKey` 判断存在 | `value, ok := categories[key]`，缺失返回零值和 `false` |
| 错误 | `throw new IllegalArgumentException(...)`，调用方可 `catch` | 返回 `(int, error)`，调用方显式处理 `err != nil` |
| 资源释放 | `try (var resource = ...)` 调用 `close()` | 成功拿到资源后 `defer resource.Close()` |

看 [Java Basics](../java/src/main/java/study/Basics.java) 和 [Go basics](../go/cmd/basics/main.go) 的 `total`。Java 的 `List<? extends Priced>` 能接收 `List<Line>`，并且可以把元素读成 `Priced`。Go 的 `[]Line` 与 `[]Priced` 是不同的类型，即使每个 `Line` 都实现了接口，仍需要逐个赋值或直接构造 `[]Priced`。

Go 切片复制的是描述切片的数据，两个切片可能共享底层数组。`append` 可能分配新数组，要接住返回值。Java 的 `List.of` 返回不可修改的列表。两种语言都需要区分集合本身的可变性和内部元素的可变性。 [Effective Go](https://go.dev/doc/effective_go) · [Java Learn](https://dev.java/learn/)

想把这些概念变成可观察的结果，可以继续做 [代码知识加练](code-foundations.md)，运行同一组值复制、集合快照、错误包装和资源释放实验。两份代码均在关键位置提供中文注释。

## 库存为零和查无商品

试着先预测输出。商品 1 有 5 件，商品 2 已售罄，商品 999 没有录入。然后运行两版代码。

```bash
(cd go && go run ./cmd/catalog)
java java/src/main/java/study/Catalog.java
```

```text
product 1: stock=5
product 2: stock=0
product 999: not found
original after shared update: 4
original after copy update: 4
copy after copy update: 3
```

| 观察 | [Go](../go/cmd/catalog/main.go) | [Java](../java/src/main/java/study/Catalog.java) |
| --- | --- | --- |
| 空库存与查无商品 | `stock, exists := stockByID[id]`。缺失时得到 `0, false` | `Integer stock = stockById.get(id)`。本例只存整数，`null` 表示没找到 |
| 通过另一个变量修改原表 | `sharedStock := stockByID` 让两个变量访问同一个 map | `var sharedStock = stockById` 复制引用，让两个变量指向同一个对象 |
| 复制后各自修改 | `maps.Clone(stockByID)` | `new HashMap<>(stockById)` |

两边复制的都是条目。如果值是可改的对象或指针，复制后还可能共享内部数据。这里的库存值是整数，所以可以独立修改。这个判断对应第 02 章的 API 行为：有商品且库存为 0 时，查询可以返回 `200` 和 `"stock":0`，查无商品时返回 `404`。

最后两行用商品名「笔记本📒」比较字符串长度。它有 4 个 Unicode 码点，编码成 UTF-8 占 13 个字节。Go 的 `len(name)` 按字节计数，Java 多打印的 `String.length()` 按 UTF-16 代码单元计数，结果是 5。后端给字段设长度限制时，先确定要限制字节数还是码点数。 [Go 字符串、字节和 rune](https://go.dev/blog/strings) · [Java String 文档](https://docs.oracle.com/en/java/javase/21/docs/api/java.base/java/lang/String.html)

**练一下**。把商品 2 从初始库存表中删掉。观察哪一行从 `stock=0` 变成 `not found`。再把商品名称改成「📒」，预测 UTF-8 字节数、Unicode 码点数和 Java 的 `length()`，运行两版核对。

用 [库存查询练习](../exercises/stock-lookup/README.md) 亲手补一次 map 查询，比较两种语言的测试怎样区分缺失和库存为零。

## 并发扣库存

```bash
java java/src/main/java/study/Concurrency.java
(cd go && go run ./cmd/concurrency)
```

预计输出 `succeeded: 3, rejected: 7, stock: 0`。

| 操作 | [Java](../java/src/main/java/study/Concurrency.java) | [Go](../go/cmd/concurrency/main.go) |
| --- | --- | --- |
| 启动任务 | 虚拟线程执行器提交任务，每次得到一个 `Future` | `go func() { ... }()` 开一个 goroutine |
| 等结果 | `Future.get()` 等待任务结果 | channel 传结果，`WaitGroup` 等待全部任务结束 |
| 保护库存 | `synchronized` 持有当前实例的监视器锁 | `sync.Mutex` 保护读库存、判断和扣减 |
| 更新同一个值 | 任务共享 `Inventory` 对象引用 | 指针接收者 `*Inventory` 修改同一份库存 |

虚拟线程和 goroutine 都让并发任务更轻量。它们的调度、取消和等待机制各有自己的 API。锁在同一个进程里协调访问，第 03 章会把这个责任交给数据库。 [Java 虚拟线程](https://docs.oracle.com/en/java/javase/21/core/virtual-threads.html) · [Go 并发](https://go.dev/tour/concurrency/1)

### 练习

1. 新增一种实现 `Priced` 的运费，固定 500 分。把它放进同一组明细，验证总额变为 3200 分。
2. 把库存改成 1，并发任务改成 20。预期只有 1 个成功。说出 Go 示例为什么由主 goroutine 在 `Wait()` 后关闭 channel。
3. 用 Go 的 `go run -race ./cmd/concurrency` 观察竞态检测。临时拿掉锁再多跑几次，然后恢复锁。

继续阅读 [并发、等待与取消](concurrency-and-cancellation.md)，解释为什么先提交任务再逐个取结果仍可并发，以及 channel 缓冲容量与等待顺序怎样配合。
