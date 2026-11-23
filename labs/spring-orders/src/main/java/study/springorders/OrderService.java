package study.springorders;

import java.sql.Statement;
import java.util.List;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.RowMapper;
import org.springframework.jdbc.support.GeneratedKeyHolder;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Isolation;
import org.springframework.transaction.annotation.Transactional;

// @Service 让 Spring 管理这个业务对象。Controller 通过注入获得它并调用订单方法。
@Service
public class OrderService {
    record Product(long id, String name, int stock) {}
    record Order(long id, long productId, int quantity) {}
    record Creation(Order order, boolean created) {}

    static final class Rejected extends RuntimeException {
        final int status;

        Rejected(int status, String code) {
            super(code);
            this.status = status;
        }
    }

    private static final RowMapper<Order> ORDER = (row, index) ->
            new Order(row.getLong("id"), row.getLong("product_id"), row.getInt("quantity"));
    private final JdbcTemplate jdbc;

    OrderService(JdbcTemplate jdbc) {
        // 构造器注入由框架提供 JdbcTemplate，类似 Go 启动时把连接池传入存储实现。
        // JdbcTemplate 负责准备 SQL、绑定参数、管理 JDBC 资源和映射数据库异常。
        this.jdbc = jdbc;
    }

    Product product(long id) {
        return jdbc.query("SELECT id, name, stock FROM products WHERE id = ?",
                (row, index) -> new Product(row.getLong("id"), row.getString("name"), row.getInt("stock")), id)
                .stream().findFirst().orElseThrow(() -> new Rejected(404, "not_found"));
    }

    Order order(long id) {
        return jdbc.query("SELECT id, product_id, quantity FROM orders WHERE id = ?", ORDER, id)
                .stream().findFirst().orElseThrow(() -> new Rejected(404, "not_found"));
    }

    List<Order> history(int rows, long before) {
        if (before == 0) {
            return jdbc.query("SELECT id, product_id, quantity FROM orders ORDER BY id DESC LIMIT ?", ORDER, rows);
        }
        // 主键索引从游标范围继续读，过滤条件在 LIMIT 之前生效。
        return jdbc.query("SELECT id, product_id, quantity FROM orders WHERE id < ? ORDER BY id DESC LIMIT ?",
                ORDER, before, rows);
    }

    boolean ready() {
        return Integer.valueOf(1).equals(jdbc.queryForObject("SELECT 1", Integer.class));
    }

    // 从 Controller 经 Spring 代理进入此方法时，@Transactional 为这些数据库操作建立事务。
    // 默认规则下，向外传播的 RuntimeException 会触发回滚，正常返回时由代理完成提交。
    // 与 Go store.go 中显式 BeginTx / Commit / Rollback 对照，关注相同的业务提交边界。
    @Transactional(isolation = Isolation.READ_COMMITTED)
    public Creation place(String key, long productId, int quantity) {
        // 先锁幂等键，再锁商品。唯一主键让相同键的请求在 InnoDB 中串行。
        // FOR UPDATE 读取并锁定该键的记录，锁保留到事务结束，让后来的同键请求等待结果。
        jdbc.update("INSERT INTO order_keys (request_key) VALUES (?) ON DUPLICATE KEY UPDATE request_key = request_key", key);
        Long previousId = jdbc.queryForObject("SELECT order_id FROM order_keys WHERE request_key = ? FOR UPDATE", Long.class, key);
        if (previousId != null) {
            Order previous = order(previousId);
            if (previous.productId() != productId || previous.quantity() != quantity) {
                throw new Rejected(409, "idempotency_conflict");
            }
            return new Creation(previous, false);
        }

        // 把库存检查和扣减合并在一个 UPDATE，多个 API 实例仍共享这条数据库约束。
        int changed = jdbc.update("UPDATE products SET stock = stock - ? WHERE id = ? AND stock >= ?",
                quantity, productId, quantity);
        if (changed == 0) {
            product(productId);
            throw new Rejected(409, "out_of_stock");
        }

        var keys = new GeneratedKeyHolder();
        // GeneratedKeyHolder 接收数据库生成的订单编号，可对照 PostgreSQL INSERT ... RETURNING id。
        jdbc.update(connection -> {
            var insert = connection.prepareStatement("INSERT INTO orders (product_id, quantity) VALUES (?, ?)",
                    Statement.RETURN_GENERATED_KEYS);
            insert.setLong(1, productId);
            insert.setInt(2, quantity);
            return insert;
        }, keys);
        long orderId = keys.getKey().longValue();
        jdbc.update("UPDATE order_keys SET order_id = ? WHERE request_key = ?", orderId, key);
        // 未捕获的 RuntimeException 会触发回滚，刚插入的键和扣减也一起撤销。
        return new Creation(new Order(orderId, productId, quantity), true);
    }
}
