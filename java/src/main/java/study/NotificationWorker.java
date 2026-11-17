package study;

import com.fasterxml.jackson.databind.DeserializationFeature;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import com.rabbitmq.client.ConnectionFactory;
import com.rabbitmq.client.Delivery;

import java.io.IOException;
import java.sql.DriverManager;
import java.sql.SQLException;
import java.util.concurrent.CountDownLatch;

public class NotificationWorker {
    private static final String QUEUE = "order.created";
    private static final ObjectMapper JSON = new ObjectMapper()
            .enable(DeserializationFeature.FAIL_ON_TRAILING_TOKENS);

    // 字段与 Go relay 发布的 JSON 一一对应，两种语言通过这个约定协作。
    private record OrderCreated(long eventId, String type, long orderId) {}

    public static void main(String[] args) throws Exception {
        var factory = new ConnectionFactory();
        factory.setUri(environment("RABBITMQ_URL"));
        factory.setConnectionTimeout(5000);
        // 连接断开时让进程退出，由 Compose 重新启动并连接。
        factory.setAutomaticRecoveryEnabled(false);

        String database = environment("NOTIFICATIONS_JDBC_URL");
        String user = environment("NOTIFICATIONS_DB_USER");
        String password = environment("NOTIFICATIONS_DB_PASSWORD");
        try (var connection = factory.newConnection("java-notifications");
             var channel = connection.createChannel()) {
            var stopped = new CountDownLatch(1);
            channel.addShutdownListener(reason -> stopped.countDown());
            // 队列由 Go relay 声明，passive 只检查它是否存在。
            channel.queueDeclarePassive(QUEUE);
            // 每次最多 1 条未确认消息。basicConsume 的第二个参数 false 表示手动确认。
            channel.basicQos(1);
            channel.basicConsume(QUEUE, false,
                    (tag, delivery) -> process(channel, delivery, database, user, password),
                    tag -> stopped.countDown());
            System.out.println("Listening for " + QUEUE);
            stopped.await();
            throw new IllegalStateException("RabbitMQ consumer stopped");
        }
    }

    private static void process(Channel channel, Delivery delivery, String database,
                                String user, String password) throws IOException {
        // tag 标识当前 channel 上的一次投递，用来 ACK 或拒绝。eventId 才是业务去重的依据。
        long tag = delivery.getEnvelope().getDeliveryTag();
        OrderCreated event;
        try {
            event = JSON.readValue(delivery.getBody(), OrderCreated.class);
            if (event == null || event.eventId() < 1 || event.orderId() < 1
                    || !"order.created.v1".equals(event.type())) {
                throw new IOException("invalid order.created.v1 event");
            }
        } catch (IOException e) {
            // 消息格式错了，重复投递也不会修好。false 告诉 RabbitMQ 把它转入死信流程。
            System.err.println("Rejecting invalid event: " + e.getMessage());
            channel.basicReject(tag, false);
            return;
        }

        int inserted;
        try (var connection = DriverManager.getConnection(database, user, password);
             var statement = connection.prepareStatement("""
                     INSERT INTO order_notifications (event_id, order_id)
                     VALUES (?, ?)
                     ON CONFLICT (event_id) DO NOTHING
                     """)) {
            // JDBC 用 ? 绑定参数。唯一主键配合 ON CONFLICT 使相同事件只落库一次。
            statement.setLong(1, event.eventId());
            statement.setLong(2, event.orderId());
            // 连接默认自动提交，这条写入执行成功时就提交了。
            inserted = statement.executeUpdate();
        } catch (SQLException e) {
            // 数据库故障可能恢复，短暂等待后用 true 要求重新入队。
            System.err.println("Retrying event " + event.eventId() + ": SQLSTATE " + e.getSQLState());
            try {
                Thread.sleep(1000);
            } catch (InterruptedException interrupted) {
                Thread.currentThread().interrupt();
                channel.abort();
                return;
            }
            channel.basicReject(tag, true);
            return;
        }

        // 先落库再 ACK。若两者之间断线，消息可以重投，数据库会识别重复事件。
        channel.basicAck(tag, false);
        System.out.printf("%s event %d for order %d%n",
                inserted == 1 ? "Processed" : "Ignored duplicate", event.eventId(), event.orderId());
    }

    private static String environment(String name) {
        String value = System.getenv(name);
        if (value == null || value.isBlank()) {
            throw new IllegalArgumentException(name + " is required");
        }
        return value;
    }
}
