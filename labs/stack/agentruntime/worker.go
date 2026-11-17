package agentruntime

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

// 租约让崩溃后遗留的运行中任务重新可领取。每次领取只负责一个步骤。
const leaseDuration = 6 * time.Second

func Work(ctx context.Context, store *Store, engines map[string]Engine, checkpointDelay time.Duration) {
	releases := make([]string, 0, len(engines))
	for release := range engines {
		releases = append(releases, release)
	}
	for ctx.Err() == nil {
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
