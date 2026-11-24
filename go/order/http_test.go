package order_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"study-go/order"
)

const validOrder = `{"productId":1,"quantity":1}`

// recorder 记录 Handler 写出的响应，不监听端口，也不使用正在运行的学习服务。
func callHTTP(t *testing.T, handler http.Handler, method, path, key, contentType, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("X-Request-ID", "http-contract")
	if key != "" {
		request.Header.Set("Idempotency-Key", key)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if !strings.HasPrefix(response.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("response must be JSON: %v", response.Header())
	}
	if response.Header().Get("X-Request-ID") != "http-contract" {
		t.Fatalf("request ID was not returned: %v", response.Header())
	}
	return response
}

func responseJSON[T any](t *testing.T, response *httptest.ResponseRecorder, status int) T {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status=%d body=%s; want %d", response.Code, response.Body, status)
	}
	var value T
	if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return value
}

func TestHTTPRejectsInvalidOrders(t *testing.T) {
	// 表格驱动：每行改变一种输入，检查稳定的状态码和业务错误码。
	cases := []struct {
		name, key, contentType, body string
		status                       int
		code                         string
	}{
		{"missing_key", "", "application/json", validOrder, 400, "invalid_idempotency_key"},
		{"invalid_key", "bad key", "application/json", validOrder, 400, "invalid_idempotency_key"},
		{"wrong_media_type", "contract", "text/plain", validOrder, 415, "expected_json"},
		{"malformed_json", "contract", "application/json", `{`, 400, "invalid_order"},
		{"empty_body", "contract", "application/json", "", 400, "invalid_order"},
		{"null_body", "contract", "application/json", `null`, 400, "invalid_order"},
		{"array_body", "contract", "application/json", `[]`, 400, "invalid_order"},
		{"missing_quantity", "contract", "application/json", `{"productId":1}`, 400, "invalid_order"},
		{"unknown_field", "contract", "application/json", `{"productId":1,"quantity":1,"price":1}`, 400, "invalid_order"},
		{"zero_quantity", "contract", "application/json", `{"productId":1,"quantity":0}`, 400, "invalid_order"},
		{"large_quantity", "contract", "application/json", `{"productId":1,"quantity":101}`, 400, "invalid_order"},
		{"string_quantity", "contract", "application/json", `{"productId":1,"quantity":"1"}`, 400, "invalid_order"},
		{"fractional_quantity", "contract", "application/json", `{"productId":1,"quantity":1.5}`, 400, "invalid_order"},
		{"overflow_product", "contract", "application/json", `{"productId":9223372036854775808,"quantity":1}`, 400, "invalid_order"},
		{"trailing_json", "contract", "application/json", validOrder + `{}`, 400, "invalid_order"},
		{"oversized_body", "contract", "application/json", validOrder + strings.Repeat(" ", 4097), 400, "invalid_order"},
		{"missing_product", "contract", "application/json", `{"productId":999,"quantity":1}`, 404, "not_found"},
		{"out_of_stock", "contract", "application/json", `{"productId":1,"quantity":6}`, 409, "out_of_stock"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// 每个子测试有自己的库存和幂等键空间，执行顺序不会影响结果。
			handler := order.HTTPServer("", order.NewMemoryStore()).Handler
			response := callHTTP(t, handler, "POST", "/orders", tc.key, tc.contentType, tc.body)
			failure := responseJSON[map[string]string](t, response, tc.status)
			if failure["error"] != tc.code || response.Header().Get("Location") != "" {
				t.Fatalf("failure=%v location=%q; want error=%s and no Location", failure, response.Header().Get("Location"), tc.code)
			}
			product := responseJSON[order.Product](t, callHTTP(t, handler, "GET", "/products/1", "", "", ""), 200)
			page := responseJSON[struct {
				Items []order.Order `json:"items"`
			}](t, callHTTP(t, handler, "GET", "/orders", "", "", ""), 200)
			if product.Stock != 5 || len(page.Items) != 0 {
				t.Fatalf("rejected request changed state: stock=%d orders=%v", product.Stock, page.Items)
			}
			// 拒绝不能占用合法键：纠正输入后仍可以用这个键首次下单。
			responseJSON[order.Order](t, callHTTP(t, handler, "POST", "/orders", "contract", "application/json", validOrder), 201)
		})
	}
}

func TestHTTPOrderReplayContract(t *testing.T) {
	handler := order.HTTPServer("", order.NewMemoryStore()).Handler
	first := callHTTP(t, handler, "POST", "/orders", "contract", "application/json; charset=utf-8", validOrder)
	created := responseJSON[order.Order](t, first, 201)
	if created.ProductID != 1 || created.Quantity != 1 || created.ID <= 0 {
		t.Fatalf("unexpected order: %+v", created)
	}
	location := first.Header().Get("Location")
	if location != "/orders/"+strconv.FormatInt(created.ID, 10) {
		t.Fatalf("Location=%q does not identify order %d", location, created.ID)
	}
	// 按 Location 查回整笔订单，验证响应头可实际使用。
	loaded := responseJSON[order.Order](t, callHTTP(t, handler, "GET", location, "", "", ""), 200)
	replay := callHTTP(t, handler, "POST", "/orders", "contract", "application/json", validOrder)
	replayed := responseJSON[order.Order](t, replay, 200)
	if loaded != created || replayed != created || replay.Header().Get("Location") != location {
		t.Fatalf("read/replay differs: created=%+v loaded=%+v replayed=%+v", created, loaded, replayed)
	}
	conflict := callHTTP(t, handler, "POST", "/orders", "contract", "application/json", `{"productId":1,"quantity":2}`)
	if value := responseJSON[map[string]string](t, conflict, 409); value["error"] != "idempotency_conflict" {
		t.Fatalf("wrong conflict: %v", value)
	}
	product := responseJSON[order.Product](t, callHTTP(t, handler, "GET", "/products/1", "", "", ""), 200)
	page := responseJSON[struct {
		Items []order.Order `json:"items"`
	}](t, callHTTP(t, handler, "GET", "/orders", "", "", ""), 200)
	if product.Stock != 4 || len(page.Items) != 1 || page.Items[0] != created {
		t.Fatalf("retry/conflict changed state: stock=%d orders=%v", product.Stock, page.Items)
	}
}
