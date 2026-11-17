import java.util.ArrayList;
import java.util.List;
import java.util.Objects;

final class OrderHistoryCheck {
    public static void main(String[] args) {
        // 构造可变列表，检查分页方法是否改动了调用方持有的数据。
        var ids = new ArrayList<>(List.of(3L, 5L, 1L, 4L, 2L));
        var first = OrderHistory.pageOrders(ids, 2, 0);
        expectPage(first, List.of(5L, 4L), 4L);
        var second = OrderHistory.pageOrders(ids, 2, first.nextCursor());
        expectPage(second, List.of(3L, 2L), 2L);
        expectPage(OrderHistory.pageOrders(ids, 2, second.nextCursor()), List.of(1L), null);
        assert ids.equals(List.of(3L, 5L, 1L, 4L, 2L)) : "分页不应改变输入顺序";

        expectPage(OrderHistory.pageOrders(List.of(), 2, 0), List.of(), null);
        expectPage(OrderHistory.pageOrders(List.of(1L, 2L), 2, 0), List.of(2L, 1L), null);
        expectsInvalid(() -> OrderHistory.pageOrders(List.of(), 0, 0));
        expectsInvalid(() -> OrderHistory.pageOrders(List.of(), 101, 0));
        expectsInvalid(() -> OrderHistory.pageOrders(List.of(), 2, -1));
        System.out.println("订单分页用例通过");
    }

    private static void expectPage(OrderHistory.Page page, List<Long> items, Long cursor) {
        assert items.equals(page.items()) : "预期订单 " + items + "，实际为 " + page.items();
        assert Objects.equals(cursor, page.nextCursor())
                : "预期游标 " + cursor + "，实际为 " + page.nextCursor();
    }

    private static void expectsInvalid(Runnable call) {
        try {
            call.run();
        } catch (IllegalArgumentException expected) {
            return;
        }
        throw new AssertionError("无效分页参数应抛出 IllegalArgumentException");
    }
}
