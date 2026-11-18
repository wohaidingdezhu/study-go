package agentruntime

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
)

const BaselineRelease = "ops-runbook-v1"

//go:embed runtime_schema.sql
var runtimeSchema string

// Catalog 保留旧版本的定义，运行中的任务可以按已保存的版本继续执行。
func Catalog() []Engine {
	baseline := DefaultEngine()
	candidate := DefaultEngine()
	candidate.Release.ID = "ops-runbook-v2"
	candidate.Tools["retrieve_runbook"] = func(ctx context.Context, query string) ([]Evidence, error) {
		return searchRunbook(ctx, query, 3)
	}
	regression := DefaultEngine()
	regression.Release.ID = "ops-runbook-regression"
	regression.Release.AllowedTools = nil
	return []Engine{baseline, candidate, regression}
}

// Init 为新旧数据库应用增量变更。事务级锁避免 API 与 worker 并发修改表结构。
func (s *Store) Init(ctx context.Context) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(2841601)`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, runtimeSchema); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO agent_active_release (singleton, release_id)
		VALUES (true, $1) ON CONFLICT DO NOTHING`, BaselineRelease); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) ActiveRelease(ctx context.Context) (string, error) {
	var id string
	err := s.db.QueryRow(ctx, `SELECT release_id FROM agent_active_release WHERE singleton`).Scan(&id)
	return id, err
}

var ErrReleaseGate = errors.New("candidate did not pass the release gate")

// Publish 在同一事务内切换默认版本并写入评估快照。expected 用于拒绝已经过期的发布操作。
func (s *Store) Publish(ctx context.Context, expected string, report EvalReport) error {
	if !report.Passed {
		return ErrReleaseGate
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var active string
	if err := tx.QueryRow(ctx, `SELECT release_id FROM agent_active_release WHERE singleton FOR UPDATE`).Scan(&active); err != nil {
		return err
	}
	if active != expected {
		return fmt.Errorf("active release is %s, expected %s", active, expected)
	}
	if active == report.Release {
		return tx.Commit(ctx)
	}
	if _, err := tx.Exec(ctx, `UPDATE agent_active_release SET release_id = $1 WHERE singleton`, report.Release); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO agent_release_changes (previous_release, next_release, evaluation)
		VALUES ($1, $2, $3)`, active, report.Release, report); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
