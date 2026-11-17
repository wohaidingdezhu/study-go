# Java / Go 对照练习

每题有一个待补全的函数、两种语言共用的业务规则和可以立即运行的检查。从项目根目录执行命令，先看哪项失败，再编辑题目指定的源文件。两边都通过后，翻开题目末尾的参考解，比较数据类型、错误处理和写法。

| 练习 | 接在什么时候 | 做完能解释什么 |
| --- | --- | --- |
| [库存查询](stock-lookup/README.md) · 10–15 分钟 | [第 01 章](../docs/01-languages.md#库存为零和查无商品) | Go map 的存在标记和 Java 的 OptionalInt 怎样表示已售罄 |
| [订单历史分页](order-history/README.md) · 25–35 分钟 | [第 08 章](../docs/08-tests.md) 和 [第 10 章](../docs/10-sql-performance.md) | 如何用上一页末尾的 ID 查询下一页，并给出边界测试 |

```bash
bash scripts/check-exercise.sh stock-lookup go
bash scripts/check-exercise.sh stock-lookup java
```

看到失败是练习起点。先改一个实现，重新跑同一条命令，直到通过。Go 检查用 `go test`，Java 检查用 JDK 编译并启用断言。两道题可以离线完成。
