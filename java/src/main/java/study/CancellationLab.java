package study;

import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;

public class CancellationLab {
    static int readStock(String mode) throws InterruptedException {
        // sleep 用于模拟等待。它会响应线程中断，让取消请求可以推进到收尾。
        Thread.sleep(mode.equals("timeout") ? 2000 : 20);
        if (mode.equals("failure")) {
            throw new IllegalStateException("dependency_failed");
        }
        return 5;
    }

    public static void main(String[] args) throws InterruptedException {
        String mode = args.length == 0 ? "success" : args[0];
        if (args.length > 1 || !(mode.equals("success") || mode.equals("timeout") || mode.equals("failure"))) {
            throw new IllegalArgumentException("usage: CancellationLab [success|timeout|failure]");
        }

        // CountDownLatch 是一次性倒计数信号，await 等待计数归零，countDown 将计数减一。
        // 分开记录启动和收尾，便于对照调用者的等待状态与工作函数的执行状态。
        var started = new CountDownLatch(1);
        var finished = new CountDownLatch(1);
        try (var executor = Executors.newVirtualThreadPerTaskExecutor()) {
            var future = executor.submit(() -> {
                // 先确认任务已经启动，便于观察取消运行中任务时的中断与 finally。
                started.countDown();
                try {
                    return readStock(mode);
                } finally {
                    finished.countDown();
                }
            });
            started.await();
            try {
                // 此处限制这一次 get 的等待时长，等待超时以后还需要明确请求任务取消。
                int stock = future.get(200, TimeUnit.MILLISECONDS);
                System.out.printf("result: stock=%d%n", stock);
            } catch (TimeoutException timeout) {
                System.out.println("result: timeout");
                // true 表示允许中断正在执行的线程，任务是否及时结束取决于它如何响应中断。
                System.out.printf("cancel requested: %b%n", future.cancel(true));
            } catch (ExecutionException failure) {
                // 工作线程的异常通过 Future 交给调用者，getCause 保留本例的原始失败原因。
                System.out.println("result: " + failure.getCause().getMessage());
            }
            // Future 的取消状态与工作函数收尾分开观察，等待 finally 发出完成信号。
            finished.await();
            System.out.println("worker cleanup: finished");
        }
    }
}
