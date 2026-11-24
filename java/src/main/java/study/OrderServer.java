package study;

import com.fasterxml.jackson.databind.DeserializationFeature;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;

import java.io.IOException;
import java.net.InetSocketAddress;
import java.net.URLDecoder;
import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import java.util.regex.Pattern;

public class OrderServer {
    private static final ObjectMapper JSON = new ObjectMapper()
            .enable(DeserializationFeature.FAIL_ON_TRAILING_TOKENS);
    private static final Pattern HEADER_ID = Pattern.compile("[a-zA-Z0-9_-]{1,64}");
    private static final Pattern DECIMAL = Pattern.compile("[0-9]+");

    private record PageRequest(int limit, long before) {}
    private record OrderPage(List<MemoryStore.Order> items, Long nextCursor) {}

    private final MemoryStore store = new MemoryStore();

    public static void main(String[] args) throws IOException {
        // 绑定回环地址只供本机练习。Java 默认用 8081，可以和 Go 的 8080 同时运行。
        int port = args.length == 0 ? 8081 : Integer.parseInt(args[0]);
        var server = HttpServer.create(new InetSocketAddress("127.0.0.1", port), 0);
        var executor = Executors.newVirtualThreadPerTaskExecutor();
        server.setExecutor(executor);
        // HttpServer 将请求交给 handle，再由 route 区分方法和路径。
        server.createContext("/", new OrderServer()::handle);
        Runtime.getRuntime().addShutdownHook(new Thread(() -> {
            server.stop(0);
            executor.shutdown();
        }));
        server.start();
        System.out.println("Java order API: http://127.0.0.1:" + port);
    }

    // 同包测试复用真正的请求入口；每个新 OrderServer 都有独立内存存储。
    void handle(HttpExchange exchange) throws IOException {
        long started = System.nanoTime();
        var suppliedId = exchange.getRequestHeaders().getFirst("X-Request-ID");
        var requestId = suppliedId != null && HEADER_ID.matcher(suppliedId).matches()
                ? suppliedId : UUID.randomUUID().toString();
        exchange.getResponseHeaders().set("X-Request-ID", requestId);

        // exchange 封装一次请求与响应，退出 try 时关闭它。业务拒绝统一转换成 JSON。
        try (exchange) {
            try {
                route(exchange);
            } catch (MemoryStore.Rejected rejected) {
                write(exchange, rejected.status, Map.of("error", rejected.code));
            }
        } finally {
            // 日志使用和响应头相同的 ID。耗时用单调时钟计算。
            var row = JSON.createObjectNode();
            row.put("time", Instant.now().toString());
            row.put("msg", "http_request");
            row.put("request_id", requestId);
            row.put("method", exchange.getRequestMethod());
            row.put("path", exchange.getRequestURI().getPath());
            row.put("status", exchange.getResponseCode());
            row.put("duration_ms", TimeUnit.NANOSECONDS.toMillis(System.nanoTime() - started));
            System.out.println(row);
        }
    }

    private void route(HttpExchange exchange) throws IOException {
        var path = exchange.getRequestURI().getPath();
        var method = exchange.getRequestMethod();
        if (path.equals("/livez") || path.equals("/readyz")) {
            requireMethod(exchange, method, "GET");
            // 入门服务的存储在进程内，启动后可以直接接请求。
            write(exchange, 200, Map.of("status", path.equals("/livez") ? "live" : "ready"));
            return;
        }
        if (path.equals("/orders")) {
            switch (method) {
                case "GET" -> list(exchange);
                case "POST" -> create(exchange);
                default -> {
                    exchange.getResponseHeaders().set("Allow", "GET, POST");
                    throw new MemoryStore.Rejected(405, "method_not_allowed");
                }
            }
            return;
        }
        var parts = path.split("/", -1);
        if (parts.length == 3 && !parts[2].isEmpty()) {
            if (parts[1].equals("products")) {
                requireMethod(exchange, method, "GET");
                write(exchange, 200, store.product(id(parts[2])));
                return;
            }
            if (parts[1].equals("orders")) {
                requireMethod(exchange, method, "GET");
                write(exchange, 200, store.order(id(parts[2])));
                return;
            }
        }
        throw new MemoryStore.Rejected(404, "not_found");
    }

    private void list(HttpExchange exchange) throws IOException {
        var request = pageRequest(exchange.getRequestURI().getRawQuery());
        int limit = request.limit();
        var items = store.listOrders(limit + 1, request.before());
        // 额外查到的订单用于判断还有下一页，本次返回仍限制为 limit 条。
        var page = items.size() > limit
                ? new OrderPage(items.subList(0, limit), items.get(limit - 1).id())
                : new OrderPage(items, null);
        write(exchange, 200, page);
    }

    private static PageRequest pageRequest(String rawQuery) {
        var values = new HashMap<String, Long>();
        if (rawQuery != null) {
            try {
                for (String pair : rawQuery.split("&")) {
                    if (pair.isEmpty()) {
                        continue;
                    }
                    // 先识别参数的边界，再解码各参数，避免把编码后的 & 当成分隔符。
                    var parts = pair.split("=", 2);
                    if (parts.length != 2) {
                        throw new IllegalArgumentException();
                    }
                    var name = URLDecoder.decode(parts[0], StandardCharsets.UTF_8);
                    var rawValue = URLDecoder.decode(parts[1], StandardCharsets.UTF_8);
                    if ((!name.equals("limit") && !name.equals("before"))
                            || !DECIMAL.matcher(rawValue).matches()) {
                        throw new IllegalArgumentException();
                    }
                    long value = Long.parseLong(rawValue);
                    if (value <= 0 || values.putIfAbsent(name, value) != null) {
                        throw new IllegalArgumentException();
                    }
                }
            } catch (IllegalArgumentException ignored) {
                throw new MemoryStore.Rejected(400, "invalid_pagination");
            }
        }
        long limit = values.getOrDefault("limit", 20L);
        if (limit > 100) {
            throw new MemoryStore.Rejected(400, "invalid_pagination");
        }
        return new PageRequest((int) limit, values.getOrDefault("before", 0L));
    }

    private void create(HttpExchange exchange) throws IOException {
        // 幂等键由客户端生成。同一笔下单在网络超时后可用原键和原参数重试。
        var key = exchange.getRequestHeaders().getFirst("Idempotency-Key");
        if (key == null || !HEADER_ID.matcher(key).matches()) {
            throw new MemoryStore.Rejected(400, "invalid_idempotency_key");
        }
        var type = exchange.getRequestHeaders().getFirst("Content-Type");
        if (type == null || !type.split(";", 2)[0].trim().equalsIgnoreCase("application/json")) {
            throw new MemoryStore.Rejected(415, "expected_json");
        }

        // 读取上限比允许的 4096 字节多 1，用于识别超长请求，避免读完整个请求体。
        var body = exchange.getRequestBody().readNBytes(4097);
        if (body.length > 4096) {
            throw new MemoryStore.Rejected(400, "invalid_order");
        }
        JsonNode request;
        try {
            request = JSON.readTree(body);
        } catch (IOException e) {
            throw new MemoryStore.Rejected(400, "invalid_order");
        }
        if (request == null) {
            throw new MemoryStore.Rejected(400, "invalid_order");
        }
        // Jackson 把请求变成 JSON 树。先校验字段与数值范围，再构造业务用的 Request。
        var product = request.path("productId");
        var quantity = request.path("quantity");
        if (request.size() != 2 || !product.isIntegralNumber() || !product.canConvertToLong()
                || product.longValue() < 1 || !quantity.isIntegralNumber() || !quantity.canConvertToInt()
                || quantity.intValue() < 1 || quantity.intValue() > 100) {
            throw new MemoryStore.Rejected(400, "invalid_order");
        }
        // Location 指向可查询的订单。三元表达式在新建时选 201，重试时选 200。
        var result = store.create(key, new MemoryStore.Request(product.longValue(), quantity.intValue()));
        exchange.getResponseHeaders().set("Location", "/orders/" + result.order().id());
        write(exchange, result.created() ? 201 : 200, result.order());
    }

    private static long id(String raw) {
        try {
            long id = Long.parseLong(raw);
            if (id > 0) {
                return id;
            }
        } catch (NumberFormatException ignored) {
            // 非整数或超出 long 的范围，都用同一个 400 响应。
        }
        throw new MemoryStore.Rejected(400, "invalid_id");
    }

    private static void requireMethod(HttpExchange exchange, String method, String expected) {
        if (!method.equals(expected)) {
            exchange.getResponseHeaders().set("Allow", expected);
            throw new MemoryStore.Rejected(405, "method_not_allowed");
        }
    }

    private static void write(HttpExchange exchange, int status, Object value) throws IOException {
        byte[] body = JSON.writeValueAsBytes(value);
        exchange.getResponseHeaders().set("Content-Type", "application/json; charset=utf-8");
        exchange.sendResponseHeaders(status, body.length);
        exchange.getResponseBody().write(body);
    }
}
