package study.springorders;

import java.net.URI;
import java.util.List;
import java.util.Map;
import java.util.regex.Pattern;
import org.slf4j.LoggerFactory;
import org.springframework.dao.DataAccessException;
import org.springframework.http.ResponseEntity;
import org.springframework.http.converter.HttpMessageNotReadableException;
import org.springframework.util.MultiValueMap;
import org.springframework.web.HttpMediaTypeNotSupportedException;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestHeader;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.bind.annotation.RestControllerAdvice;

@RestController
class OrderController {
    record Request(long productId, int quantity) {}
    record Page(List<OrderService.Order> items, Long nextCursor) {}

    static final Pattern REQUEST_TOKEN = Pattern.compile("[a-zA-Z0-9_-]{1,64}");
    private static final Pattern DECIMAL = Pattern.compile("[0-9]+");
    private final OrderService service;

    OrderController(OrderService service) {
        this.service = service;
    }

    @GetMapping("/livez")
    Map<String, String> live() {
        return Map.of("status", "live");
    }

    @GetMapping("/readyz")
    ResponseEntity<Map<String, String>> ready() {
        // 进程能接请求只代表存活。连接 MySQL 才能继续完成业务请求。
        try {
            if (service.ready()) {
                return ResponseEntity.ok(Map.of("status", "ready"));
            }
        } catch (DataAccessException unavailable) {
            LoggerFactory.getLogger(OrderController.class).debug("database readiness failed", unavailable);
        }
        return ResponseEntity.status(503).body(Map.of("status", "not_ready"));
    }

    @GetMapping("/products/{id}")
    OrderService.Product product(@PathVariable String id) {
        return service.product(positiveNumber(id, "invalid_id"));
    }

    @GetMapping("/orders/{id}")
    OrderService.Order order(@PathVariable String id) {
        return service.order(positiveNumber(id, "invalid_id"));
    }

    @PostMapping(path = "/orders", consumes = "application/json")
    ResponseEntity<OrderService.Order> place(@RequestHeader(name = "Idempotency-Key", required = false) String key,
                                             @RequestBody Request request) {
        // Controller 处理 HTTP 校验和状态码，库存与幂等规则交给事务服务。
        if (key == null || !REQUEST_TOKEN.matcher(key).matches()) {
            throw new OrderService.Rejected(400, "invalid_idempotency_key");
        }
        if (request.productId() < 1 || request.quantity() < 1 || request.quantity() > 100) {
            throw new OrderService.Rejected(400, "invalid_order");
        }
        OrderService.Creation result = service.place(key, request.productId(), request.quantity());
        return ResponseEntity.status(result.created() ? 201 : 200)
                .location(URI.create("/orders/" + result.order().id())).body(result.order());
    }

    @GetMapping("/orders")
    Page history(@RequestParam MultiValueMap<String, String> query) {
        int limit = 20;
        long before = 0;
        String error = "invalid_pagination";
        for (var entry : query.entrySet()) {
            if (entry.getValue().size() != 1) {
                throw new OrderService.Rejected(400, error);
            }
            long value = positiveNumber(entry.getValue().getFirst(), error);
            switch (entry.getKey()) {
                case "limit" -> {
                    if (value > 100) {
                        throw new OrderService.Rejected(400, error);
                    }
                    limit = (int) value;
                }
                case "before" -> before = value;
                default -> throw new OrderService.Rejected(400, error);
            }
        }
        // 多查一条判断还有没有下一页，游标取已经返回的最后一个 ID。
        var items = service.history(limit + 1, before);
        return items.size() <= limit ? new Page(items, null)
                : new Page(items.subList(0, limit), items.get(limit - 1).id());
    }

    private static long positiveNumber(String raw, String code) {
        if (DECIMAL.matcher(raw).matches()) {
            try {
                long value = Long.parseLong(raw);
                if (value > 0) {
                    return value;
                }
            } catch (NumberFormatException tooLarge) {
                // 大于 long 范围的输入和其他无效编号一样，返回 400。
            }
        }
        throw new OrderService.Rejected(400, code);
    }
}

@RestControllerAdvice
class OrderErrors {
    @ExceptionHandler(OrderService.Rejected.class)
    ResponseEntity<Map<String, String>> rejected(OrderService.Rejected error) {
        return ResponseEntity.status(error.status).body(Map.of("error", error.getMessage()));
    }

    @ExceptionHandler(HttpMessageNotReadableException.class)
    ResponseEntity<Map<String, String>> invalidBody() {
        return ResponseEntity.badRequest().body(Map.of("error", "invalid_order"));
    }

    @ExceptionHandler(HttpMediaTypeNotSupportedException.class)
    ResponseEntity<Map<String, String>> invalidType() {
        return ResponseEntity.status(415).body(Map.of("error", "expected_json"));
    }

    @ExceptionHandler(DataAccessException.class)
    ResponseEntity<Map<String, String>> databaseError(DataAccessException error) {
        LoggerFactory.getLogger(OrderErrors.class).error("order database request failed", error);
        return ResponseEntity.internalServerError().body(Map.of("error", "internal_error"));
    }
}
