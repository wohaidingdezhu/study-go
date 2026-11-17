\set ON_ERROR_STOP on
\echo 基准数据：5 万笔临时订单，50 个客户，每人 1000 笔

-- 临时表只在当前连接中可见。会话关闭后 PostgreSQL 会清理它。
CREATE TEMP TABLE lesson_orders (
    id bigint PRIMARY KEY,
    customer_id integer NOT NULL,
    quantity integer NOT NULL
);

INSERT INTO lesson_orders (id, customer_id, quantity)
SELECT id, 1 + id % 50, 1 + id % 4
FROM generate_series(1, 50000) AS series(id);
-- 收集统计信息，并让后续的索引仅扫描有机会跳过数据页。
VACUUM (ANALYZE) lesson_orders;

\echo 1. 只有主键索引
EXPLAIN (ANALYZE, BUFFERS)
SELECT id, quantity FROM lesson_orders
WHERE customer_id = 7 ORDER BY id DESC LIMIT 20;

-- 先定位客户，再在同一客户的索引范围内按订单编号读取。
-- INCLUDE 保存查询所需的 quantity 字段，它不参与排序。
CREATE INDEX lesson_orders_customer_id_idx
ON lesson_orders (customer_id, id DESC) INCLUDE (quantity);
ANALYZE lesson_orders;

\echo 2. 增加复合索引后运行相同查询
EXPLAIN (ANALYZE, BUFFERS)
SELECT id, quantity FROM lesson_orders
WHERE customer_id = 7 ORDER BY id DESC LIMIT 20;

\echo 3. OFFSET 分页，需要越过前 800 行
EXPLAIN (ANALYZE, BUFFERS)
SELECT id, quantity FROM lesson_orders
WHERE customer_id = 7 ORDER BY id DESC LIMIT 20 OFFSET 800;

\echo 4. 游标分页，10006 是上一页最后一条的订单编号
EXPLAIN (ANALYZE, BUFFERS)
SELECT id, quantity FROM lesson_orders
WHERE customer_id = 7 AND id < 10006 ORDER BY id DESC LIMIT 20;

\echo 5. 比较两种分页的编号和顺序
WITH offset_page AS (
    SELECT id FROM lesson_orders WHERE customer_id = 7
    ORDER BY id DESC LIMIT 20 OFFSET 800
), cursor_page AS (
    SELECT id FROM lesson_orders WHERE customer_id = 7 AND id < 10006
    ORDER BY id DESC LIMIT 20
)
SELECT (SELECT array_agg(id ORDER BY id DESC) FROM offset_page)
       = (SELECT array_agg(id ORDER BY id DESC) FROM cursor_page) AS same_page;

DROP TABLE lesson_orders;
