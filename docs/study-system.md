# Go × Java 后端学习系统

在同一套订单系统里练接口实现、业务规则验证和故障定位。每次先预测结果，再执行一个小改动，用测试或观测反馈纠正。做完以后关掉源码，用自己的话说清原因，并在几天后从记忆重做。

## React 工作台

静态工作台将练习记录和复习日期保存在当前浏览器。「Agent 实验」里的构建回放可逐步比较不同版本的工具请求、检索结果和门禁判断。实时提交 Agent 任务和共享命令行 SQLite 进度使用本机模式。 [GitHub Pages](https://wohaidingdezhu.github.io/study-go/) · [构建回放说明](../labs/agent-runtime/README.md#逐步查看构建回放)

从项目根目录运行，需要 Node.js 18+、npm 和 Python 3.9+。首次下载依赖后，页面可以在本机离线运行，打开 [学习工作台](http://127.0.0.1:4173)。

```bash
npm --prefix web ci
npm --prefix web run build
npm --prefix web start
```

- **今日学习**展示下一项练习和到期卡片。项目路线按阶段展示练习，材料和代码能在网页中打开。
- **练习记录**与命令行共用 `.study/progress.sqlite3`。浏览器保存后，`python3 scripts/study.py today` 能看到同一份进度。
- **间隔复习**先显示题目，点「揭晓要点」后再自行评分。评分和下一次复习日期一起保存。
- **Agent 实验**连接本机 `18084` 的 Go 服务。可以切换试跑版本、取消和重试任务，从历史里筛选、翻页并查看工具证据。启用方式见 [Agent 启动说明](../labs/agent-runtime/README.md#运行和查看)。

改 React 源码后重新运行构建并刷新浏览器。若端口冲突，改用 `npm --prefix web start -- --port 4174`，页面地址也随之改为 4174。若 Agent 连接失败，可以先查看容器状态，再从页面重试连接。日常的任务和复习在网页服务启动后即可使用。

## 每次怎么练

1. **5–10 分钟闭卷复习。** 先说出答案，再按回车对照。说错或漏关键点选 `0`，按提示完成一次纠错。
2. **15–25 分钟练一个弱点。** 先写预期，运行一条检查，然后改一处。做同一业务的第二种语言时，从接口契约出发重新写，最后再对照实现。
3. **5 分钟留下证据。** 保存 HTTP 状态码、测试结果或 SQL 行数，再写下为什么。下一次可以改输入或故障条件，检验能否迁移。

只有 15 分钟时，复习两三张，做一个最小实验。第一次练习看注释定位关键步骤，第二次闭卷只留题目和预期。复习答对后，从本次实际答题日顺延 `1、3、7、14、30` 天，答错则次日再来并重置间隔。这组天数是便于执行的默认值。主动提取和分散复习的研究支持这种练法，具体间隔需要根据保留目标和掌握情况调整。 [主动回忆实验](https://pubmed.ncbi.nlm.nih.gov/16507066/) · [间隔效应实验](https://doi.org/10.1111/j.1467-9280.2008.02209.x) · [刻意练习与反馈](https://doi.org/10.1098/rsos.190327)

## 第一次从商品接口开始

从仓库根目录执行。Python 3.9 或以上可运行学习工具，进度保存在本机 `.study/progress.sqlite3`，默认不会进入 Git。首次练习使用 Go 和 Java 的内存服务，无需 Docker。

```bash
bash scripts/study-doctor.sh
python3 scripts/study.py today
```

按 [README 的启动命令](../README.md#对比两个-http-服务) 在两个终端启动服务。第三个终端分别查存在和不存在的商品，先写下你预测的状态码。

```bash
curl -i http://127.0.0.1:8080/products/1
curl -i http://127.0.0.1:8081/products/1
curl -i http://127.0.0.1:8080/products/999
curl -i http://127.0.0.1:8081/products/999
```

按 [HTTP 项目步骤](02-http.md#亲手创建订单并重试) 完成正常下单和数量为 `0` 的请求。记下两种语言的结果、正常下单的 `Location`，以及从 HTTP 层如何找到存储方法。完成后用以下命令写入真实结果并开始首次闭卷复习。

```bash
python3 scripts/study.py done http
python3 scripts/study.py review
python3 scripts/study.py today
```

`practice` 可查看全部任务，`practice pagination` 可查看一项。`progress` 可以回看已经记录的证据。`review --limit 2` 最多做两张，退出时已评分的卡会保存。已经掌握的任务可以直接做验收并记录证据，系统会把对应卡片加入队列。

## 主线和实战验收

| 项目阶段 | 优先练什么 | 通过条件 |
| --- | --- | --- |
| 双语言订单 API | HTTP 契约、集合和缺失值、错误处理和测试 | 能凭同一组用例在 Go 和 Java 解释或补齐输入校验、幂等和库存检查 |
| 订单正确性 | 并发临界区、PostgreSQL 事务、约束和游标分页 | 能复现并发竞争，证明不超卖、事务失败会回滚、分页无重复 |
| 一笔订单跨服务 | Redis 回源、Go outbox 发布、Java 消费去重、超时和探针 | 能暂停 Redis 或 Java 消费者，依据 HTTP、SQL 和日志解释恢复过程 |
| 独立交付一个变更 | 为订单列表加入商品筛选，并迁移到两种语言 | 能闭卷写出接口和测试，处理非法值、空页、混合数据和跨页，原链路通过回归 |

进到数据库、缓存或消息队列时，先启动 [独立练习栈](15-capstone.md#启动独立练习环境)。涉及下单、暂停服务和故障注入的命令都在这个栈执行。读章节示例时使用端口 `18082`，把对应的容器命令改用 `bash scripts/practice-stack.sh`。两道待填练习在修改前预期会有测试失败，这是反馈的起点。 [双语言练习](../exercises/README.md)

遇到写不出来时，把题缩小到一个行为。比如出现重复扣库存，先写同键重放的测试，只观察订单 ID 和库存差值。再次练习时改成同键改参，检查错误码和库存都符合约定。综合任务之后，学习工具会推荐 Agent 任务恢复和版本发布两项进阶任务。想从进阶任务开始，可用 `practice agent-runtime` 或 `practice agent-release` 查看验收条件。 [AI Agent Infra 岗位路线](agent-infra-roadmap.md) · [扩展练习](study-workbook.md)
