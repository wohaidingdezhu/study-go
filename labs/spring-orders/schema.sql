CREATE TABLE products (
    id BIGINT PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    stock INT NOT NULL CHECK (stock >= 0)
) ENGINE=InnoDB;

CREATE TABLE orders (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    product_id BIGINT NOT NULL,
    quantity INT NOT NULL CHECK (quantity BETWEEN 1 AND 100),
    CONSTRAINT orders_product FOREIGN KEY (product_id) REFERENCES products(id)
) ENGINE=InnoDB;

-- 大小写不同的 HTTP 幂等键要保持不同。InnoDB 的主键也提供并发时可锁定的记录。
CREATE TABLE order_keys (
    request_key VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin PRIMARY KEY,
    order_id BIGINT UNIQUE,
    CONSTRAINT order_keys_order FOREIGN KEY (order_id) REFERENCES orders(id)
) ENGINE=InnoDB;

INSERT INTO products (id, name, stock) VALUES (1, 'Notebook', 5);
