package order

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"
)

var headerIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
var decimalPattern = regexp.MustCompile(`^[0-9]+$`)

type orderPage struct {
	Items      []Order `json:"items"`
	NextCursor *int64  `json:"nextCursor"`
}

func HTTPServer(addr string, store Store) *http.Server {
	// 路由器根据 HTTP 方法和路径选择处理函数，{id} 捕获路径中的商品或订单编号。
	// handler 是处理一次请求的函数。r 提供请求数据，w 用来写响应状态、响应头和正文。
	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "live"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		// 探针单独使用一个短期限，以免数据库不可用时占住请求。
		ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
		defer cancel()
		if err := store.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("GET /products/{id}", func(w http.ResponseWriter, r *http.Request) {
		// 路径参数来自 URL，例如 /products/1 的 id 为字符串 "1"，先校验再交给业务层。
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_id")
			return
		}
		product, err := store.Product(r.Context(), id)
		writeResult(r.Context(), w, http.StatusOK, product, err)
	})
	mux.HandleFunc("GET /orders/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_id")
			return
		}
		value, err := store.Order(r.Context(), id)
		writeResult(r.Context(), w, http.StatusOK, value, err)
	})
	mux.HandleFunc("GET /orders", func(w http.ResponseWriter, r *http.Request) {
		limit, before, valid := parseOrderPage(r.URL.RawQuery)
		if !valid {
			writeError(w, http.StatusBadRequest, "invalid_pagination")
			return
		}
		// 多读一行判断是否还有下一页，游标仍取本页最后一条。
		items, err := store.ListOrders(r.Context(), limit+1, before)
		if err != nil {
			writeResult(r.Context(), w, http.StatusOK, nil, err)
			return
		}
		page := orderPage{Items: items}
		if len(items) > limit {
			page.Items = items[:limit]
			cursor := page.Items[limit-1].ID
			page.NextCursor = &cursor
		}
		writeJSON(w, http.StatusOK, page)
	})
	mux.HandleFunc("POST /orders", func(w http.ResponseWriter, r *http.Request) {
		// 客户端为同一笔下单重试时复用这个键。存储层负责查找原来创建的订单。
		// 幂等让重复的同一业务请求返回原订单，避免网络超时重试造成再次扣库存。
		key := r.Header.Get("Idempotency-Key")
		if !headerIDPattern.MatchString(key) {
			writeError(w, http.StatusBadRequest, "invalid_idempotency_key")
			return
		}
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			writeError(w, http.StatusUnsupportedMediaType, "expected_json")
			return
		}

		// 先限制请求体的大小，再把 JSON 反序列化到 Go 的 Request。
		// 反序列化把文本字段转换成程序中的结构体字段，可对照 Java 的 JSON 对象映射。
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		var request Request
		if err := decoder.Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_order")
			return
		}
		// 再读一次应该到达输入末尾（EOF），确保请求体里恰好只有一个 JSON 值。
		if err := decoder.Decode(new(any)); err != io.EOF ||
			request.ProductID < 1 || request.Quantity < 1 || request.Quantity > 100 {
			writeError(w, http.StatusBadRequest, "invalid_order")
			return
		}

		// 新建用 201，幂等重放用 200。Location 告诉客户端从哪里查这笔订单。
		value, created, err := store.Create(r.Context(), key, request)
		status := http.StatusOK
		if err == nil {
			w.Header().Set("Location", "/orders/"+strconv.FormatInt(value.ID, 10))
			if created {
				status = http.StatusCreated
			}
		}
		writeResult(r.Context(), w, status, value, err)
	})

	return &http.Server{
		Addr:              addr,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
		Handler: logRequests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 一次请求内，缓存和数据库共享 3 秒时间预算。
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			defer cancel()
			mux.ServeHTTP(w, r.WithContext(ctx))
		})),
	}
}

func parseOrderPage(rawQuery string) (limit int, before int64, valid bool) {
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return 0, 0, false
	}
	limit = 20
	for name, entries := range values {
		if len(entries) != 1 || !decimalPattern.MatchString(entries[0]) {
			return 0, 0, false
		}
		value, err := strconv.ParseInt(entries[0], 10, 64)
		if err != nil || value <= 0 {
			return 0, 0, false
		}
		switch name {
		case "limit":
			if value > 100 {
				return 0, 0, false
			}
			limit = int(value)
		case "before":
			before = value
		default:
			return 0, 0, false
		}
	}
	return limit, before, true
}

func writeResult(ctx context.Context, w http.ResponseWriter, status int, value any, err error) {
	switch {
	case err == nil:
		writeJSON(w, status, value)
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found")
	case errors.Is(err, ErrOutOfStock):
		writeError(w, http.StatusConflict, "out_of_stock")
	case errors.Is(err, ErrKeyConflict):
		writeError(w, http.StatusConflict, "idempotency_conflict")
	default:
		// 服务端日志保留故障细节，客户端得到稳定的错误码。
		log.Printf("order store request_id=%s: %v", RequestID(ctx), err)
		writeError(w, http.StatusInternalServerError, "internal_error")
	}
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write response: %v", err)
	}
}
