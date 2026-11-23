# Java × Go 后端学习项目

从计算订单金额开始，逐步做出订单 API，再让 Go 订单服务和 Java 通知服务通过 RabbitMQ 协作。每章都有可以运行的例子、预期结果和动手练习。

## 今天练什么

在线打开 [学习工作台](https://wohaidingdezhu.github.io/study-go/)，可以阅读材料、记录练习、做间隔复习。在线进度保存在当前浏览器。

可以在 React 工作台查看任务、记录结果、做闭卷复习，并观察 Agent 的执行事件。从项目根目录运行以下命令，再打开 [本机工作台](http://127.0.0.1:4173)。网页与命令行共享本机进度。 [网页启动与排错](docs/study-system.md#react-工作台)

```bash
npm --prefix web ci
npm --prefix web run build
npm --prefix web start
```

也可以在终端查看下一项任务。初次从商品接口开始，Go 和 Java 使用相同的请求和验收标准。 [学习系统使用说明](docs/study-system.md)

```bash
python3 scripts/study.py today
```

## 项目与课程

[Go × Java 后端招聘能力地图](docs/backend-job-roadmap.md)将公开岗位样本映射到可验证的练习。Java 可用 [Spring Boot + MySQL](labs/spring-orders/README.md) 实现与 Go 相同的订单契约。

AI Agent 可以沿同一个订单项目一起学习。完成 HTTP 对照后，先走 [Go × Java × AI Agent 合并路线](docs/agent-app-roadmap.md)，用自选问题观察工具调用、检索、结果回填和评估。继续用 [Go Agent Runtime](labs/agent-runtime/README.md) 练任务恢复、发布门禁、版本回滚和事件审计，岗位进阶见 [AI Agent Infra](docs/agent-infra-roadmap.md)。

第一次学后端，可以先完成第 01、02 章。先对照两边的代码，再发一次 HTTP 请求，顺着代码找到处理它的地方。到第 03 章再用 Docker，跟着一笔订单理解每个组件的作用。已经看到了浏览器里的商品 JSON，可以直接做[第 02 章的读取练习](docs/02-http.md#先读懂浏览器里的-json)。

跑通两套商品接口后，接着做 [Electron × Go × Java 商品查询](labs/electron-orders/README.md)。从同一次点击追踪桌面 IPC 和两次后端 HTTP 请求，按请求 ID 对照两边日志。

准备开始时，打开 [18 次学习任务和自测](docs/study-workbook.md)。要了解每一阶段对应哪些岗位能力，可以看 [Go 后端学习路线](docs/go-roadmap.md)。路线把公开岗位要求映射到五个学习阶段，每个阶段都有能亲手验证的目标。刚完成第 02 章，可以先跑第 08、09 章，再深入数据和分布式实验。

想直接动手时，打开 [Java / Go 对照练习](exercises/README.md)，先运行测试，再补函数。可以在 [GitHub 项目对照](docs/github-study-references.md) 查看练习设计参考。

| 顺序 | 学习内容 | 可以做什么 |
| --- | --- | --- |
| 01 | [Java / Go 语法对照](docs/01-languages.md) | 对照接口、错误处理、集合复制、字符串长度和并发 |
| 02 | [HTTP 后端服务](docs/02-http.md) | 跑起同接口的两套实现，比较路由、校验、状态码和幂等 |
| 03 | [PostgreSQL](docs/03-database.md) | 看表、约束、索引和事务，验证并发下单不会超卖 |
| 04 | [Redis 缓存](docs/04-cache.md) | 观察命中、未命中、过期、写后失效和 Redis 停机回源 |
| 05 | [RabbitMQ 消息队列](docs/05-messaging.md) | 暂停消费者、重放消息，观察确认、重投、去重和死信 |
| 06 | [微服务与分布式](docs/06-distributed.md) | 实验局部故障、重试与最终一致性，理解何时需要 Saga |
| 07 | [跟踪一笔订单](docs/07-trace-an-order.md) | 从 HTTP 响应查到订单、outbox 事件和 Java 消费结果 |
| 08 | [给订单规则写自动测试](docs/08-tests.md) | 用 Java 和 Go 检查重试、错误结果和库存变化，可在第 02 章后练 |
| 09 | [Go 服务的超时、取消与重试](docs/09-service-reliability.md) | 模拟下游 503 和超时，观察同一请求预算 |
| 10 | [数据库查询计划、分页与锁](docs/10-sql-performance.md) | 用 5 万行验证索引、游标分页和锁等待，对照 MySQL |
| 11 | [缓存一致性、热点与 Redis 原子限流](docs/11-cache-reliability.md) | 推演并发回填旧值，跑 Lua 限流 |
| 12 | [消息投递语义、积压与 Kafka](docs/12-messaging-reliability.md) | 查不可路由的消息和积压，比较队列与分区日志 |
| 13 | [微服务边界、多实例与分布式故障](docs/13-microservices-in-practice.md) | 让两台 API 处理同一幂等键并推演支付补偿 |
| 14 | [Go 性能排查](docs/14-go-profiling.md) | 比较并发基准和锁竞争，实跑 pprof |
| 15 | [综合练习](docs/15-capstone.md) | 在独立环境验证通知恢复，再给订单历史加商品筛选 |
| 16 | [在真实 API 里翻两页订单](docs/16-order-history-api.md) | 比对 Go 内存、Java 内存和 PostgreSQL 的游标分页 |
| 17 | [用请求 ID 和探针排查服务](docs/17-observability-and-probes.md) | 按 ID 查日志，实测缓存和数据库故障 |
| 18 | [Spring Boot × MySQL 订单实战](labs/spring-orders/README.md) | 用同一组用例验证 Java 事务、并发幂等、回滚和探针 |

## 检查学习环境

从项目根目录执行 `bash scripts/study-doctor.sh`，查看 Go、JDK、Maven 和其他工具。进阶实验加上 `--docker`，同时确认 Docker 引擎可连接。

## 先跑一段语法对照

先在终端进入项目根目录，也就是能看到这份 README 的目录。用 `ls` 确认能看到 `go`、`java` 和 `docs`。

```bash
go version
java -version
```

语法练习需要 Go 1.22+ 和 JDK 21+。第 02 章启动 Java HTTP 服务时还会用到 Maven，它会在第一次运行时下载项目依赖。

如果 macOS 提示找不到 Java，可以用下面的命令安装 JDK 和 Maven。两条 `export` 让当前终端使用这份 JDK，另开终端时可再执行一次。 [Homebrew JDK 21](https://formulae.brew.sh/formula/openjdk@21) · [Homebrew Maven](https://formulae.brew.sh/formula/maven)

```bash
brew install openjdk@21 maven
export JAVA_HOME="$(brew --prefix openjdk@21)/libexec/openjdk.jdk/Contents/Home"
export PATH="$JAVA_HOME/bin:$PATH"
java -version
mvn -version
```

回到项目根目录，运行两版购物车。

```bash
(cd go && go run ./cmd/basics)
java java/src/main/java/study/Basics.java
```

两边都会算出 `2700 cents`，然后演示查不到分类和商品数量不合法时各怎么写。

打开 [Go 的 basics](go/cmd/basics/main.go) 和 [Java 的 Basics](java/src/main/java/study/Basics.java)，按 `main → total → subtotal` 的顺序看。试着把第一件商品的数量从 2 改成 3，再运行，预期总额变成 `3900 cents`。

## 对比两个 HTTP 服务

分别在两个终端启动。看到终端打印地址，就可以用另一个终端访问。服务会一直等新请求，按 Ctrl+C 可以结束它。

```bash
(cd go && go run ./cmd/orders)
```

```bash
mvn -q -f java/pom.xml compile exec:java -Dexec.mainClass=study.OrderServer
```

在第三个终端用 `curl` 向两个服务查询商品。首次应看到 JSON 中的 `"stock":5`。可以先跟着 [第 02 章](docs/02-http.md) 手动发一笔订单，再重启服务跑并发练习。并发练习需要 Python 3，会把两边库存都用完，每次从库存为 5 的新进程开始。

```bash
curl http://127.0.0.1:8080/products/1
curl http://127.0.0.1:8081/products/1
python3 scripts/check-orders.py http://127.0.0.1:8080
python3 scripts/check-orders.py http://127.0.0.1:8081
```

## 启动进阶实验

Docker 引擎负责运行容器，Compose 根据一份 YAML 一次启动整组服务。Docker 镜像可以理解为启动容器所需的程序与运行环境，容器是镜像启动后运行的实例。Go 和 Java 进阶程序都在容器里编译。

```bash
docker version
docker compose version
```

看到 `docker version` 同时列出 Client 和 Server，说明命令行已连上引擎。使用 Colima 的 macOS 上，如果引擎停了，运行 `colima start` 再检查。 [Colima 文档](https://github.com/abiosoft/colima#usage)

```bash
docker compose -f labs/stack/compose.yaml up --build --wait
docker compose -f labs/stack/compose.yaml ps
curl --retry 10 --retry-connrefused --retry-delay 1 http://127.0.0.1:8082/products/1
```

`-f` 指定配置文件，`--build` 先构建本项目镜像，`--wait` 等服务启动并通过已配置的健康检查。Go API 的检查访问 `/readyz`。第一次还会下载镜像和依赖。首次启动时查询应返回 `{"id":1,"name":"Notebook","stock":5}`。 [Docker Compose up](https://docs.docker.com/reference/cli/docker/compose/up/)

进阶服务复用 Go 的 HTTP 层，把存储换成 PostgreSQL，并加入 Redis 和 RabbitMQ。Java 通知服务消费事件并写入独立的通知数据库。第 03–07、10–13、15–17 章都可以用这套栈练习。要从一套独立数据开始，用 [第 15 章](docs/15-capstone.md#启动独立练习环境) 的命令。

遇到请求失败时，可以先运行上面的 `ps` 看服务状态，再用下面的命令看最近 30 行日志。

```bash
docker compose -f labs/stack/compose.yaml logs --tail=30 api relay notifications
```

学完这次练习可以停掉容器。

```bash
docker compose -f labs/stack/compose.yaml down
```

停止后保留 PostgreSQL 和 RabbitMQ 的实验数据。需要重做整套实验时，在结束当前练习后执行 `docker compose -f labs/stack/compose.yaml down -v`，清空这两个实验卷并重新启动。

| 代码入口 | 对照重点 |
| --- | --- |
| [Go 语法与并发](go/cmd)、[Java 语法](java/src/main/java/study/Basics.java)、[Java 并发](java/src/main/java/study/Concurrency.java) | 同一任务的两种写法 |
| [Go HTTP 和内存存储](go/order)、[Java HTTP](java/src/main/java/study/OrderServer.java)、[Java 内存存储](java/src/main/java/study/MemoryStore.java) | 请求到达后怎么变成一次业务操作 |
| [Go 进阶服务](labs/stack)、[Java 通知服务](java/src/main/java/study/NotificationWorker.java) | 持久化、缓存、发布和消费 |
