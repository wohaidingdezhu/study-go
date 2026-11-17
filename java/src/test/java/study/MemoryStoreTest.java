package study;

import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

class MemoryStoreTest {
    @Test
    void replaysAtZeroStock() {
        // 每个测试创建自己的存储，从库存 5 开始。
        var store = new MemoryStore();
        var request = new MemoryStore.Request(1, 5);

        var first = store.create("buy-all", request);
        assertTrue(first.created(), "第一次提交应创建订单");

        // 库存已经用完。重试应返回原订单，且库存保持为 0。
        var replayed = store.create("buy-all", request);
        assertFalse(replayed.created(), "重试无需创建新订单");
        assertEquals(first.order(), replayed.order(), "重试应返回同一笔订单");
        assertEquals(0, store.product(1).stock(), "重试后库存");
    }

    @Test
    void canRetryAfterOutOfStock() {
        var store = new MemoryStore();

        // 把要执行的代码交给 assertThrows，它会捕获预期异常，接着可以检查错误码。
        var rejected = assertThrows(MemoryStore.Rejected.class,
                () -> store.create("try-again", new MemoryStore.Request(1, 6)));
        assertEquals("out_of_stock", rejected.code);
        assertEquals(5, store.product(1).stock(), "拒绝后库存");

        var retried = store.create("try-again", new MemoryStore.Request(1, 1));
        assertTrue(retried.created(), "失败后可用同一键提交合法数量");
        assertEquals(1, retried.order().quantity());
        assertEquals(4, store.product(1).stock(), "成功后库存");
    }
}
