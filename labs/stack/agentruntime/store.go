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
	ErrKeyConflict = errors.New("key already belongs to another request")
	ErrLeaseLost   = errors.New("worker lease is no longer valid")
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

const runColumns = "id, release_id, input, status, step, state, lease_epoch, lease_until"

func scanRun(row pgx.Row) (Run, error) {
	var run Run
	err := row.Scan(&run.ID, &run.Release, &run.Input, &run.Status, &run.Step, &run.State,
		&run.LeaseEpoch, &run.LeaseUntil)
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
	if _, err := tx.Exec(ctx, `
		INSERT INTO agent_events (run_id, seq, kind, data)
		VALUES ($1, 1, 'run_created', jsonb_build_object('release', $2::text))`, run.ID, run.Release); err != nil {
		return Run{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Run{}, false, err
	}
	return run, true, nil
}

func (s *Store) Run(ctx context.Context, id int64) (Run, error) {
	return scanRun(s.db.QueryRow(ctx, "SELECT "+runColumns+" FROM agent_runs WHERE id=$1", id))
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
