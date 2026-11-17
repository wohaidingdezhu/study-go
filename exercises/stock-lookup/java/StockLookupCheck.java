import java.util.Map;
import java.util.OptionalInt;

final class StockLookupCheck {
    public static void main(String[] args) {
        var stockById = Map.of(1, 5, 2, 0);
        var available = StockLookup.lookupStock(stockById, 1);
        assert available.equals(OptionalInt.of(5)) : "商品 1 应返回库存 5，实际为 " + available;

        // OptionalInt.of(0) 仍有值。空 OptionalInt 才表示找不到商品。
        var soldOut = StockLookup.lookupStock(stockById, 2);
        assert soldOut.equals(OptionalInt.of(0)) : "商品 2 存在，库存为 0，实际为 " + soldOut;

        var missing = StockLookup.lookupStock(stockById, 999);
        assert missing.isEmpty() : "商品 999 应返回空值，实际为 " + missing;
        System.out.println("库存查询用例通过");
    }
}
