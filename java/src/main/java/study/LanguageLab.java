package study;

import java.util.ArrayList;
import java.util.List;

public class LanguageLab {
    interface StockReader {
        int available();
    }

    // Java 用 implements 声明接口。这里保留可修改的字段，观察引用共享的效果。
    static final class Inventory implements StockReader {
        int stock;

        Inventory(int stock) {
            this.stock = stock;
        }

        @Override
        public int available() {
            return stock;
        }
    }

    static void valueAndCollectionDemo() {
        var original = new Inventory(5);
        // Java 这里显式构造新对象以复制库存，可对照 Go 结构体赋值时复制字段值。
        var valueCopy = new Inventory(original.stock);
        valueCopy.stock--;
        System.out.printf("value copy: original=%d copy=%d%n", original.stock, valueCopy.stock);

        // 变量赋值复制引用，两个变量都访问同一个对象。Java 的参数传递也按值复制。
        var shared = original;
        shared.stock--;
        System.out.printf("shared reference: original=%d%n", original.stock);

        var source = List.of(original);
        // 新列表复制的是元素引用，元素本身仍共享。List.of 约束列表结构，元素仍可修改。
        var shallow = new ArrayList<>(source);
        shallow.get(0).stock--;
        System.out.printf("shallow snapshot: original=%d%n", original.stock);

        var independent = new ArrayList<Inventory>();
        for (var item : source) {
            independent.add(new Inventory(item.stock));
        }
        independent.get(0).stock--;
        System.out.printf("independent snapshot: original=%d copy=%d%n", original.stock, independent.get(0).stock);

        StockReader reader = original;
        System.out.printf("interface stock: %d%n", reader.available());
    }

    static final class Rejected extends RuntimeException {
        Rejected(String code) {
            super(code);
        }
    }

    static void errorDemo() {
        var outOfStock = new Rejected("out_of_stock");
        // 构造异常时传入 cause，补充当前操作的上下文，同时保留底层业务失败的原因。
        RuntimeException wrapped = new IllegalStateException("reserve product 1", outOfStock);
        System.out.printf("error direct: %b%n", wrapped instanceof Rejected);
        // 本例只包装了一层，因此从 getCause 取得原异常后即可检查类型。
        System.out.printf("error cause: %b%n", wrapped.getCause() instanceof Rejected);
    }

    // AutoCloseable 让 try-with-resources 在退出代码块时调用 close。
    // 这里用打印模拟关闭动作，实际资源可使用相同的生命周期管理方式。
    static final class TraceResource implements AutoCloseable {
        @Override
        public void close() {
            System.out.println("resource: close");
        }
    }

    static void useResource() {
        try (var resource = new TraceResource()) {
            System.out.println("resource: use");
        }
    }

    public static void main(String[] args) {
        valueAndCollectionDemo();
        errorDemo();
        useResource();
        System.out.println("resource: returned");
    }
}
