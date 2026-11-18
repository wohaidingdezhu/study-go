package study;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;

import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;

public final class AgentHistoryProbe {
    private static final ObjectMapper JSON = new ObjectMapper();

    public static void main(String[] args) throws IOException, InterruptedException {
        URI base = URI.create(args.length > 0 ? args[0] : "http://127.0.0.1:18084");
        try (var client = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(2)).build()) {
            JsonNode first = fetch(client, base.resolve("/runs?limit=2"));
            Long last = printRows("第 1 页", first, null);
            JsonNode cursor = first.required("nextCursor");
            if (!cursor.isNull()) {
                long before = readID(cursor);
                if (last == null || before != last) {
                    throw new IOException("游标必须等于第一页最后一个任务 ID");
                }
                // 复用服务给出的游标，第二页中的每个 ID 都应严格小于它。
                JsonNode second = fetch(client, base.resolve("/runs?limit=2&before=" + before));
                printRows("第 2 页", second, before);
            }
        }
    }

    private static JsonNode fetch(HttpClient client, URI uri) throws IOException, InterruptedException {
        // 建连和整次请求各有限时，先验证状态码再解析 JSON。
        HttpRequest request = HttpRequest.newBuilder(uri).timeout(Duration.ofSeconds(4)).build();
        var response = client.send(request, HttpResponse.BodyHandlers.ofString());
        if (response.statusCode() != 200) {
            throw new IOException("Agent API 返回 HTTP " + response.statusCode());
        }
        return JSON.readTree(response.body());
    }

    private static Long printRows(String title, JsonNode page, Long previous) throws IOException {
        JsonNode items = page.required("items");
        if (!items.isArray()) {
            throw new IOException("items 应为数组");
        }
        System.out.println(title);
        for (JsonNode row : items) {
            long id = readID(row.required("id"));
            if (previous != null && id >= previous) {
                throw new IOException("任务 ID 未严格倒序，可能出现重复记录");
            }
            System.out.printf("  #%d %s %s%n", id, row.required("status").asText(), row.required("input").asText());
            previous = id;
        }
        return previous;
    }

    private static long readID(JsonNode value) throws IOException {
        // Go API 把 bigint 编成字符串。Java 用 long 恢复精度并校验边界。
        if (!value.isTextual() || !value.textValue().matches("[1-9][0-9]*")) {
            throw new IOException("任务 ID 应为十进制字符串");
        }
        try {
            return Long.parseLong(value.textValue());
        } catch (NumberFormatException ex) {
            throw new IOException("任务 ID 超出 long 的范围", ex);
        }
    }
}
