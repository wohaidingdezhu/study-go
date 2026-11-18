# Spring Boot × MySQL 订单实战

用同一套订单接口对照 Java Spring MVC、Go `net/http`、MySQL InnoDB 和 PostgreSQL。这个独立模块使用 JDK 21 与 Spring Boot 3.5。连接配置由 Docker Compose 注入。 [Spring Boot 运行要求](https://docs.spring.io/spring-boot/3.5/system-requirements.html)

## 启动和下单

从仓库根目录执行。练习端口默认是 `18085`，独立的数据卷会在首次启动时初始化商品 1，库存 5。

```bash
docker compose -f labs/spring-orders/compose.yaml up -d --build --wait --wait-timeout 120
curl -fsS http://127.0.0.1:18085/readyz
curl -fsS http://127.0.0.1:18085/products/1
```

用相同的键执行以下代码两次，再把 `quantity` 改成 `1` 执行一次。用全新的幂等键可创建另一个订单。

```bash
curl -i http://127.0.0.1:18085/orders \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: spring-lesson-1' \
  -H 'X-Request-ID: spring-lesson-request' \
  -d '{"productId":1,"quantity":2}'
```

在新库首次执行时预期得到 `201` 和 `Location`，原样重试返回 `200` 和相同订单。复用键改参数返回 `409 idempotency_conflict`。库存只扣一次。可用以下命令核对访问日志。

```bash
docker compose -f labs/spring-orders/compose.yaml logs --tail 40 api
curl -fsS 'http://127.0.0.1:18085/orders?limit=2'
```

## 对照代码与事务

| 环节 | Java / MySQL | Go / PostgreSQL |
| --- | --- | --- |
| HTTP → 业务 | `@RestController` 声明路由，构造方法注入 Service，异常处理器映射 JSON。 [OrderController.java](src/main/java/study/springorders/OrderController.java) | `net/http` 注册 handler 并解码 JSON。 [http.go](../../go/order/http.go) |
| 重试与库存 | InnoDB 先锁幂等键，再执行带库存条件的 `UPDATE`。Spring `@Transactional` 包住扣库存、插订单和关联键。 [OrderService.java](src/main/java/study/springorders/OrderService.java) | PostgreSQL 用唯一键和 `INSERT ... ON CONFLICT` 协调并发，Go 显式开始和提交事务。 [store.go](../stack/store.go) |
| 查询和就绪 | `JdbcTemplate` 绑定 `?` 参数。`/readyz` 从连接池取连接执行 `SELECT 1`。 | pgx 参数写成 `$1` 等。就绪通过 `Ping` 检查数据库。 [数据库和探针练习](../../docs/17-observability-and-probes.md) |

`@Transactional` 由 Spring 代理在调用公开方法时开启事务。订单写入抛出运行时异常时，已执行的库存更新和临时幂等键一起回滚。`READ COMMITTED` 是这次练习显式使用的隔离级别。写业务前先约定统一的加锁顺序。 [Spring 事务代理](https://docs.spring.io/spring-framework/reference/6.2/data-access/transaction/declarative/annotations.html) · [InnoDB 锁](https://dev.mysql.com/doc/refman/8.4/en/innodb-locking.html)

## 自动验收

```bash
python3 labs/spring-orders/check.py
```

- 先在隔离库临时制造订单写入失败，观察 HTTP `500`，库存仍为 5，订单和幂等键行数都为 0。
- 由两个独立 Spring Boot 实例共用同一个 MySQL。共用验收脚本交替向两个 API 发送 8 个同键请求，只有一个创建，另外 7 个返回原订单。之后用 8 个不同键竞争最后 3 件商品，恰好 3 笔成功，两个实例看到的订单与库存一致。
- 检查游标的两页、空结果和非法参数，再停止隔离库中的 MySQL。两个实例的 `/livez` 都报 `200`，`/readyz` 都报 `503`。

排序、存储和故障的测试使用本次临时库，测试后会清理临时资源。手工练习结束后，可以通过 `docker compose -f labs/spring-orders/compose.yaml down` 停止服务并保留手工数据。
