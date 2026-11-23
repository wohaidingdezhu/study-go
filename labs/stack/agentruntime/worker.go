package agentruntime

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

// 租约（lease）给 worker 一段有期限的任务占用权，超时后其他 worker 可重新领取。
// worker 崩溃时无法主动交还任务，租约到期让任务仍能继续。每次领取只负责一个步骤。
const leaseDuration = 6 * time.Second

// Work 是后台执行循环。API 先接收任务，worker 再领取、计算和提交，适合跨多步的任务。
func Work(ctx context.Context, store *Store, engines map[string]Engine, checkpointDelay time.Duration) {
	releases := make([]string, 0, len(engines))
	for release := range engines {
		releases = append(releases, release)
	}
	for ctx.Err() == nil {
		// WithTimeout 从父 context 派生一个带截止时间的调用预算。
		// 调用后执行 cancel 释放定时器，也让相关子操作收到取消信号。
		claimCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		run, err := store.Claim(claimCtx, releases, leaseDuration)
		cancel()
		if err != nil {
			if ctx.Err() == nil && !errors.Is(err, pgx.ErrNoRows) {
				slog.Error("claim_failed", "error", err)
			}
			// 队列空或暂时读库失败时稍等再领，避免空转占满 CPU 或打满数据库。
			wait(ctx, 200*time.Millisecond)
			continue
		}

		// 单步执行预算短于租约，正常情况下还留有时间提交结果。
		stepCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		// 每一步都读取任务创建时记录的版本，默认版本切换不会影响已接收的任务。
		result := engines[run.Release].Advance(stepCtx, run.Input, run.Step, run.State)
		cancel()
		slog.Info("step_computed", "run_id", run.ID, "epoch", run.LeaseEpoch, "kind", result.Kind)
		// 故障演示可以让 worker 在计算完成后、写 checkpoint 前停留一段时间。
		if checkpointDelay > 0 && !wait(ctx, checkpointDelay) {
			return
		}
		commitCtx, cancel := context.WithTimeout(ctx, time.Second)
		// 计算完成后才提交检查点。若在这两步之间崩溃，接手的 worker 会重跑未提交步骤。
		// 只读手册可安全重读，扣款、发消息等副作用需要在对应系统使用稳定幂等键去重。
		err = store.Commit(commitCtx, run, result)
		cancel()
		if err != nil {
			slog.Warn("checkpoint_rejected", "run_id", run.ID, "epoch", run.LeaseEpoch, "error", err)
			continue
		}
		slog.Info("checkpoint_committed", "run_id", run.ID, "epoch", run.LeaseEpoch, "kind", result.Kind)
	}
}

func wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
