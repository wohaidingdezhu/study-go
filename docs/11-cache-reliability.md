# 11 · 缓存一致性、热点与 Redis 原子限流

第 04 章已经能看到命中和回源。这章追问两个问题：什么时候会读到旧数据，流量太大时拿什么保护后面的数据库。

## 缓存一致性的失效窗口

按下面的顺序在纸上推演 Cache-Aside。

1. A 缓存未命中，从数据库读到库存 5，暂时还没回填缓存。
2. B 成功下单，提交库存 4，然后删除缓存。
3. A 恢复执行，把先前读到的 5 写进缓存并设置 TTL。
4. 新 GET 可能暂时显示 5。一次新的下单仍以数据库条件更新为准。

TTL 限制这次旧值在缓存里可存多久。是否愿意看到这段旧结果取决于读接口的业务要求。更严格的读可直接读主库，或者让写入的数据带版本并在并发读写时避免旧值覆盖。设计时列出写成功、删缓存失败、回填延迟这三种情况，说明每种情况靠什么收敛。 [Redis Cache-Aside](https://redis.io/docs/latest/develop/use-cases/cache-aside/)

| 现象 | 先看什么 | 可实验的应对措施 |
| --- | --- | --- |
| 穿透：请求大量不存在的商品 | 404 和数据库缺失查询是否一起上升 | 短暂缓存「未找到」，同时限制无效查询流量 |
| 击穿：同一个热门键过期 | 某个键过期前后，回源请求是否成批出现 | 在进程内合并同键回源，多实例时再考虑跨实例协调 |
| 雪崩：许多键同时失效或 Redis 不可用 | 回源率、数据库连接等待和超时是否一起上升 | TTL 加抖动，限制回源并发，优先保护数据库 |
| 淘汰：缓存达到内存上限 | `evicted_keys` 是否增加 | 根据工作负载选择策略和内存预算 |

TTL 是时间上的失效，淘汰是内存压力下的释放。两种都可能表现为 MISS。仅看总体命中率也容易看不见热门键的问题。 [Redis 键淘汰](https://redis.io/docs/latest/develop/reference/eviction/)

## 数据结构和持久化如何影响设计

本项目用 String 存一份商品 JSON，用 `INCR` 维护计数。局部字段常改时可以考虑 Hash，排行榜可从 Sorted Set 的分值和范围查询入手。先确定访问方式，再选数据结构、是否设置 TTL 和大致内存预算。

Redis 的 RDB 在某个时间点保存快照，AOF 记录写操作。重启后可能恢复到哪个时间点由快照间隔和刷盘策略决定。商品缓存丢失可以回源重建，计数或业务数据丢失的影响需要单独评估。若用 Redis 做短期互斥，可以用带有效期和随机令牌的 `SET NX`，释放时原子校验令牌。仍需考虑持有者暂停到租约过期后继续写入的情况，关键库存由数据库约束保护。 [Redis 持久化](https://redis.io/docs/latest/operate/oss_and_stack/management/persistence/) · [Redis 分布式锁](https://redis.io/docs/latest/develop/clients/patterns/distributed-locks/)

## 实验：把计数和设置过期放在一起

同一个调用方在窗口内允许 3 次请求。可以用 Redis `INCR` 维护计数。如果把首次设置过期放在另一条命令，进程在中间退出时会留下无过期的计数键。[Lua 脚本](../labs/redis/fixed-window.lua) 把两步放在同一次 Redis 执行中。在项目根目录运行。

```bash
key="rate:study:$(date +%s)"
for request in 1 2 3 4; do
  printf '请求 %s\n' "$request"
  docker compose -f labs/stack/compose.yaml exec -T redis sh -c \
    'redis-cli --raw EVAL "$(cat)" 1 "$1" 3 60000' sh "$key" \
    < labs/redis/fixed-window.lua
done
docker compose -f labs/stack/compose.yaml exec -T redis redis-cli PTTL "$key"
```

每次返回三行，依次是是否放行（1 或 0）、窗口内计数、剩余毫秒数。前 3 次放行，第 4 次返回 0，TTL 不会因后续请求被重置。等过期后用同一个键再执行一条命令，计数应重新从 1 开始。想快速验证过期，可以把窗口改成 3000ms，并用一个新键。

Redis Lua 执行期间，同一个实例中的其他命令不能插入两步之间。脚本应做少量工作，并把参数校验放在写入前。固定窗口在两个窗口交界处可能允许短时突发。滑动窗口用近一段时间内的请求衡量速率，令牌桶允许消耗先前积累的令牌来吸收一部分突发。 [Redis Lua 脚本](https://redis.io/docs/latest/develop/programmability/eval-intro/)

### 练习

设计商品查询的限流键，比较按用户、按 IP 和按商品分别能控制谁的请求。再决定 Redis 不可用时是否仍允许读取商品，以及怎样限制回源以保护数据库。把「商品展示」和「下单」两个接口分开作答。
