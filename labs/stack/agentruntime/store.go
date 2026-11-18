package agentruntime

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrKeyConflict             = errors.New("key already belongs to another request")
	ErrLeaseLost               = errors.New("worker lease is no longer valid")
	ErrRunFinished             = errors.New("run already reached a final result")
	ErrRunNotRetryable         = errors.New("only failed or cancelled runs can be retried")
	ErrRetryReleaseUnavailable = errors.New("the original release is no longer available")
)

type Run struct {
	ID         int64      `json:"id"`
	Release    string     `json:"release"`
	Input      string     `json:"input"`
	Status     string     `json:"status"`
	Step       int        `json:"step"`
	State      Checkpoint `json:"state"`
	LeaseEpoch int64      `json:"leaseEpoch"`
	LeaseUntil *time.Time `json:"leaseUntil,omitempty"`
	RetryOf    *int64     `json:"retryOf,omitempty,string"`
}

type Event struct {
	Seq       int             `json:"seq"`
	Kind      string          `json:"kind"`
	Data      json.RawMessage `json:"data"`
	CreatedAt time.Time       `json:"createdAt"`
}

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

const runColumns = "id, release_id, input, status, step, state, lease_epoch, lease_until, retry_of"

func scanRun(row pgx.Row) (Run, error) {
	var run Run
	err := row.Scan(&run.ID, &run.Release, &run.Input, &run.Status, &run.Step, &run.State,
		&run.LeaseEpoch, &run.LeaseUntil, &run.RetryOf)
	return run, err
}

func (s *Store) Create(ctx context.Context, key, release, input string) (Run, bool, error) {
	// 幂等键由数据库唯一约束保护，多个 API 实例也只有一个能成功插入。
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Run{}, false, err
	}
	defer tx.Rollback(ctx)

	run, err := scanRun(tx.QueryRow(ctx, `
		INSERT INTO agent_runs (idempotency_key, release_id, input)
		SELECT $1, COALESCE(NULLIF($2, ''), release_id), $3 FROM agent_active_release WHERE singleton
		ON CONFLICT (idempotency_key) DO NOTHING RETURNING `+runColumns, key, release, input))
	if errors.Is(err, pgx.ErrNoRows) {
		// READ COMMITTED 下新语句能读到先胜出的已提交任务，再检查是否确实是同一请求。
		run, err = scanRun(tx.QueryRow(ctx,
			"SELECT "+runColumns+" FROM agent_runs WHERE idempotency_key=$1", key))
		// 省略版本代表新任务使用当时的默认值。重试时优先返回原任务，即使刚切换过版本。
		if err == nil && (run.Input != input || (release != "" && run.Release != release)) {
			return Run{}, false, ErrKeyConflict
		}
		return run, false, err
	}
	if err != nil {
		return Run{}, false, err
	}
	if err := commitNewRun(ctx, tx, run); err != nil {
		return Run{}, false, err
	}
	return run, true, nil
}

// commitNewRun 让新任务与首条事件同事务可见。重试任务的首条事件同时记录来源。
func commitNewRun(ctx context.Context, tx pgx.Tx, run Run) error {
	if _, err := tx.Exec(ctx, `INSERT INTO agent_events (run_id, seq, kind, data)
		VALUES ($1, 1, 'run_created', jsonb_strip_nulls(jsonb_build_object('release', $2::text, 'retryOf', $3::bigint::text)))`,
		run.ID, run.Release, run.RetryOf); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Retry 创建新任务和空检查点，继承原问题及原版本。重发请求只返回已创建的直接重试。
func (s *Store) Retry(ctx context.Context, id int64, releases map[string]bool) (Run, bool, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Run{}, false, err
	}
	defer tx.Rollback(ctx)
	var status, release string
	if err := tx.QueryRow(ctx, "SELECT status, release_id FROM agent_runs WHERE id=$1", id).Scan(&status, &release); err != nil {
		return Run{}, false, err
	}
	if status != Failed && status != Cancelled {
		return Run{}, false, ErrRunNotRetryable
	}

	// 仅创建 worker 能识别的版本。保留前缀避免与用户幂等键冲突。
	available := releases[release]
	run, err := scanRun(tx.QueryRow(ctx, `INSERT INTO agent_runs (idempotency_key, release_id, input, retry_of)
		SELECT 'retry:' || id::text, release_id, input, id FROM agent_runs WHERE id=$1 AND $2::boolean
		ON CONFLICT DO NOTHING RETURNING `+runColumns, id, available))
	if errors.Is(err, pgx.ErrNoRows) {
		// 并发时读胜出的任务。版本下线后，已产生的重试也仍可读取。
		run, err = scanRun(tx.QueryRow(ctx, "SELECT "+runColumns+" FROM agent_runs WHERE retry_of=$1", id))
		if errors.Is(err, pgx.ErrNoRows) && !available {
			return Run{}, false, ErrRetryReleaseUnavailable
		}
		return run, false, err
	}
	if err != nil {
		return Run{}, false, err
	}
	if err := commitNewRun(ctx, tx, run); err != nil {
		return Run{}, false, err
	}
	return run, true, nil
}

func (s *Store) Run(ctx context.Context, id int64) (Run, error) {
	return scanRun(s.db.QueryRow(ctx, "SELECT "+runColumns+" FROM agent_runs WHERE id=$1", id))
}

// Cancel 提交一个终态事件。与 worker 竞争时，行锁决定谁先更新，另一方会按新状态重判条件。
func (s *Store) Cancel(ctx context.Context, id int64) (Run, error) {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Run{}, err
	}
	defer tx.Rollback(ctx)

	// 清除待调用工具和租约，保留此前已提交的证据。
	run, err := scanRun(tx.QueryRow(ctx, `UPDATE agent_runs
		SET status='cancelled', step=step+1, lease_until=NULL, state=state-'pending'
		WHERE id=$1 AND status IN ('queued', 'running') RETURNING `+runColumns, id))
	if errors.Is(err, pgx.ErrNoRows) {
		run, err = scanRun(tx.QueryRow(ctx, "SELECT "+runColumns+" FROM agent_runs WHERE id=$1", id))
		if err == nil && run.Status != Cancelled {
			return Run{}, ErrRunFinished
		}
		return run, err
	}
	if err != nil {
		return Run{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO agent_events (run_id, seq, kind, data)
		VALUES ($1, $2, 'run_cancelled', '{"reason":"user_requested"}')`, run.ID, run.Step+1); err != nil {
		return Run{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Run{}, err
	}
	return run, nil
}

func (s *Store) Events(ctx context.Context, id int64, after int) ([]Event, error) {
	rows, err := s.db.Query(ctx, `
		SELECT seq, kind, data, created_at FROM agent_events
		WHERE run_id=$1 AND seq>$2 ORDER BY seq`, id, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]Event, 0)
	for rows.Next() {
		var event Event
		if err := rows.Scan(&event.Seq, &event.Kind, &event.Data, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *Store) Claim(ctx context.Context, releases []string, lease time.Duration) (Run, error) {
	// 抢占时跳过其他事务正在锁住的行。成功领取后重新计时，以数据库时钟判断租约。
	return scanRun(s.db.QueryRow(ctx, `
		WITH candidate AS (
			SELECT id FROM agent_runs
			WHERE release_id=ANY($1::text[]) AND (status='queued' OR (status='running' AND lease_until<=clock_timestamp()))
			ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED
		)
		UPDATE agent_runs
		SET status='running', lease_epoch=lease_epoch+1,
		    lease_until=clock_timestamp() + $2::bigint * interval '1 millisecond'
		WHERE id=(SELECT id FROM candidate)
		RETURNING `+runColumns, releases, lease.Milliseconds()))
}

func (s *Store) Commit(ctx context.Context, run Run, result Transition) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var step int
	// 恢复后的新 worker 会取得更高的 epoch，旧 worker 无法再提交。
	err = tx.QueryRow(ctx, `
		UPDATE agent_runs
		SET step=step+1, status=$1, state=$2, lease_until=NULL
		WHERE id=$3 AND lease_epoch=$4 AND status='running' AND lease_until>clock_timestamp()
		RETURNING step`, result.Status, result.State, run.ID, run.LeaseEpoch).Scan(&step)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrLeaseLost
	}
	if err != nil {
		return err
	}

	// 状态和事件同事务提交。读取者只看到完整的 checkpoint。
	_, err = tx.Exec(ctx, "INSERT INTO agent_events (run_id, seq, kind, data) VALUES ($1,$2,$3,$4)",
		run.ID, step+1, result.Kind, result.Data)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
