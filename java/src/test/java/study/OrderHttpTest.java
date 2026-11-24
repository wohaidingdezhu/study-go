package study;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.sun.net.httpserver.HttpServer;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

import java.net.InetSocketAddress;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.stream.Stream;

import static org.junit.jupiter.api.Assertions.*;

class OrderHttpTest {
    private static final ObjectMapper JSON = new ObjectMapper();
    private static final String VALID_ORDER = "{\"productId\":1,\"quantity\":1}";
    private HttpServer server;
    private ExecutorService executor;
    private HttpClient client;
    private URI base;

    @BeforeEach
    void startIsolatedServer() throws Exception {
        // 端口 0 让系统分配空闲端口；不连接 8081，不修改正在学习的订单。
        server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
        executor = Executors.newVirtualThreadPerTaskExecutor();
        server.setExecutor(executor);
        server.createContext("/", new OrderServer()::handle);
        server.start();
        base = URI.create("http://127.0.0.1:" + server.getAddress().getPort());
        client = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(2)).build();
    }

    @AfterEach
    void stopIsolatedServer() {
        // 即使断言失败也执行清理，避免留下测试端口、客户端和线程。
        if (client != null) client.close();
        if (server != null) server.stop(0);
        if (executor != null) executor.close();
    }

    static Stream<Arguments> invalidOrders() {
        // 与 Go 表格使用同样的 18 种输入。每行都是独立的参数化测试。
        return Stream.of(
                Arguments.of("missing_key", "", "application/json", VALID_ORDER, 400, "invalid_idempotency_key"),
                Arguments.of("invalid_key", "bad key", "application/json", VALID_ORDER, 400, "invalid_idempotency_key"),
                Arguments.of("wrong_media_type", "contract", "text/plain", VALID_ORDER, 415, "expected_json"),
                Arguments.of("malformed_json", "contract", "application/json", "{", 400, "invalid_order"),
                Arguments.of("empty_body", "contract", "application/json", "", 400, "invalid_order"),
                Arguments.of("null_body", "contract", "application/json", "null", 400, "invalid_order"),
                Arguments.of("array_body", "contract", "application/json", "[]", 400, "invalid_order"),
                Arguments.of("missing_quantity", "contract", "application/json", "{\"productId\":1}", 400, "invalid_order"),
                Arguments.of("unknown_field", "contract", "application/json", "{\"productId\":1,\"quantity\":1,\"price\":1}", 400, "invalid_order"),
                Arguments.of("zero_quantity", "contract", "application/json", "{\"productId\":1,\"quantity\":0}", 400, "invalid_order"),
                Arguments.of("large_quantity", "contract", "application/json", "{\"productId\":1,\"quantity\":101}", 400, "invalid_order"),
                Arguments.of("string_quantity", "contract", "application/json", "{\"productId\":1,\"quantity\":\"1\"}", 400, "invalid_order"),
                Arguments.of("fractional_quantity", "contract", "application/json", "{\"productId\":1,\"quantity\":1.5}", 400, "invalid_order"),
                Arguments.of("overflow_product", "contract", "application/json", "{\"productId\":9223372036854775808,\"quantity\":1}", 400, "invalid_order"),
                Arguments.of("trailing_json", "contract", "application/json", VALID_ORDER + "{}", 400, "invalid_order"),
                Arguments.of("oversized_body", "contract", "application/json", VALID_ORDER + " ".repeat(4097), 400, "invalid_order"),
                Arguments.of("missing_product", "contract", "application/json", "{\"productId\":999,\"quantity\":1}", 404, "not_found"),
                Arguments.of("out_of_stock", "contract", "application/json", "{\"productId\":1,\"quantity\":6}", 409, "out_of_stock")
        );
    }

    @ParameterizedTest(name = "{0}")
    @MethodSource("invalidOrders")
    void rejectsInvalidOrders(String name, String key, String type, String body, int status, String code) throws Exception {
        var rejected = call("POST", "/orders", key, type, body);
        assertEquals(code, json(rejected, status).path("error").asText());
        assertTrue(rejected.headers().firstValue("Location").isEmpty());
        // 只检查错误响应不够，还要通过查询证明库存和订单没有改变。
        assertEquals(5, json(call("GET", "/products/1", "", "", ""), 200).path("stock").asInt());
        assertEquals(0, json(call("GET", "/orders", "", "", ""), 200).path("items").size());
        json(call("POST", "/orders", "contract", "application/json", VALID_ORDER), 201);
    }

    @Test
    void orderReplayContract() throws Exception {
        var first = call("POST", "/orders", "contract", "application/json; charset=utf-8", VALID_ORDER);
        var created = json(first, 201);
        assertTrue(created.path("id").asLong() > 0);
        assertEquals(1, created.path("productId").asLong());
        assertEquals(1, created.path("quantity").asInt());
        var location = first.headers().firstValue("Location").orElseThrow();
        assertEquals("/orders/" + created.path("id").asLong(), location);
        assertEquals(created, json(call("GET", location, "", "", ""), 200));
        var replay = call("POST", "/orders", "contract", "application/json", VALID_ORDER);
        assertEquals(created, json(replay, 200));
        assertEquals(location, replay.headers().firstValue("Location").orElseThrow());
        var conflict = call("POST", "/orders", "contract", "application/json", "{\"productId\":1,\"quantity\":2}");
        assertEquals("idempotency_conflict", json(conflict, 409).path("error").asText());
        assertEquals(4, json(call("GET", "/products/1", "", "", ""), 200).path("stock").asInt());
        var items = json(call("GET", "/orders", "", "", ""), 200).path("items");
        assertEquals(1, items.size());
        assertEquals(created, items.get(0));
    }

    private HttpResponse<String> call(String method, String path, String key, String type, String body) throws Exception {
        var request = HttpRequest.newBuilder(base.resolve(path)).timeout(Duration.ofSeconds(2))
                .header("X-Request-ID", "http-contract");
        if (!key.isEmpty()) request.header("Idempotency-Key", key);
        if (!type.isEmpty()) request.header("Content-Type", type);
        request.method(method, HttpRequest.BodyPublishers.ofString(body));
        var response = client.send(request.build(), HttpResponse.BodyHandlers.ofString());
        assertTrue(response.headers().firstValue("Content-Type").orElse("").startsWith("application/json"));
        assertEquals("http-contract", response.headers().firstValue("X-Request-ID").orElseThrow());
        return response;
    }

    static Stream<String> invalidQueries() {
        // 同 Go 的 14 个边界：包含编码后的重复键，以及编码后的分隔符。
        return Stream.of("limit=0", "limit=101", "limit=-1", "limit=1.5", "limit=", "before=0", "before=",
                "before=9223372036854775808", "unknown=1", "limit=1&limit=2", "limit=1&%6cimit=2",
                "limit=2%26before%3D3", "=1", "limit");
    }

    @ParameterizedTest(name = "{0}")
    @MethodSource("invalidQueries")
    void rejectsInvalidPagination(String query) throws Exception {
        var failure = json(call("GET", "/orders?" + query, "", "", ""), 400);
        assertEquals("invalid_pagination", failure.path("error").asText());
    }

    @Test
    void paginationCursorContract() throws Exception {
        assertPage(readPage("/orders"), new long[]{}, null);
        var ids = new long[4];
        for (int n = 0; n < 4; n++) {
            ids[n] = json(call("POST", "/orders", "page-" + n, "application/json", VALID_ORDER), 201).path("id").asLong();
        }
        var first = readPage("/orders?limit=2");
        assertPage(first, new long[]{ids[3], ids[2]}, ids[2]);
        // 插入第五笔后，旧游标仍限定 id < 本页最后一个 ID，不是固定查询快照。
        long newest = json(call("POST", "/orders", "page-new", "application/json", VALID_ORDER), 201).path("id").asLong();
        assertPage(readPage("/orders?limit=2&before=" + first.path("nextCursor").asLong()), new long[]{ids[1], ids[0]}, null);
        assertPage(readPage("/orders?limit=1&before=" + ids[1]), new long[]{ids[0]}, null);
        assertPage(readPage("/orders?before=" + ids[0]), new long[]{}, null);
        assertPage(readPage("/orders?limit=1"), new long[]{newest}, newest);
    }

    private JsonNode readPage(String path) throws Exception {
        return json(call("GET", path, "", "", ""), 200);
    }

    private void assertPage(JsonNode page, long[] ids, Long cursor) {
        // 检查具体 ID 和顺序，而不是只检查条数，才能发现重复、遗漏或错序。
        var items = page.path("items");
        assertTrue(items.isArray());
        assertEquals(ids.length, items.size());
        for (int i = 0; i < ids.length; i++) assertEquals(ids[i], items.get(i).path("id").asLong());
        if (cursor == null) assertTrue(page.path("nextCursor").isNull());
        else assertEquals(cursor.longValue(), page.path("nextCursor").asLong());
    }

    private JsonNode json(HttpResponse<String> response, int status) throws Exception {
        assertEquals(status, response.statusCode(), response.body());
        return JSON.readTree(response.body());
    }
}
