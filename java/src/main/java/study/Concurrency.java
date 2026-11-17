package study;

import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;

public class Concurrency {
    static class Inventory {
        private int stock = 3;

        // 同一个 Inventory 实例上，每次只允许一个线程进入该方法。
        // 检查库存与扣库存必须一起受保护，对照 Go 的 Mutex。
        synchronized boolean reserve() {
            if (stock == 0) {
                return false;
            }
            stock--;
            return true;
        }
    }

    public static void main(String[] args) throws Exception {
        int attempts = 10;
        var inventory = new Inventory();
        int succeeded = 0;

        // 执行器为每个任务启动一个虚拟线程。try-with-resources 在退出时关闭执行器。
        try (var executor = Executors.newVirtualThreadPerTaskExecutor()) {
            List<Future<Boolean>> results = new ArrayList<>();
            for (int i = 0; i < attempts; i++) {
                // inventory::reserve 把方法交给执行器稍后调用，Future 代表尚未取到的结果。
                results.add(executor.submit(inventory::reserve));
            }
            // 所有任务已提交，逐个 get() 只是等待结果，不会让任务变成逐个启动。
            for (var result : results) {
                if (result.get()) {
                    succeeded++;
                }
            }
        }

        System.out.printf("succeeded: %d, rejected: %d, stock: %d%n",
                succeeded, attempts - succeeded, inventory.stock);
    }
}
