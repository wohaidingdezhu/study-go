package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"study-go-stack/agentruntime"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	// 评估报告走 stdout，命令错误走 stderr，调用方可直接读取 JSON 报告。
	cliErrors := log.New(os.Stderr, "", 0)
	if len(os.Args) < 2 {
		cliErrors.Fatal("usage: agent-runtime api | worker | eval [release] | replay | publish <expected> <candidate>")
	}
	// 本地 Ctrl+C 和容器的 SIGTERM 都会取消运行中的工作，逐步退出。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1], os.Args[2:]); err != nil {
		cliErrors.Fatal(err)
	}
}

func run(ctx context.Context, mode string, args []string) error {
	if mode == "replay" {
		if len(args) != 0 {
			return errors.New("usage: agent-runtime replay")
		}
		return json.NewEncoder(os.Stdout).Encode(agentruntime.BuildReplay(ctx))
	}
	engines := make(map[string]agentruntime.Engine)
	reports := make(map[string]agentruntime.EvalReport)
	var releases []agentruntime.Release
	// 所有角色使用同一份评估结果。评估未通过的版本不能接任务或获得默认流量。
	for _, engine := range agentruntime.Catalog() {
		report := agentruntime.Evaluate(ctx, engine)
		reports[engine.Release.ID] = report
		if report.Passed {
			engines[engine.Release.ID] = engine
			releases = append(releases, engine.Release)
		}
	}
	switch mode {
	case "eval", "publish":
		if (mode == "eval" && len(args) > 1) || (mode == "publish" && len(args) != 2) {
			return errors.New("usage: eval [release] | publish <expected> <candidate>")
		}
		id := agentruntime.BaselineRelease
		if len(args) > 0 {
			id = args[len(args)-1]
		}
		report, ok := reports[id]
		if !ok {
			return fmt.Errorf("unknown release %q", id)
		}
		if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
			return err
		}
		if !report.Passed {
			return agentruntime.ErrReleaseGate
		}
		if mode == "eval" {
			return nil
		}
	case "api", "worker":
		if len(args) > 0 {
			return fmt.Errorf("unexpected arguments for %s", mode)
		}
	default:
		return fmt.Errorf("unknown mode %q", mode)
	}

	dsn := os.Getenv("AGENT_DATABASE_URL")
	if dsn == "" {
		return errors.New("AGENT_DATABASE_URL is required")
	}
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	store := agentruntime.NewStore(db)
	initCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := store.Init(initCtx); err != nil {
		return fmt.Errorf("initialize runtime schema: %w", err)
	}
	if mode == "publish" {
		return store.Publish(initCtx, args[0], reports[args[1]])
	}
	active, err := store.ActiveRelease(initCtx)
	if err != nil {
		return err
	}
	if _, ok := engines[active]; !ok {
		return fmt.Errorf("active release %q is unavailable", active)
	}
	cancel()

	if mode == "worker" {
		var delay time.Duration
		if raw := os.Getenv("AGENT_CHECKPOINT_DELAY"); raw != "" {
			delay, err = time.ParseDuration(raw)
			if err != nil || delay < 0 || delay > 5*time.Second {
				return errors.New("AGENT_CHECKPOINT_DELAY must be between 0s and 5s")
			}
		}
		agentruntime.Work(ctx, store, engines, delay)
		return nil
	}

	// Compose 将此端口映射到宿主机回环地址。
	server := agentruntime.Server(":8084", store, releases)
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		// 原运行 context 已被取消，用新的短预算让正在处理的 HTTP 请求收尾。
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			slog.Error("http_shutdown_failed", "error", err)
		}
	}()
	err = server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		<-shutdownDone
		return nil
	}
	return err
}
