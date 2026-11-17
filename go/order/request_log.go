package order

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"os"
	"time"
)

type requestIDKey struct{}

// RequestID 供同一次 HTTP 调用里的存储或缓存日志使用。
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

func logRequests(next http.Handler) http.Handler {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		id := r.Header.Get("X-Request-ID")
		if !headerIDPattern.MatchString(id) {
			var random [16]byte
			if _, err := rand.Read(random[:]); err != nil {
				logger.Error("generate_request_id", "error", err)
				writeError(w, http.StatusInternalServerError, "internal_error")
				return
			}
			id = hex.EncodeToString(random[:])
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDKey{}, id)

		// 包一层响应 writer，收集下游 handler 真的写出的状态码。
		response := &responseStatus{ResponseWriter: w}
		next.ServeHTTP(response, r.WithContext(ctx))
		if response.status == 0 {
			response.status = http.StatusOK
		}
		logger.Info("http_request", "request_id", id, "method", r.Method,
			"path", r.URL.Path, "status", response.status, "duration_ms", time.Since(started).Milliseconds())
	})
}

type responseStatus struct {
	http.ResponseWriter
	status int
}

func (w *responseStatus) WriteHeader(status int) {
	if w.status == 0 && (status >= 200 || status == http.StatusSwitchingProtocols) {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseStatus) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(data)
}

func (w *responseStatus) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
