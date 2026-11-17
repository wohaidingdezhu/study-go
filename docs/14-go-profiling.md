# 14 · 用 Go 基准测试定位锁竞争

订单 API 的内存版本用一把锁保护库存和幂等表。本章用同一笔订单的重试做基准测试，看看串行和并行访问时的开销，再用 profile 查时间花在哪里。

## 先观察基准结果

从项目根目录执行。使用 Go 标准库即可。

```bash
(cd go && go test ./order -run '^$' -bench '^BenchmarkMemoryStoreReplay$' \
  -benchtime=400ms -benchmem -cpu=1,4)
```

打开 [基准测试](../go/order/store_bench_test.go)。每个操作都调用 `MemoryStore.Create` 重放已经创建的订单。两组共享同样的业务规则，`parallel` 还用 `RunParallel` 让多个 goroutine 同时操作。 `-cpu=1,4` 分别以两个 `GOMAXPROCS` 值运行。

| 输出 | 怎么读 |
| --- | --- |
| `ns/op` | 墙钟时间除以操作数。并行时可以用它比较总体吞吐，单笔请求的 p95 需要另测 |
| `B/op`、`allocs/op` | 每个操作平均分配的堆字节和次数 |
| `serial`、`parallel` | 相同逻辑在串行和并行访问下的对比。结果与机器、并发量和临界区有关 |

## 收集 CPU 和 mutex profile

```bash
profiles="$(mktemp -d)"
(
  cd go
  go test ./order -run '^$' -bench '^BenchmarkMemoryStoreReplay$/^parallel$' \
    -cpu=4 -benchtime=2s -o "$profiles/order.test" \
    -cpuprofile "$profiles/cpu.pprof" -mutexprofile "$profiles/mutex.pprof"
)
go tool pprof -top -nodecount=8 "$profiles/order.test" "$profiles/cpu.pprof"
go tool pprof -top -cum -nodecount=12 "$profiles/order.test" "$profiles/mutex.pprof"
```

CPU profile 是采样，先看 `flat` 了解函数自身占用，再看 `cum` 了解包含下游调用的累计占用。mutex profile 用来找争用造成的等待，记录可能归到解锁堆栈。一次本机采样中，按 `cum` 排序能从 `MemoryStore.Create` 找到主要等待归属，再对照 `sync.(*Mutex).Unlock` 确定是锁竞争。实际热点名称会受编译器内联和运行环境影响。 [Go 性能诊断](https://go.dev/doc/diagnostics) · [go test 参数](https://pkg.go.dev/cmd/go#hdr-Testing_flags) · [Go mutex profile](https://pkg.go.dev/runtime/pprof)

## 把 Go 运行时概念串起来

- **Goroutine 与调度**。多个 goroutine 可以等 I/O 或锁。 `GOMAXPROCS` 控制可同时执行 Go 代码的线程数上限，提高它后，共享锁仍可能是瓶颈。
- **堆分配与 GC**。不再引用的堆对象由 GC 回收。看分配速率和堆 profile，再决定是否减少临时对象。有分配增长时，先用数据找热点。
- **竞态检测**。基准回答执行成本。 `-race` 能记录参与执行的代码是否出现未同步的并发读写。两类工具对应不同问题。 [Go 数据竞态检测](https://go.dev/doc/articles/race_detector)

```bash
(cd go && go test -race ./order)
```

### 练习

比较 `parallel` 在 1 和 4 下的输出，写下「吞吐怎样变化」「哪个 profile 支持你的解释」。如果想把一部分临界区缩短，先指出哪些操作必须一起受锁保护，再跑第 08 章的测试。这组基准测量内存存储的重放路径，HTTP、PostgreSQL 和 Redis 的耗时可以用第 09、10 章的方法分别测量。
