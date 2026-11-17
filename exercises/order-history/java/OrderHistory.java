import java.util.List;

final class OrderHistory {
    record Page(List<Long> items, Long nextCursor) {}

    // before 为 0 表示第一页。
    static Page pageOrders(List<Long> orderIds, int limit, long before) {
        // TODO: 校验参数，筛选并排序，然后计算游标。
        return new Page(List.of(), null);
    }
}
