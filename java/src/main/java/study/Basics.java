package study;

import java.util.List;
import java.util.Map;

public class Basics {
    // 接口约定任何可计价的明细都提供 subtotal()。Java 要显式声明 implements。
    interface Priced {
        int subtotal();
    }

    // record 自动提供构造器和同名访问方法，例如 quantity()。可对照 Go 的 Line struct。
    // 金额用整数分表示，1200 表示 12 元。
    record Line(String name, int unitCents, int quantity) implements Priced {
        @Override
        public int subtotal() {
            if (quantity <= 0) {
                // throw 把失败交给调用方处理，方法的正常返回值仍然只有一个 int。
                throw new IllegalArgumentException("quantity must be positive");
            }
            return unitCents * quantity;
        }
    }

    // ? extends Priced 表示元素是某种实现了 Priced 的类型，所以 List<Line> 也能传进来。
    static int total(List<? extends Priced> items) {
        int sum = 0;
        for (Priced item : items) {
            sum += item.subtotal();
        }
        return sum;
    }

    public static void main(String[] args) {
        // var 让编译器推断局部变量的类型。List.of 和 Map.of 创建的集合不能增删元素。
        var lines = List.of(new Line("Notebook", 1200, 2), new Line("Pen", 300, 1));
        var categories = Map.of("Notebook", "Stationery");

        System.out.printf("total: %d cents%n", total(lines));
        System.out.println("category: " + categories.get("Notebook"));

        if (!categories.containsKey("Pen")) {
            System.out.println("category missing: Pen");
        }
        // 对照 Go 检查 err != nil，这里用 catch 处理 subtotal() 抛出的异常。
        try {
            total(List.of(new Line("Pen", 300, 0)));
        } catch (IllegalArgumentException e) {
            System.out.println("invalid item: " + e.getMessage());
        }
    }
}
