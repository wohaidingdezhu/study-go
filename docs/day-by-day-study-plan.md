# Go × Java × Electron × AI Agent 逐日学习计划

以本仓库的订单系统为主线，按 56 个学习日完成第一轮实践。默认每天投入 60–90 分钟，每 7 个学习日安排一次复盘。计划从后端初学者出发，把 Go 与 Java 放在同一业务里对照，再连接桌面 IPC、数据库、缓存、消息队列和 Agent 运行时。

Day 1 表示你开始执行这份计划的第一天。已经掌握的内容可以先做验收，卡住时把当天拆成两次继续，顺序按实际掌握情况推进。首次安装、下载依赖和构建镜像可能需要额外时间。

## 每天怎么执行

| 时间 | 做什么 | 留下什么 |
| --- | --- | --- |
| 10 分钟 | 闭卷回答到期复习题，再揭晓答案 | 一处说错或解释不清的地方 |
| 15 分钟 | 阅读当天指定的一小段材料和源码注释 | 概念含义、代码位置和一个预测 |
| 30 分钟 | 运行实验，改一个输入、断言或故障条件 | 响应、日志、测试或 SQL 结果 |
| 5 分钟 | 对照验收条件，用自己的话解释原因 | 一条实际证据和一个待解决问题 |
| 额外 30 分钟 | 换一种语言重做，或验证一个边界条件 | 同一行为的第二份证据 |

只有 15 分钟时，复习两张卡，再完成当天的一次最小操作。当天动手内容超过预算，就从同一步顺延，最后保留实际结果。

每次先读概念所在位置的注释，再合上源码预测。需要查术语时，打开 [概念与源码导航](07-trace-an-order.md#从概念找到源码)。Go 与 Java 对照优先观察参数、返回值、空值、错误、集合和并发边界。

## 环境与命令约定

所有命令从仓库根目录执行，也就是包含 `go`、`java`、`labs` 和 `docs` 的目录。

| 工具 | 这份计划的要求 | 何时使用 |
| --- | --- | --- |
| Go | 完整计划使用 1.26+，入门模块本身要求 1.22+ | Day 1 起，进阶与 Agent 模块要求 1.26+ |
| Java | JDK 21+，配合 Maven | Day 1 起 |
| Python | 3.9+ | 学习记录和实验验收 |
| Node.js / npm | Node.js 22.12+ 可覆盖工作台与 Electron 要求 | 工作台和 Day 12 |
| Docker Engine / Compose | 引擎能连接，Compose 命令可运行 | Day 15 起 |

先执行环境检查，具体安装与排错见 [项目说明](../README.md#检查学习环境)。

```bash
bash scripts/study-doctor.sh
```

需要时启动 [React 学习工作台](study-system.md#react-工作台)，在页面记录证据并复习。初次运行先安装和构建，后续可以直接启动。

```bash
bash scripts/study-doctor.sh --profile web
npm --prefix web ci
npm --prefix web run build
npm --prefix web start
```

| 服务或环境 | 默认地址 | 用途 |
| --- | --- | --- |
| 学习工作台 | `http://127.0.0.1:4173` | 学习材料、证据和间隔复习 |
| Go 内存订单服务 | `http://127.0.0.1:8080` | 入门接口与 Go / Java 对照 |
| Java 内存订单服务 | `http://127.0.0.1:8081` | 入门接口与 Go / Java 对照 |
| 独立订单练习栈 | `http://127.0.0.1:18082` | PostgreSQL、Redis、RabbitMQ 和 Java 通知 |
| Spring Boot / MySQL | `http://127.0.0.1:18085` | Java 框架与事务对照 |
| Agent Runtime | `http://127.0.0.1:18084` | 持久化任务、事件与恢复 |

数据库、缓存、队列与故障实验使用 [独立订单练习栈](15-capstone.md#启动独立练习环境)。阅读章节中 `8082` 的例子时，练习栈对应 `18082`，容器命令使用 `bash scripts/practice-stack.sh`。涉及第二个 API 实例时，另选空闲端口并沿用同一个练习项目。

内存服务重启会恢复初始数据，数据库卷会保存之前的订单。要求初始库存为 5 的验收脚本应按对应章节准备新练习数据，先保存需要的实验记录。

每次独立下单实验使用新的幂等键，同一笔请求的重试保留原键和原参数。下单前先查询并记录实际库存，库存耗尽时按独立环境章节重新准备练习数据，再继续后面的实验。

## 学习路线总览

| 阶段 | 学习日 | 主要产出 |
| --- | --- | --- |
| Go / Java 基础与第一条请求 | Day 1–7 | 语法对照、库存查询、商品接口 |
| 接口正确性与两种调用边界 | Day 8–14 | 下单、幂等、测试、并发、IPC 和首次 Agent 轨迹 |
| PostgreSQL 与数据正确性 | Day 15–21 | 表、约束、事务、锁和查询计划 |
| Java 工程化与分页 | Day 22–28 | Spring Boot / MySQL 对照、回滚和游标分页 |
| 缓存与消息链路 | Day 29–35 | 缓存回源、限流、outbox、消费确认和去重 |
| 微服务、可靠性与诊断 | Day 36–42 | 多实例、超时预算、探针、日志和性能证据 |
| 独立交付与 Agent 应用 | Day 43–49 | 商品筛选变更、工具循环、检索与评估 |
| Agent Infra 与综合验收 | Day 50–56 | 检查点、恢复、取消、历史、发布与回滚 |

## 第一阶段 · Go / Java 基础与第一条请求

| 学习日 | 学什么、看哪里 | 当天动手 | 验收与闭卷问题 |
| --- | --- | --- | --- |
| **Day 1** | 环境、项目目录、程序入口。[开始运行](../README.md#先跑一段语法对照) | 运行环境检查和两版购物车。找到 Go 的 `main` 与 Java 的 `main`，记录各自的启动方式。 | 两边输出 `2700 cents`。能从入口指到计算小计的函数。 |
| **Day 2** | 变量、数据类型、函数参数、返回值、结构体与 record。[语法对照](01-languages.md) | 把第一件商品数量从 2 改成 3，先预测，再运行两边代码。 | 两边得到 `3900 cents`。金额为什么用整数表示？ |
| **Day 3** | 切片、List、map、Map、零值与缺失。[库存为零和查无商品](01-languages.md#库存为零和查无商品) | 用两个商品编号分别查售罄商品和不存在的商品，对照两种语言的表达。 | 能区分“存在且库存为 0”和“没有这个键”，说出各自的判断代码。 |
| **Day 4** | 接口、指针、空值和错误传递。[语法对照](01-languages.md) · [代码知识加练](code-foundations.md) | 读商品查询路径，再运行值与集合快照实验，找到共享和独立修改的位置。 | 用同一个查无商品的场景解释两种写法，并能画出实验里的对象与引用。 |
| **Day 5** | 根据规则完成函数。[库存查询练习](../exercises/stock-lookup/README.md) | 先跑失败用例，完成 Go 版，再按相同规则完成 Java 版。参考解放在第二次对照时看。 | 两边检查通过，能闭卷写出存在性判断。完成学习任务 `stock`。 |
| **Day 6** | HTTP 方法、路径、状态码和 JSON。[读取商品](02-http.md#先读懂浏览器里的-json) | 按章节启动两个内存服务，用 `curl -i` 分别查询商品 1 和 999。 | 两边分别得到 200 与 404。能说出端口、路径参数和响应正文各是什么。 |
| **Day 7** | 第一周复盘 | 合上源码，画出客户端到商品查询的调用链。换一个不存在的商品编号，先预测结果，再验证。 | 能独立启动两套服务，并解释零库存、缺失、业务错误和 HTTP 响应的区别。记录最需要重练的一项。 |

Day 6 启动的服务会一直等待请求，各占一个终端。保留它们，可以继续做下一阶段的接口练习。

### Day 1 的操作步骤

先在终端确认所在目录与工具版本。

```bash
pwd
ls
go version
java -version
javac -version
python3 --version
bash scripts/study-doctor.sh
```

`ls` 应能看到 `go`、`java`、`docs` 和 `labs`。`java` 用来运行程序，`javac` 用来编译 Java 源码，`go run` 会编译并运行 Go 程序。默认环境检查对应基础课，检查 Go、Java、JDK、Maven、Python 和 curl。Day 15 再加 `--docker` 检查容器工具和引擎。

接着分别运行两版购物车。

```bash
(cd go && go run ./cmd/basics)
java java/src/main/java/study/Basics.java
```

两边应出现下面四行。此时先看懂输出，再读 [Go main](../go/cmd/basics/main.go) 与 [Java main](../java/src/main/java/study/Basics.java)，沿调用走到 `total`、`subtotal`。

```text
total: 2700 cents
category: Stationery
category missing: Pen
invalid item: quantity must be positive
```

今天留下四行输出，以及自己的解释。“Notebook 单价 1200 分、数量 2，Pen 单价 300 分、数量 1，总额是 2700 分”。合上源码回答 `main` 为什么是程序入口，以及最后一行错误从哪产生。

### Day 2 的操作步骤

先在纸上计算 `1200 × 3 + 300 × 1`。然后在两个源码的 `main` 中，各找到构造订单明细的那一行，只把 Notebook 数量改成 3。

Go 对应的明细如下。

```go
// 单价以整数分表示，第三个字段是购买数量。
lines := []Priced{Line{"Notebook", 1200, 3}, Line{"Pen", 300, 1}}
```

Java 对应的明细如下。

```java
// 同一笔业务用相同单价和数量，方便核对两种实现的计算结果。
var lines = List.of(new Line("Notebook", 1200, 3), new Line("Pen", 300, 1));
```

再次执行 Day 1 的两条运行命令，第一行应变为 `total: 3900 cents`。用自己的话解释声明变量、创建明细、传入集合、遍历计算的过程。

再把 Notebook 的数量临时改成 0，运行并观察错误分支。Go 会检查返回的 `error`，Java 会抛出异常；这里 Java 的第一次 `total(lines)` 调用位于后面演示用的 `try` 之外，因此会打印未捕获异常的堆栈。比较错误从哪个方法产生、由谁处理。记录后把数量恢复到 2，再跑一次确认回到 `2700 cents`，便于以后重做。

### Day 3 的操作步骤

运行两个库存与集合示例。

```bash
(cd go && go run ./cmd/catalog)
java java/src/main/java/study/Catalog.java
```

先检查这三行，区分整数 0 和缺失。

```text
product 1: stock=5
product 2: stock=0
product 999: not found
```

再看后面的共享与复制实验，对照 [Go Catalog](../go/cmd/catalog/main.go) 和 [Java Catalog](../java/src/main/java/study/Catalog.java) 的源码注释。

| 输出 | 原因 |
| --- | --- |
| `original after shared update: 4` | 两个变量访问同一个 map，修改会反映到原表 |
| `original after copy update: 4` | 复制条目后修改的是新 map，原表保持 4 |
| `copy after copy update: 3` | 副本从 4 再减去 1 |

把两边的商品 2 从初始 map 中删除，重跑后对应行应变成 `product 2: not found`。解释 Go 的 `value, exists` 和 Java 的 `Integer` / `null` 判断，再恢复原始数据。末尾的字符串长度可以先记为观察项，重点完成 map 的存在性与复制实验。

### Day 4 的操作步骤

今天沿现有代码阅读接口和错误处理。按下表找三个位置，每个位置写一句自己的解释。

| 要定位的位置 | Go | Java |
| --- | --- | --- |
| 存储能力的约定 | [Store 接口](../go/order/store.go) 的 `Product`、`Create` 等方法 | [MemoryStore](../java/src/main/java/study/MemoryStore.java) 的商品查询与下单方法，观察本例的具体类结构 |
| 查无商品的业务分支 | `MemoryStore.Product` 返回 `ErrNotFound` | `MemoryStore.product` 抛出 `Rejected` |
| 转成 HTTP 响应的位置 | [HTTP 层](../go/order/http.go) 的 `writeResult` | [OrderServer](../java/src/main/java/study/OrderServer.java) 的 `handle` 捕获异常 |

Go 的实现按方法集满足 `Store`，本例 Java HTTP 服务直接使用具体的 `MemoryStore`。需要对照显式 `implements` 时，回到 Day 1 的 `Priced` 与 `Line`。

继续打开 [代码知识加练](code-foundations.md)，运行两版 `language-lab`。今天重点看 `valueAndCollectionDemo`，解释结构体复制、对象引用、浅拷贝与接口。错误与资源部分留到 Day 10 结合测试再练。

合上源码画出“查询商品 → 业务错误 → HTTP 状态”的三个位置。再解释 Go 的 `*MemoryStore`、`nil` 与多返回值，以及 Java 的对象引用和异常各承担什么作用。遇到不熟悉的语法，只追这一条调用链。

### Day 5 的操作步骤

先运行题目提供的检查，确认起点。

```bash
bash scripts/check-exercise.sh stock-lookup go
bash scripts/check-exercise.sh stock-lookup java
```

题目的 TODO 尚未补全时，Go 的有库存和售罄用例会失败，Java 会报告商品 1 的断言失败。打开 [Go 待填函数](../exercises/stock-lookup/go/stock.go) 和 [Java 待填方法](../exercises/stock-lookup/java/StockLookup.java)，按下表补齐，先做一边，再独立写另一边。

| 输入 | Go 预期 | Java 预期 |
| --- | --- | --- |
| 商品 1 | `5, true` | `OptionalInt.of(5)` |
| 商品 2 | `0, true` | `OptionalInt.of(0)` |
| 商品 999 | `0, false` | `OptionalInt.empty()` |

再次运行相同检查，直到 Go 通过、Java 输出“库存查询用例通过”。参考解可以在第一次自己尝试后展开，核对后再合上重写。完成后保存真实证据。

```bash
python3 scripts/study.py done stock
python3 scripts/study.py review --limit 2
```

### Day 6 的操作步骤

打开三个终端，都先进入仓库根目录。终端 A 启动 Go，终端 B 启动 Java，这两个进程会持续等待请求。

```bash
# 终端 A
(cd go && go run ./cmd/orders)
```

```bash
# 终端 B，首次运行会下载依赖
mvn -q -f java/pom.xml compile exec:java -Dexec.mainClass=study.OrderServer
```

在终端 C 执行四次查询。

```bash
curl --max-time 5 -i http://127.0.0.1:8080/products/1
curl --max-time 5 -i http://127.0.0.1:8081/products/1
curl --max-time 5 -i http://127.0.0.1:8080/products/999
curl --max-time 5 -i http://127.0.0.1:8081/products/999
```

前两条预期是 200，刚启动的内存服务返回商品 `Notebook`、库存 5。后两条预期是 404，正文包含 `not_found`。已经运行并下过单的内存进程以实际库存为准。`-i` 显示响应头，`--max-time 5` 限制这次查询最多等待 5 秒。

Day 3 的 Catalog 是独立的语法演示，其中录入了商品 2。这里的订单 API 初始只录入商品 1，读源码时分别核对各自的初始化数据。

今天记录“访问的端口、状态码、正文与处理方法”。Day 8 继续完成写入和校验后，再按 `http` 的整项验收保存完成。

### Day 7 的操作步骤

用 10 分钟闭卷完成下面的回访，再用代码和命令纠正。

- [ ] 从两种语言的 `main` 找到小计方法，解释单价与数量的类型。
- [ ] 算出数量为 3 时的金额，并指出数量为 0 的错误来源。
- [ ] 解释“库存为 0”和“没有商品”的区别。
- [ ] 解释 map 的共享赋值和条目复制。
- [ ] 指出商品查询的业务错误与 HTTP 响应转换位置。
- [ ] 独立启动两个内存服务，用 200 和 404 验证一次读取。

再做一次小故障实验。在这次启动 Go 的终端按 Ctrl+C，重跑终端 C 的 Go 查询，观察连接失败；Java 查询仍可返回。重新启动 Go 后再查一次，预期恢复 200。用自己的话说明“连接失败”和“服务返回 404”各发生在哪一步。

写下本周最容易混淆的一项，作为下一次开始前的闭卷问题。可用文末的记录模板保存 Day 1–4 与 Day 6–7 的结果，已有 `stock` 卡片继续按实际到期时间复习。

### 第一周常见排错

| 看到的现象 | 先检查什么 | 怎样继续 |
| --- | --- | --- |
| 找不到 `go`、`java`、`javac` 或 `mvn` | 对应的版本命令，Java 的运行环境与编译器是否都可用 | 补齐缺少的工具后重开终端，重新检查版本；JDK 与 Maven 的步骤见 [安装说明](../README.md#先跑一段语法对照) |
| 找不到 `java/...` 文件或 Go 提示目录不对 | `pwd` 与 `ls` 是否显示仓库根目录 | 回到含 `go`、`java`、`docs` 的目录，按原命令继续 |
| 容器阶段的 Docker Compose 或引擎检查失败 | 使用的阶段参数、Compose 版本和引擎是否启动 | 按 [容器启动说明](../README.md#启动进阶实验) 处理后重跑带 `--docker` 的检查 |
| `address already in use` 或 Java 绑定端口失败 | 对应端口是否已有这次练习的服务 | 先查询已有服务；需要重启时，在原启动终端按 Ctrl+C 后再启动 |
| `curl` 无法连接或等待超时 | 服务终端的启动输出、端口和进程是否仍在运行 | 用启动命令恢复对应服务，再查同一地址 |
| 商品查询得到 404 | 当前进程的初始化数据以及请求中的商品编号 | 用 `/products/1` 验证正常路径，再对照源码解释缺失编号 |
| 库存练习检查失败 | TODO 是否已补齐，以及失败信息中的实际值和预期值 | 每次修正一个分支，再运行同一条检查 |
| 金额与预期不同 | Day 2 改过的数量、单价和是否已恢复原始数据 | 按当前输入重新计算，保存对照后再重跑 |

## 第二阶段 · 接口正确性与两种调用边界

| 学习日 | 学什么、看哪里 | 当天动手 | 验收与闭卷问题 |
| --- | --- | --- | --- |
| **Day 8** | 请求头、请求体、参数校验与创建响应。[HTTP 下单](02-http.md) · [HTTP 契约测试](08-tests.md#自动验证-http-契约) | 在两种语言中分别提交一笔合法订单和一笔数量为 0 的请求。先预测三个边界用例，再读 handler 的 JSON 校验。 | 记录 201、400、`Location` 和库存变化。完成 `http`。 |
| **Day 9** | 幂等与同键冲突。[HTTP 重试](02-http.md) · [订单测试](08-tests.md) | 用原键原参数重放成功订单，再用相同键改数量，分别核对订单 ID 和库存。 | 重放返回原订单且不重复扣库存，改参得到 409。完成 `idempotency`。客户端超时后为何仍用原键？ |
| **Day 10** | 自动测试、错误原因和资源边界。[测试步骤](08-tests.md) · [代码知识加练](code-foundations.md) | 改错一个断言再恢复；跑两版 HTTP 契约测试，数量 -1 的变式可顺延。对照 `errorDemo` 与 `useResource` 的包装和关闭顺序。 | 能定位实际与预期，证明拒绝下单没有改变库存或订单，解释 Day 4 的复制实验及本次清理行为，完成 `code-foundations`。 |
| **Day 11** | goroutine、线程、锁与结果汇总。[并发扣库存](01-languages.md#并发扣库存) · [提交和汇总](concurrency-and-cancellation.md) | 把演示改为 1 件库存、20 次尝试，两边各重复运行三次。标出共享状态、临界区与取结果的位置。 | 每次只有 1 次成功且库存为 0。完成 `concurrency`，解释 Future.get 与 channel 汇总的等待顺序。 |
| **Day 12** | Electron Main、Renderer、Preload、IPC 与隔离。[桌面商品查询](../labs/electron-orders/README.md) | 用同一个窗口查两套后端，比较 200、404，再停止这次启动的 Java 服务并重查。 | Go 仍能独立返回，窗口显示 Java 连接故障。用同一个请求 ID 找两边日志，完成 `electron-ipc`。 |
| **Day 13** | Agent 的输入、工具请求、结果回填与回答。[短循环练习](agent-app-roadmap.md#现在就能做的三项练习) | 用 `trace` 分别输入 Redis 故障与“如何烤面包？”，逐条读事件。 | 能解释三步事件及无匹配时的空引用。达到任务验收后记录 `agent-loop`，以后继续做变式。 |
| **Day 14** | 第二周复盘 | 闭卷画出桌面 IPC、后端 HTTP 和 Agent 工具循环三条调用链。选一个薄弱点换输入重做。 | 能指出各处谁校验参数、谁决定业务结果、谁把结果展示给用户。 |

常用测试命令如下。测试自己创建数据，可以重复运行。

```bash
(cd go && go test -v ./order)
mvn -f java/pom.xml test
```

Day 13 的入门命令如下。当前由确定性模型适配器按规则产生决策，状态在内存中推进。

```bash
bash scripts/study-doctor.sh --profile agent
(cd labs/stack && go run ./cmd/agent-runtime trace ops-runbook-v1 'Redis 下线后商品查询怎么办？')
```

Day 12 启动 Electron 前，运行 `bash scripts/study-doctor.sh --profile electron`，核对当前终端的 Node.js 版本及两种后端的启动工具。工作台可单独用 `--profile web` 检查。

## 第三阶段 · PostgreSQL 与数据正确性

| 学习日 | 学什么、看哪里 | 当天动手 | 验收与闭卷问题 |
| --- | --- | --- | --- |
| **Day 15** | 镜像、容器、端口、网络、卷与健康检查。[独立练习栈](15-capstone.md#启动独立练习环境) | 检查 Docker，引擎可连接后启动练习栈，查看服务状态并查询商品。 | `18082/products/1` 可访问。解释哪些数据在卷里、重建容器后为什么还存在。 |
| **Day 16** | 表、主键、外键、唯一约束、CHECK 与参数绑定。[数据库基础](03-database.md) · [初始化 SQL](../labs/stack/init.sql) | 查询商品、订单与 outbox 的结构，在练习库中验证一个无效写入，并读 Go 的 `$1` 参数。 | 能指出各约束保护的规则，解释为什么参数要绑定。 |
| **Day 17** | 事务与提交边界。[Go 数据库存储](../labs/stack/store.go) | 创建一笔订单，用 SQL 核对库存、订单和 outbox，画出开始、三次写入与提交。 | 能把同一次下单的三种记录对应起来。中途失败应撤销哪些写入？ |
| **Day 18** | 条件更新、影响行数与业务回滚。[事务和并发](03-database.md) | 在练习库记录当前库存，尝试购买超过库存的数量，再查是否留下失败订单或多扣库存。 | 失败写入被回滚，能解释 `WHERE stock >= quantity` 和影响零行的含义。 |
| **Day 19** | READ COMMITTED、行锁和普通读取。[两个连接观察锁](10-sql-performance.md#用两个连接观察锁等待) | 开两个 psql 会话，按章节持锁、普通读、锁定读、超时和回滚。 | 能解释读到什么、在哪等待，释放锁后为何恢复。结合前两天证据完成 `database`。 |
| **Day 20** | B-tree、复合索引、执行计划、OFFSET 与游标。[查询计划实验](10-sql-performance.md) | 在练习栈运行临时表脚本，比较索引前后及两种分页的实际扫描行数。 | 记录自己的计划与 `same_page` 结果，解释为何有索引的深 OFFSET 仍可能多读数据。 |
| **Day 21** | 第三周复盘 | 闭卷画出两个请求竞争最后一件库存的时间线，再把其中一个请求改为原样重试。 | 分别说明锁、唯一约束、条件更新和事务的作用，并给每个结论配一条已有证据。 |

Day 15 启动命令如下。后续涉及数据库、Redis 和 RabbitMQ 的实验沿用此环境。

```bash
bash scripts/study-doctor.sh --docker
bash scripts/practice-stack.sh up --build --wait
bash scripts/practice-stack.sh ps
curl -fsS http://127.0.0.1:18082/products/1
```

Day 20 的脚本使用临时表，会话结束后清理实验表。

```bash
bash scripts/practice-stack.sh exec -T postgres psql -U study -d orders < labs/postgres/query-plans.sql
```

## 第四阶段 · Java 工程化与分页

| 学习日 | 学什么、看哪里 | 当天动手 | 验收与闭卷问题 |
| --- | --- | --- | --- |
| **Day 22** | Spring Boot、Controller、Service、依赖注入与 JdbcTemplate。[Java 模块](../labs/spring-orders/README.md) | 启动独立 MySQL / Spring Boot 实验，查询商品，沿 Controller 找到 Service 和 SQL。 | Java 商品查询成功，能与 Go handler 和 Store 对照职责。 |
| **Day 23** | Spring 代理、`@Transactional` 与 InnoDB 锁。[Java 事务源码](../labs/spring-orders/src/main/java/study/springorders/OrderService.java) | 重放同键请求并尝试同键改参，标出锁幂等键、扣库存、插订单和关联键的位置。 | 库存只扣一次，解释代理提交与 Go 显式提交的相同业务边界。 |
| **Day 24** | Java 事务回滚和跨实例幂等。[Java 隔离验收](../labs/spring-orders/README.md) | 运行模块的 `check.py`，对照强制写入失败与两个 API 实例处理同键请求的检查。 | 能解释库存、幂等键和订单的预期状态，记录证据并完成 `spring-mysql`。 |
| **Day 25** | 游标分页的边界与测试。[分页对照题](../exercises/order-history/README.md) | 先完成 Go 的待填函数，覆盖空结果、第一页、下一页和末页。 | Go 练习检查通过，能手算一组 ID 的分页结果。 |
| **Day 26** | 把同一分页规则写到 Java。[分页对照题](../exercises/order-history/README.md) | 合上 Go 实现，按相同输入完成 Java 版，再对照集合、比较与截取。 | 两边都通过，解释边界条件怎样保持一致。 |
| **Day 27** | 真实 API 的 `limit+1` 与 `nextCursor`。[翻两页订单](16-order-history-api.md) | 按章节准备订单并翻页，比较 Go、Java 和 PostgreSQL 的结果。 | 两页无重复、顺序正确、末页游标为空，完成 `pagination`。 |
| **Day 28** | 第四周复盘 | 闭卷画出 Spring 下单事务，再写一段不看参考解的分页伪代码。 | 能解释运行时异常的回滚、游标的取值，以及过滤与 LIMIT 的先后顺序。 |

## 第五阶段 · 缓存与消息链路

| 学习日 | 学什么、看哪里 | 当天动手 | 验收与闭卷问题 |
| --- | --- | --- | --- |
| **Day 29** | Cache-Aside、命中、回源与 TTL。[缓存读取](04-cache.md) · [源码](../labs/stack/store.go) | 在练习栈连续查询同一商品，等待过期再查询，用请求 ID 对照日志。 | 找到 MISS 和 HIT。解释键不存在、回填和过期各在哪一步发生。 |
| **Day 30** | 写后失效、缓存故障与旧值回填。[缓存一致性](11-cache-reliability.md) | 写订单后重查商品，再按章节停 Redis 验证 BYPASS，最后画出并发回填旧值的时间线。 | 区分缓存失效与故障回源，说明下单由数据库校验库存，完成 `cache`。 |
| **Day 31** | 热点、原子操作、Lua 与固定窗口限流。[Redis 限流](11-cache-reliability.md) | 按章节执行 Lua 限流四次，观察窗口过期后的下一次请求。 | 第四次被拒绝，过期后重新计数。为什么读取计数与递增需要原子执行？ |
| **Day 32** | exchange、queue、routing key 与异步处理。[消息基础](05-messaging.md) · [Go relay](../labs/stack/relay.go) | 提交订单，按订单 ID 查询 outbox，再看 relay 与 Java 消费日志。 | 用同一事件 ID 串起发布和消费，能画出消息经过的组件。 |
| **Day 33** | ACK、未确认消息、消费幂等与恢复。[Java 消费者](../java/src/main/java/study/NotificationWorker.java) · [暂停实验](05-messaging.md) | 暂停消费者后下单，观察积压，再恢复并查询通知表。 | 能解释写库前 ACK 与写库后 ACK 的崩溃窗口，以及 `event_id` 的去重作用。 |
| **Day 34** | 发布确认、mandatory 退回、死信与 Kafka 分区日志。[可靠投递](12-messaging-reliability.md) | 做无法路由的消息实验，查相应发布结果。再画出 Kafka 的分区、消费组和 offset 关系。 | 分清发布确认、成功路由和通知落库。Kafka 部分以概念图验收，结合前两天完成 `messaging`。 |
| **Day 35** | 第五周复盘 | 闭卷画出订单事务、outbox、发布、消费写库与 ACK，依次插入一次崩溃。 | 能预测哪里积压、哪里重投、哪里去重，指出要用哪份日志或 SQL 证明。 |

## 第六阶段 · 微服务、可靠性与诊断

| 学习日 | 学什么、看哪里 | 当天动手 | 验收与闭卷问题 |
| --- | --- | --- | --- |
| **Day 36** | 服务边界、同步调用、异步事件与局部故障。[分布式概念](06-distributed.md) · [故障边界](13-microservices-in-practice.md) | 画出本项目的进程、数据库与消息关系，逐个假设组件停止并预测影响。 | 能区分进程可用、依赖可用、订单已提交和通知已完成。 |
| **Day 37** | 多 API 实例与共享数据库。[两个实例处理同键请求](13-microservices-in-practice.md) | 在独立练习项目里按章节增加第二个 API 实例，用同一键分别访问两边。 | 两边返回同一订单，库存只扣一次。为什么两个进程各自的锁无法协调这个请求？ |
| **Day 38** | 等待、取消、总预算与有限重试。[双语言取消](concurrency-and-cancellation.md) · [真实 HTTP 超时](09-service-reliability.md) | 比较两种语言的成功、超时与失败，再用真实 HTTP 场景核对共享预算。 | 保存输出，解释等待结束与任务收尾、第二次请求的剩余预算，完成 `async-cancellation`。 |
| **Day 39** | 请求 ID、日志、`/livez`、`/readyz` 与依赖故障。[探针实验](17-observability-and-probes.md) | 按章节验证 Redis 与数据库故障，给一条请求设置自己的 ID 并查日志。 | 结合 Day 38 完成 `reliability`。能定位是连接、缓存、数据库还是进程问题。 |
| **Day 40** | Go 竞态检测、基准、CPU 与锁 profile。[性能排查](14-go-profiling.md) | 跑 `-race` 与串行 / 并行基准，查看 CPU 和 mutex profile。 | 记录自己的 `ns/op` 和热点，区分数据竞争与锁竞争，用证据解释一次性能现象。 |
| **Day 41** | 变更设计、接口兼容与回归范围。[商品筛选任务](15-capstone.md#任务二给订单历史加商品筛选) | 为 `GET /orders` 的商品筛选写出四类输入、预期结果与测试草图。圈出将修改的 Go / Java 入口。 | 明确正常、空结果、非法参数、跨页四种行为，说明筛选应放在哪里。 |
| **Day 42** | 第六周复盘 | 选择一条真实错误日志，写出排查顺序。再口述一次多实例下单、超时重试和缓存故障的边界。 | 每个判断能落到日志、状态码、SQL 或 profile；保留 Day 41 的变更草图供下周实现。 |

## 第七阶段 · 独立交付与 Agent 应用

| 学习日 | 学什么、看哪里 | 当天动手 | 验收与闭卷问题 |
| --- | --- | --- | --- |
| **Day 43** | Go 接口与内存实现的同步变更。[筛选约定](15-capstone.md#任务二给订单历史加商品筛选) | 先补测试，再给 HTTP 参数解析、Store 与内存查询增加筛选，保持原有调用可用。 | 原接口和新筛选的正常、空结果、非法参数用例通过。 |
| **Day 44** | SQL 筛选与分页顺序 | 把相同筛选接入 PostgreSQL，使用参数绑定，准备两种商品交错的订单验证跨页。 | 筛选在 LIMIT 前生效，跨页保持相同筛选条件，结果无重复和混入。 |
| **Day 45** | Java 行为对照与综合回归 | 把相同行为补到 Java 内存版，再按 [综合练习](15-capstone.md) 核对订单、outbox 和通知。 | 两种语言契约一致，原链路回归通过，完成 `project`。任务超过一天时，从未完成用例继续。 |
| **Day 46** | 重访 Agent loop、接口与工具分发。[Agent Engine](../labs/stack/agentruntime/engine.go) | 换一个订单系统问题，先预测 `Pending` 和 `Observations`，再运行轨迹核对。标出 `Model.Next`、参数校验和停止条件。 | 能从结果回填解释下一步决策，并把后端方法画成一个工具适配器的输入输出。更新 `agent-loop` 的变式证据。 |
| **Day 47** | 检索排序、上下文选择、引用与记忆边界。[检索对照](agent-app-roadmap.md#现在就能做的三项练习) | 对相同的跨组件问题比较 v1、v2，定位工具返回与答案引用的数量差异。 | 解释两条和三条证据的来源，区分会话历史、检查点与长期记忆，完成 `agent-context`。 |
| **Day 48** | 固定评估集、引用检查与工具权限。[评估实现](../labs/stack/agentruntime/eval.go) | 运行合格版与回归版评估，单独看 `tool_denied` 轨迹，设计一个引用存在但内容错误的答案。 | 能指出四项评估覆盖什么、还缺什么，完成 `agent-evaluation`。 |
| **Day 49** | 第七周复盘 | 从自己的筛选变更挑一条回归用例，再从 Agent 选一条失败轨迹，用五分钟分别解释。 | 能把业务正确性与回答质量各自落到可检查的证据，并说明已有工具怎样复用后端接口。 |

Day 43–45 是需要亲自实现的综合任务。涉及增加商品与下单的操作放在独立练习栈，按任务说明准备数据并记录当前库存。

## 第八阶段 · Agent Infra 与综合验收

| 学习日 | 学什么、看哪里 | 当天动手 | 验收与闭卷问题 |
| --- | --- | --- | --- |
| **Day 50** | API、后台 worker、持久化任务与事件。[Agent Runtime 启动](../labs/agent-runtime/README.md#运行和查看) | 启动 Agent 实验，提交问题并按任务 ID 查询状态、事件和最终引用。 | 指出接收、领取、工具执行和提交的进程边界，解释 HTTP 202 的含义。 |
| **Day 51** | 检查点、租约与代次校验。[隔离验收](../labs/agent-runtime/README.md#验收) · [worker](../labs/stack/agentruntime/worker.go) | 运行隔离验收，重点读 worker 崩溃后接手，以及旧 worker 恢复后提交被拒绝的证据。 | 画出旧代次、新代次与条件更新，解释工具结果返回但未提交时会发生什么。 |
| **Day 52** | 取消竞争、外部副作用与稳定幂等键。[取消说明](../labs/agent-runtime/README.md#取消运行中的任务) | 复盘取消与提交竞争，分别推演谁先成功。把只读工具替换成“扣款”场景，设计去重与结果查询。 | 解释为何本地检查点还需要外部业务幂等，以及取消后迟到结果在哪被拒绝。 |
| **Day 53** | 任务历史、重试来源和 Java 客户端。[任务历史](../labs/agent-runtime/README.md#翻看任务历史) | 用已有 Java 客户端读取 Go Agent 历史，检查两页 ID；复盘重试任务的版本与来源。 | 能区分原任务接手与创建新尝试，解释字符串 ID 的精度，结合前几天完成 `agent-runtime`。 |
| **Day 54** | 版本固定、评估门禁、发布审计与回滚。[发布与回滚](../labs/agent-runtime/README.md#发布与回滚) | 对照隔离验收里的回归拒绝、合格版切换与回滚，核对新旧任务保存的版本。 | 解释默认版本与任务绑定版本、预期版本检查和两条发布审计，完成 `agent-release`。 |
| **Day 55** | 综合故障复盘与下一阶段设计 | 选择一个订单故障和一个 Agent 故障，各写出输入、证据、恢复步骤与适用边界。再对照 [真实模型验收单](agent-app-roadmap.md#真实模型接入的验收单) 写下一项最小实现。 | 每个结论都有已有实验支撑；真实模型接入的工具关联、预算、错误处理和评估要求写清。 |
| **Day 56** | 全路线验收与薄弱点回访 | 闭卷画完整架构，从任意一个请求走到业务数据、消息与通知，再走到 Agent 工具和检查点。重做最薄弱的一项。 | 能在 10 分钟内解释系统并展示三份可复现证据，列出接下来最值得补的两个能力。 |

Agent 隔离验收命令如下。它会为本次检查创建独立环境，结束时清理本次资源。

```bash
python3 labs/agent-runtime/check.py
```

该脚本一次覆盖多个场景。Day 51、52、54 分别围绕恢复、取消和发布读取相关证据，结合源码解释机制，需要验证变式时再重跑。

## 学习任务与进度对应

每天都可以写自己的笔记。达到任务验收时，再保存到工作台或命令行，对应复习卡会进入队列。同一任务再次保存证据会保留已有卡片的日期与评分。

| 达到验收的学习日 | 学习任务 ID |
| --- | --- |
| Day 5 | `stock` |
| Day 8 | `http` |
| Day 9 | `idempotency` |
| Day 10 | `code-foundations` |
| Day 11 | `concurrency` |
| Day 12 | `electron-ipc` |
| Day 13，Day 46 回访 | `agent-loop` |
| Day 19 | `database` |
| Day 24 | `spring-mysql` |
| Day 27 | `pagination` |
| Day 30 | `cache` |
| Day 34 | `messaging` |
| Day 38 | `async-cancellation` |
| Day 39 | `reliability` |
| Day 45 | `project` |
| Day 47 | `agent-context` |
| Day 48 | `agent-evaluation` |
| Day 53 | `agent-runtime` |
| Day 54 | `agent-release` |

例如完成 HTTP 的验收后，执行下面的命令并填写你实际观察到的结果。

```bash
python3 scripts/study.py practice http
python3 scripts/study.py done http
python3 scripts/study.py review --limit 2
python3 scripts/study.py today
```

对尚未对应单独任务的学习日，把证据留在笔记里；到相关任务验收时一起整理。课程后续给已完成任务补充新卡片时，工作台和 `today` / `review` 会自动补入。

## 主动回忆与间隔复习

闭卷先说清三件事，再揭晓答案。

1. 今天的输入或故障是什么，预期会得到什么结果？
2. 哪段代码、哪个数据库约束或哪条消息规则决定了结果？
3. 如果换语言、增加实例，或把崩溃提前一步，结果会怎样？

系统按实际答题日期使用 `1、3、7、14、30` 天的间隔，答错后次日重练。连续每天学习且每次答对时，一张 Day 1 首次复习的卡片，会依次安排到 Day 2、Day 5、Day 12、Day 26、Day 56。每周复盘再选一个弱点做完整实验。

可复制下面的记录格式，每次只写几行。

```text
Day 与主题
先预测的结果
实际响应、日志、测试或 SQL 证据
能解释结果的代码位置与概念
仍然卡住的问题
下一次要改的输入或故障条件
```

## 第一轮结束后的延伸路线

当前可运行的 Agent 实验使用确定性模型、关键词检索与只读手册，持久化模式提供 PostgreSQL 检查点。真实模型、向量检索、MCP 服务和操作系统级工具隔离按下面的里程碑继续实现。

| 方向 | 下一项可交付任务 | 验收重点 |
| --- | --- | --- |
| 真实模型应用 | 为现有 `Model.Next` 增加一种模型适配器 | 模型提出工具请求，关联调用 ID，结果正确回填；超时、取消和无证据输入有可追溯结果 |
| RAG 与上下文工程 | 从现有手册扩展文档切块、元数据、召回与重排 | 同一问题比较检索结果与回答依据，保留资料版本并测试截断 |
| MCP 与工具接入 | 把已有只读查询封装为 MCP 工具 | 核对工具发现、输入校验、传输与用户权限，保留调用证据 |
| 长期记忆 | 定义跨会话信息的存储与更新流程 | 区分用户归属、读写权限、过期、删除与冲突处理 |
| Agent 执行隔离 | 为工具引入受限进程或沙箱 | 用受控样例验证文件、网络、凭证和资源限制，以及失败后的清理 |
| Go 深入 | 针对一个测得的瓶颈补调度、GC、连接池或 RPC 实验 | 改动前后保留性能证据，原业务规则通过回归 |
| Java 深入 | 在相同订单业务中补 JVM、线程、连接池与框架诊断 | 能从调用日志、线程或内存证据解释故障与恢复 |
| 分布式深入 | 选 Kafka、gRPC、Kubernetes 或 Saga 的一个小场景实现 | 明确数据归属、幂等、失败恢复、观测与回滚边界 |

选择方向时，对照 [后端岗位能力地图](backend-job-roadmap.md) 与 [Agent Infra 路线](agent-infra-roadmap.md)，先完成一个能重复验证的小交付，再扩大范围。
