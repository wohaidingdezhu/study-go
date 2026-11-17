package agentruntime

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var keyPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

func Server(addr string, store *Store, releases []Release) *http.Server {
	known := make(map[string]bool, len(releases))
	for _, release := range releases {
		known[release.ID] = true
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		// 就绪探针验证 API 能连接持久化依赖，并为检查自身设置短超时。
		ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
		defer cancel()
		if err := store.db.Ping(ctx); err != nil {
			writeError(w, http.StatusServiceUnavailable, "database_unavailable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("GET /releases", func(w http.ResponseWriter, r *http.Request) {
		active, err := store.ActiveRelease(r.Context())
		if err != nil {
			writeStoreError(w, err)
			return
		}
		type releaseStatus struct {
			Release
			Active bool `json:"active"`
		}
		result := make([]releaseStatus, 0, len(releases))
		for _, release := range releases {
			result = append(result, releaseStatus{release, release.ID == active})
		}
		writeJSON(w, http.StatusOK, result)
	})
	mux.HandleFunc("POST /runs", func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Idempotency-Key")
		if !keyPattern.MatchString(key) {
			writeError(w, http.StatusBadRequest, "invalid_idempotency_key")
			return
		}
		var request struct {
			Input   string `json:"input"`
			Release string `json:"release"`
		}
		// 限制请求体，校验未知字段，并检查正文只包含一个 JSON 值。
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil || decoder.Decode(new(any)) != io.EOF {
			writeError(w, http.StatusBadRequest, "invalid_request")
			return
		}
		request.Input = strings.TrimSpace(request.Input)
		if request.Input == "" || len(request.Input) > 512 || strings.ContainsRune(request.Input, 0) {
			writeError(w, http.StatusBadRequest, "invalid_input")
			return
		}
		if request.Release != "" && !known[request.Release] {
			writeError(w, http.StatusNotFound, "release_not_found")
			return
		}

		run, created, err := store.Create(r.Context(), key, request.Release, request.Input)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		status := http.StatusOK
		// 首次创建只代表已接收任务，调用方再按 Location 查询异步执行结果。
		if created {
			status = http.StatusAccepted
		}
		w.Header().Set("Location", "/runs/"+strconv.FormatInt(run.ID, 10))
		writeJSON(w, status, run)
	})
	mux.HandleFunc("GET /runs/{id}", func(w http.ResponseWriter, r *http.Request) {
		run, ok := readRun(w, r, store)
		if ok {
			writeJSON(w, http.StatusOK, run)
		}
	})
	mux.HandleFunc("GET /runs/{id}/events", func(w http.ResponseWriter, r *http.Request) {
		// after 表示客户端已经读到的最后一个事件序号，后续轮询只取更新的事件。
		after := 0
		if r.URL.Query().Has("after") {
			var err error
			after, err = strconv.Atoi(r.URL.Query().Get("after"))
			if err != nil || after < 0 || after > 2147483647 {
				writeError(w, http.StatusBadRequest, "invalid_cursor")
				return
			}
		}
		run, ok := readRun(w, r, store)
		if !ok {
			return
		}
		events, err := store.Events(r.Context(), run.ID, after)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, events)
	})

	return &http.Server{
		Addr: addr, Handler: http.TimeoutHandler(mux, 4*time.Second, "request timed out"),
		ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 6 * time.Second,
		IdleTimeout: 30 * time.Second,
	}
}

func readRun(w http.ResponseWriter, r *http.Request, store *Store) (Run, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "invalid_run_id")
		return Run{}, false
	}
	run, err := store.Run(r.Context(), id)
	if err != nil {
		writeStoreError(w, err)
		return Run{}, false
	}
	return run, true
}

func writeStoreError(w http.ResponseWriter, err error) {
	// 存储层返回可区分的错误，HTTP 层决定状态码。意外错误的细节留在服务日志。
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		writeError(w, http.StatusNotFound, "run_not_found")
	case errors.Is(err, ErrKeyConflict):
		writeError(w, http.StatusConflict, "idempotency_conflict")
	default:
		log.Printf("agent store: %v", err)
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
