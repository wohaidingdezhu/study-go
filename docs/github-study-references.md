# 从 GitHub 学习项目借鉴了什么

以 2026 年 11 月 16 日读取的 GitHub Star 数为快照。下面的仓库分别适合查语法、学测试、做双语练习和理解真实应用的测试结构。

| 项目 | Star | 值得借鉴的做法 | 本项目的改动 |
| --- | ---: | --- | --- |
| [Go by Example](https://github.com/mmcgrana/gobyexample) | 8,229 | 从带注释的短程序讲一个具体概念，方便运行和修改 | [练习入口](../exercises/README.md) 按库存和分页两个概念给出源文件、运行命令和预期 |
| [Learn Go with Tests](https://github.com/quii/learn-go-with-tests) | 23,964 | 用小测试练语法和设计，先看到失败，再逐步补实现 | 两题都带待完成函数和测试，先定位一条失败，再实现最小逻辑 |
| Exercism [Go](https://github.com/exercism/go) / [Java](https://github.com/exercism/java) | 1,197 / 839 | 不同语言沿用公共题目描述和标准用例 | 同一个订单规则可切换语言验证，显式比较 Go 返回值、Java 的 OptionalInt 和异常。 [公共题目规范](https://github.com/exercism/problem-specifications) |
| [Spring PetClinic](https://github.com/spring-projects/spring-petclinic) | 9,659 | 同一应用提供可运行的开发环境和数据库集成测试 | [综合练习](15-capstone.md) 增加跨服务检查，用订单 ID 和事件 ID 核对 API、PostgreSQL 和 Java 通知的结果 |

练习从一个能读完的函数起步，再接回订单 API。学习手册的第 2 次用库存题练 map，第 18 次先验证真实接口的分页，再为它扩展商品筛选。单函数通过只能说明那段规则成立，跨服务检查还可以观察数据库提交、消息发布和消费者写入。
