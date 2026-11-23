# Electron × Go × Java 商品查询

先跑通 [HTTP 读取练习](../../docs/02-http.md#先读懂浏览器里的-json)。这次用一个桌面窗口查询同一个商品，沿途观察桌面 IPC 和两次后端 HTTP 请求。只查询商品，库存不会变化。

```text
商品输入框 → Renderer → Preload → IPC → Electron Main
                                         ├─ HTTP GET → Go   :8080
                                         └─ HTTP GET → Java :8081
```

## 启动

准备 Node.js 22.12+、npm，以及 [Go、JDK 和 Maven](../../README.md#检查学习环境)。以下命令都从仓库根目录执行。第一、第二条启动命令各占一个终端，服务已经在默认端口运行时直接复用。

先运行 `bash scripts/study-doctor.sh --profile electron`。若提示 Node.js 版本过低，切换到 22.12 或以上版本，重新检查通过后再安装 Electron 依赖。

```bash
# 终端 1 · Go
(cd go && go run ./cmd/orders)
```

```bash
# 终端 2 · Java
mvn -q -f java/pom.xml compile exec:java -Dexec.mainClass=study.OrderServer
```

```bash
# 终端 3 · Electron
npm --prefix labs/electron-orders ci
npm --prefix labs/electron-orders start
```

首次安装可能下载 Electron 运行时。要复用自定义端口，可在第三个终端设置 `STUDY_GO_PORT`、`STUDY_JAVA_PORT`。例如 macOS/Linux 上运行 `STUDY_GO_PORT=8082 STUDY_JAVA_PORT=18085 npm --prefix labs/electron-orders start`，适用于已经运行在这两个端口的 Go 进阶服务和 Java Spring Boot 服务。

## 先预测，再查询

| 操作 | 预期 | 核对 |
| --- | --- | --- |
| 查询 `1` | 两边都是 `HTTP 200`，商品名为 `Notebook` | 把窗口中的请求 ID 复制到三个终端，确认 Electron、Go、Java 记录了同一次查询。刚启动的内存服务库存为 `5`。 |
| 查询 `999` | 两边都是 `HTTP 404`，正文含 `not_found` | 两条后端日志都收到请求，比较返回正文。 |
| 按 Ctrl+C 停止本次启动的 Java 内存服务，再查 `1` | Go 仍是 `HTTP 200`，Java 显示未收到 HTTP 响应 | 比较 Go 日志和 Electron 单独记录的 Java 连接故障，再启动 Java 查询一次。 |
| 从桌面窗口打开开发者工具，运行 `window.orders.compareProduct('0')` | Main 拒绝非法商品 ID | 对照查询按钮的表单校验。直接调用桥接方法绕过表单时，Main 仍会验证参数。 |

开发者工具可用 macOS 的 `⌥⌘I` 或 Windows/Linux 的 `Ctrl+Shift+I`。如果两边均连接失败，先在终端分别用 `curl` 访问 `http://127.0.0.1:8080/products/1` 和 `http://127.0.0.1:8081/products/1`，核对服务端口。

## 顺着代码走一遍

1. [Renderer](renderer.js) 读取输入并调用 `window.orders.compareProduct(id)`。Renderer 负责显示画面，收到响应后更新两张结果卡。
2. [Preload](preload.cjs) 通过 `contextBridge` 提供一个查询方法，由 `ipcRenderer.invoke` 发起桌面进程间通信，再等待结果。
3. [Main](main.cjs) 校验发起请求的页面和商品 ID，再向两套后端并行发起 HTTP GET。它为这次点击生成一个请求 ID，带在两次请求的 `X-Request-ID` 头里。
4. [Go HTTP 处理](../../go/order/http.go) 和 [Java HTTP 处理](../../java/src/main/java/study/OrderServer.java) 根据同一份接口契约查商品，并记录请求。每条日志还有对应的服务、状态和耗时，可用来判断在哪一段失败。

| 边界 | 在这次练习中做什么 |
| --- | --- |
| 进程隔离 | Renderer 与 Main 运行在不同进程，通过 IPC 交换参数和结果。 |
| 上下文隔离 | 在 Renderer 内，把网页与 Preload 放入不同的 JavaScript 上下文，网页通过 `contextBridge` 访问公开方法。 |
| 渲染进程沙箱 | 约束 Renderer 能执行的系统操作。应用显式关闭网页的 Node 集成，页面不能直接 `require` Node 模块。 |

[Electron IPC](https://www.electronjs.org/docs/latest/tutorial/ipc) · [上下文隔离](https://www.electronjs.org/docs/latest/tutorial/context-isolation) · [IPC 来源校验](https://www.electronjs.org/docs/latest/tutorial/security#17-validate-the-sender-of-all-ipc-messages)

## 留下学习记录

关掉源码，用自己的话画出执行顺序。给每段连线写上 IPC 或 HTTP，说出从哪里查状态码，以及在哪一步检查用户输入。再换成一个你没查过的商品 ID，先预测两边的结果。

```bash
python3 scripts/study.py practice electron-ipc
python3 scripts/study.py done electron-ipc
python3 scripts/study.py review
```

把三组结果和一条能在两个后端查到的请求 ID 写进练习记录。完成后，工作台和命令行的复习队列会加入三道题。
