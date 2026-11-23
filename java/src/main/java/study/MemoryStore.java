package study;

import java.util.Comparator;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

// 入门版本把状态放在单个 Java 进程里，退出后两个 Map 和库存都会重置。
final class MemoryStore {
    // record 自动提供构造器、字段访问器和按值比较，适合表示接口中的数据。
    // 本例各字段都是数值或 String，创建后通过新 record 表达更新，可对照 Go 的结构体值。
    record Product(long id, String name, int stock) {}
    record Request(long productId, int quantity) {}
    record Order(long id, long productId, int quantity) {}
    record Creation(Order order, boolean created) {}

    static final class Rejected extends RuntimeException {
        final int status;
        final String code;

        Rejected(int status, String code) {
            super(code);
            this.status = status;
            this.code = code;
        }
    }

    private Product product = new Product(1, "Notebook", 5);
    // 同一笔订单有两条查找路径，订单编号用于 GET，幂等键用于 POST 重试。
    private final Map<Long, Order> orders = new HashMap<>();
    private final Map<String, Order> keys = new HashMap<>();
    private long nextId = 1;

    // synchronized 实例方法持有 this 的监视器锁，与其他锁定同一实例的方法互斥。
    // HashMap 的并发读写由此保护，可对照 Go MemoryStore 的 Mutex。
    synchronized Product product(long id) {
        if (product.id() != id) {
            throw new Rejected(404, "not_found");
        }
        return product;
    }

    synchronized Order order(long id) {
        var order = orders.get(id);
        if (order == null) {
            throw new Rejected(404, "not_found");
        }
        return order;
    }

    List<Order> listOrders(int maxRows, long before) {
        List<Order> snapshot;
        synchronized (this) {
            snapshot = List.copyOf(orders.values());
        }
        // 快照里的 record 可安全读取。筛选和排序不必持有下单使用的锁。
        // stream 把筛选、排序和截取组合成处理流程，最后由 toList 生成结果列表。
        return snapshot.stream()
                .filter(order -> before == 0 || order.id() < before)
                .sorted(Comparator.comparingLong(Order::id).reversed())
                .limit(maxRows)
                .toList();
    }

    synchronized Creation create(String key, Request request) {
        // 此锁协调当前 Java 实例的请求。多实例部署时，需要共享数据库约束来保护同一业务。
        // 先处理重试。用同一个键且参数一样就返回原订单，参数变了则拒绝。
        var existing = keys.get(key);
        if (existing != null) {
            if (existing.productId() != request.productId() || existing.quantity() != request.quantity()) {
                throw new Rejected(409, "idempotency_conflict");
            }
            return new Creation(existing, false);
        }
        if (product.id() != request.productId()) {
            throw new Rejected(404, "not_found");
        }
        if (product.stock() < request.quantity()) {
            throw new Rejected(409, "out_of_stock");
        }

        // 查询、扣库存、保存订单与键都在同一个 synchronized 方法中。
        // Product 是 record，所以更新库存时用一个新的 Product 替换旧值。
        var order = new Order(nextId++, request.productId(), request.quantity());
        product = new Product(product.id(), product.name(), product.stock() - request.quantity());
        orders.put(order.id(), order);
        keys.put(key, order);
        return new Creation(order, true);
    }
}
