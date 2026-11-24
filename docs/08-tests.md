# 08 · 给订单规则写自动测试

第 02 章靠你发请求、看响应和库存。先用两个存储测试理解“准备 → 执行 → 断言”，再用同一组 Go / Java HTTP 测试检查请求解析、响应头与业务状态。存储部分约需 15 分钟，HTTP 部分可以在下一次练习继续。

## 先跑一遍

从项目根目录执行。Go 使用标准库，Java 需要 JDK 21+ 和 Maven，首次会下载 JUnit。 [Go 测试教程](https://go.dev/doc/tutorial/add-a-test) · [JUnit 5 用户指南](https://docs.junit.org/5.13.4/user-guide/)

```bash
(cd go && go test -v ./order -run '^TestCreate')
mvn -f java/pom.xml -Dtest=MemoryStoreTest test
```

Go 会显示两个 `PASS`。Maven 的统计会显示 `Tests run: 2, Failures: 0, Errors: 0, Skipped: 0`。两边测试都自己创建库存为 5 的内存存储，可以反复运行。

## 用同一组例子读两种测试

| 场景 | 执行 | 要验证的结果 |
| --- | --- | --- |
| 库存归零后重试成功订单 | 先买 5 件，再用相同的键和数量提交一次 | 第二次返回原订单，`created` 为 `false`，库存保持 0 |
| 库存不足后调整数量 | 先尝试买 6 件，再用原键买 1 件 | 第一次返回库存不足，库存仍为 5。第二次新建成功，库存为 4 |

打开 [Go 的测试](../go/order/store_test.go) 和 [Java 的测试](../java/src/test/java/study/MemoryStoreTest.java)，按「准备数据 → 执行操作 → 检查结果」读。同一段测试会同时检查返回值和状态变化。例如只检查重试返回同一个订单 ID，会漏掉「库存被多扣了一次」的错误。

| 要做什么 | Go | Java |
| --- | --- | --- |
| 让工具发现测试 | `_test.go` 文件中写 `func Test…(t *testing.T)` | 在 `src/test/java` 中写带 `@Test` 的方法 |
| 比较实际与预期 | 用 `if` 判断，再用 `t.Fatalf` 输出实际值与预期 | `assertEquals(预期值, 实际值)` |
| 检查预期失败 | 调用拿到 `err`，用 `errors.Is` 判断错误类型 | 把调用放进 `() -> …`，交给 `assertThrows` 捕获异常 |

Go 测试放在 `order_test` 包里，只使用对外公开的存储方法。Java 测试放在 `study` 包里，可以直接访问同包的 `MemoryStore`。这些测试验证存储的业务规则。下半节继续自动检查 HTTP 层；已经启动的服务也可用 [接口练习脚本](../scripts/check-orders.py) 验证。若想检查通知是否落库，再做 [第 07 章](07-trace-an-order.md) 的跨服务查询。

## 先观察失败，再恢复通过

1. 在第一条测试中，把「重试后库存」的预期从 `0` 暂时改成 `1`。Go 中把条件里的 `product.Stock != 0` 和对应报错文案一起改成 1。Java 中把对应的 `assertEquals(0, …)` 改成 1。
2. 只重跑这一条，找出输出里的实际值和预期值。

   ```bash
   (cd go && go test -v ./order -run '^TestCreateReplaysAtZeroStock$')
   mvn -f java/pom.xml '-Dtest=MemoryStoreTest#replaysAtZeroStock' test
   ```

3. 把预期恢复成 0，重跑同一条。回到通过后，试着新增「同一个成功的幂等键，修改购买数量后返回冲突」的用例。先写出预期的错误和最终库存，再打开内存存储的实现核对。

## 自动验证 HTTP 契约

契约是调用方和服务方对可观察行为的约定，包括请求格式、状态码、响应数据、响应头，以及操作后的业务状态。存储测试通过，还需要验证 HTTP 层是否把这些规则正确暴露出去。

从仓库根目录执行，测试自行准备数据，无需手动启动 8080、8081 或 Docker。

```bash
(cd go && go test -v ./order -run '^TestHTTP')
mvn -f java/pom.xml -Dtest=OrderHttpTest test
```

Go 显示四个顶层 HTTP 测试通过。Java 的 `OrderHttpTest` 显示 `Tests run: 34, Failures: 0, Errors: 0, Skipped: 0`：18 次拒绝下单测试、一次创建与重试链路、14 次非法分页测试及一次分页链路。本节先读下单测试，分页部分留到 [第 16 章](16-order-history-api.md#自动验证分页边界) 对照。

打开 [Go HTTP 测试](../go/order/http_test.go) 和 [Java HTTP 测试](../java/src/test/java/study/OrderHttpTest.java)，先找到输入表，再找发送请求和断言的位置。两边使用相同的场景名称，方便逐行对照。

| 边界 | 用例中的输入 | 预期状态与错误码 |
| --- | --- | --- |
| 请求头 | 缺少幂等键、键中含空格 | `400 invalid_idempotency_key` |
| 媒体类型 | `text/plain` | `415 expected_json` |
| JSON 格式与根类型 | 不完整 JSON、空正文、`null`、数组 | `400 invalid_order` |
| 字段 | 缺少数量、额外传入 `price` | `400 invalid_order` |
| 数值 | 数量 0 或 101、字符串 `"1"`、小数、商品 ID 超出有符号 64 位整数范围 | `400 invalid_order` |
| 输入边界 | 一个 JSON 后又跟一个 JSON、超过本项目 4096 字节上限 | `400 invalid_order` |
| 业务规则 | 商品 999、购买数量 6 而库存为 5 | 分别为 `404 not_found`、`409 out_of_stock` |

这些状态与严格解析规则是本项目的约定。例如本项目把超长订单正文映射为 `400 invalid_order`，其他服务可以明确约定使用 `413`。开发客户端时应核对具体接口契约。

每条拒绝用例还会查询商品与订单列表，确认库存保持 5、订单数为 0，且拒绝响应没有 `Location`。随后用合法输入和键 `contract` 再下单，预期 `201`；原请求使用合法键的用例由此证明失败没有占用这个键。

成功用例依次验证：首次下单 `201` → 按 `Location` 查回订单 → 原键原参数重放 `200` → 原键改数量 `409 idempotency_conflict` → 库存为 4、订单只有一笔。每次响应都检查 JSON 媒体类型和回传的请求 ID。比较 JSON 解析后的字段，避免把字段排列顺序当成业务差异。

## 两种语言怎样组织隔离测试

| 动作 | Go | Java |
| --- | --- | --- |
| 每次准备干净的数据 | 每个子测试新建 `MemoryStore` 和 Handler | `@BeforeEach` 新建 `OrderServer`，内部存储独立 |
| 复用真实入口 | `HTTPServer(...).Handler` | 注册真实的 `OrderServer.handle` |
| 发请求 | `httptest.NewRequest` 加 `ServeHTTP`，在进程内直接调用 Handler | `HttpClient` 请求绑定到 `127.0.0.1:0` 的临时服务 |
| 取得响应 | `ResponseRecorder` 记录状态、头和正文 | `HttpResponse<String>` 保存响应 |
| 同一规则运行多次 | 结构体切片、`t.Run` 子测试 | `Stream<Arguments>`、`@MethodSource` 参数化测试 |
| 清理资源 | 当前写法没有监听端口 | `@AfterEach` 关闭客户端、服务和执行器，断言失败也清理 |

Java 的端口 `0` 由系统分配实际可用端口，测试从 `getAddress().getPort()` 读取。它会使用本机回环连接。Go 当前写法直接测试 Handler，覆盖路由、中间件和内存存储，不验证 TCP 监听、TLS 或真实网络超时。两边均未覆盖 PostgreSQL、消息投递与多实例故障，相关验证继续沿后续章节进行。 [Go httptest](https://pkg.go.dev/net/http/httptest) · [JDK HttpServer](https://docs.oracle.com/en/java/javase/21/docs/api/jdk.httpserver/com/sun/net/httpserver/HttpServer.html) · [JUnit 参数化测试与生命周期](https://docs.junit.org/5.13.4/user-guide/)

Go 的 `responseJSON[T any]` 是泛型辅助函数，调用时用 `order.Product` 或订单列表结构指定要解析的类型；`t.Helper()` 让失败位置指向调用辅助函数的测试代码。Java 用 `JsonNode` 读取 JSON 字段，`assertEquals(预期, 实际)` 失败时展示两者差异。先看业务断言，再回看辅助函数如何减去重复的发送和解析代码。

## 刻意练习与闭卷回忆

Day 8 先预测表中三种请求会得到什么；Day 10 跑完整组用例，并完成一次“失败 → 定位 → 恢复”的反馈循环。

1. 在 Go 的 `zero_quantity` 行或 Java 的同名行中，暂时把预期状态 `400` 改成 `201`。只改测试的预期。
2. 分别用下面的命令定位同一种失败，比较实际状态、预期状态与失败行号。

   ```bash
   (cd go && go test -v ./order -run '^TestHTTPRejectsInvalidOrders/zero_quantity$')
   mvn -f java/pom.xml '-Dtest=OrderHttpTest#rejectsInvalidOrders' test
   ```

   Go 命令只选择这个子用例，Java 命令运行整个参数化方法，其中 `zero_quantity` 失败。

3. 恢复预期 `400`，再给两版表格增加数量为 `-1` 的行。合上实现，写出预期错误码、库存、订单数，以及改回合法数量后原键是否还能使用，再运行验证。
4. 用下列命令检查原有存储规则和新增 HTTP 契约。GitHub 自动部署也会执行两种语言的测试，检查通过后再发布。

   ```bash
   (cd go && go test -race ./...)
   mvn -f java/pom.xml test
   ```

闭卷回答：只断言 `400` 会漏掉什么？为什么字符串 `"1"` 和数字 `1` 是不同输入？测试是否验证了监听端口？相同请求返回同一个 ID，是否已经能证明库存只扣一次？复习卡仍沿用 `http` 与 `idempotency` 任务，无需重置已有进度。
