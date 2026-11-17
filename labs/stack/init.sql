-- CHECK 是数据库约束。即使另一段程序直接写表，库存也不能低于零。
CREATE TABLE products (
    id bigint PRIMARY KEY,
    name text NOT NULL,
    stock integer NOT NULL CHECK (stock >= 0)
);

-- UNIQUE 保证每个成功的幂等键只有一笔订单。REFERENCES 保证订单关联的商品存在。
CREATE TABLE orders (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    idempotency_key text NOT NULL UNIQUE,
    product_id bigint NOT NULL REFERENCES products(id),
    quantity integer NOT NULL CHECK (quantity BETWEEN 1 AND 100)
);

-- outbox 保存已经随订单提交、等待发给 RabbitMQ 的事件。published_at 为空表示待发。
CREATE TABLE outbox (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    order_id bigint NOT NULL UNIQUE REFERENCES orders(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz
);

-- 部分索引只包含待发事件，配合 relay 的筛选和按 id 排序。
CREATE INDEX outbox_pending ON outbox (id) WHERE published_at IS NULL;

INSERT INTO products (id, name, stock) VALUES (1, 'Notebook', 5);

-- 通知服务拥有自己的逻辑数据库，通过消息接收订单信息。
CREATE DATABASE notifications;
\connect notifications

-- event_id 作为主键，在重复投递时拦住重复写入。
CREATE TABLE order_notifications (
    event_id bigint PRIMARY KEY,
    order_id bigint NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now()
);
