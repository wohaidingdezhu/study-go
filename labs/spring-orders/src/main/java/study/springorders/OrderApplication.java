package study.springorders;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

@SpringBootApplication
public class OrderApplication {
    public static void main(String[] args) {
        // 启动 Web 服务器，并由 Spring 创建 Controller、Service 和 JdbcTemplate。
        SpringApplication.run(OrderApplication.class, args);
    }
}
