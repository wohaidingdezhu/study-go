package study;

import java.nio.charset.StandardCharsets;
import java.util.HashMap;
import java.util.Map;

public class Catalog {
    static void showStock(Map<Integer, Integer> stockById, int id) {
        // 这里只保存整数。用 Integer 接收结果，就能在自动拆箱前区分 null 和 0。
        Integer stock = stockById.get(id);
        if (stock == null) {
            System.out.printf("product %d: not found%n", id);
            return;
        }
        System.out.printf("product %d: stock=%d%n", id, stock);
    }

    public static void main(String[] args) {
        // Map.of 建立初始条目，HashMap 让练习中的库存可以修改。
        var stockById = new HashMap<>(Map.of(1, 5, 2, 0));
        for (int id : new int[]{1, 2, 999}) {
            showStock(stockById, id);
        }

        // 赋值复制的是引用，两个变量指向同一个 HashMap。
        var sharedStock = stockById;
        sharedStock.put(1, sharedStock.get(1) - 1);
        System.out.printf("original after shared update: %d%n", stockById.get(1));

        // 构造新的 HashMap 会复制条目。这里修改的是副本里的库存值。
        var copiedStock = new HashMap<>(stockById);
        copiedStock.put(1, copiedStock.get(1) - 1);
        System.out.printf("original after copy update: %d%n", stockById.get(1));
        System.out.printf("copy after copy update: %d%n", copiedStock.get(1));

        String name = "笔记本📒";
        // length() 数 UTF-16 代码单元。比较 UTF-8 字节数时要显式指定编码。
        System.out.printf("UTF-16 code units: %d%n", name.length());
        System.out.printf("UTF-8 bytes: %d%n", name.getBytes(StandardCharsets.UTF_8).length);
        System.out.printf("Unicode code points: %d%n", name.codePointCount(0, name.length()));
    }
}
