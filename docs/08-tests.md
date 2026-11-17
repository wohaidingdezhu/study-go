# 08 · 给订单规则写自动测试

第 02 章靠你发请求、看响应和库存。现在把两条业务规则写成测试，让程序每次都按同一方式检查。两个测试都直接调用内存存储，跑完这节约需 15 分钟。

## 先跑一遍

从项目根目录执行。Go 使用标准库，Java 需要 JDK 21+ 和 Maven，首次会下载 JUnit。 [Go 测试教程](https://go.dev/doc/tutorial/add-a-test) · [JUnit 5 用户指南](https://docs.junit.org/5.13.4/user-guide/)

```bash
(cd go && go test -v ./order)
mvn -f java/pom.xml test
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

Go 测试放在 `order_test` 包里，只使用对外公开的存储方法。Java 测试放在 `study` 包里，可以直接访问同包的 `MemoryStore`。这些测试验证存储的业务规则。HTTP 状态码、JSON 结构和请求头可以在服务启动后用已有的 [接口练习脚本](../scripts/check-orders.py) 验证。若想检查通知是否落库，再做 [第 07 章](07-trace-an-order.md) 的跨服务查询。

## 先观察失败，再恢复通过

1. 在第一条测试中，把「重试后库存」的预期从 `0` 暂时改成 `1`。Go 中把条件里的 `product.Stock != 0` 和对应报错文案一起改成 1。Java 中把对应的 `assertEquals(0, …)` 改成 1。
2. 只重跑这一条，找出输出里的实际值和预期值。

   ```bash
   (cd go && go test -v ./order -run '^TestCreateReplaysAtZeroStock$')
   mvn -f java/pom.xml '-Dtest=MemoryStoreTest#replaysAtZeroStock' test
   ```

3. 把预期恢复成 0，重跑同一条。回到通过后，试着新增「同一个成功的幂等键，修改购买数量后返回冲突」的用例。先写出预期的错误和最终库存，再打开内存存储的实现核对。
