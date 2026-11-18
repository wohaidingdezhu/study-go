package agentruntime

import (
	"context"
	"time"
)

type QueueOverview struct {
	Queued         int64      `json:"queued"`
	Running        int64      `json:"running"`
	ExpiredLeases  int64      `json:"expiredLeases"`
	OldestQueuedAt *time.Time `json:"oldestQueuedAt"`
}

type RecentOverview struct {
	Hours           int      `json:"hours"`
	Succeeded       int64    `json:"succeeded"`
	Failed          int64    `json:"failed"`
	Cancelled       int64    `json:"cancelled"`
	DurationSamples int64    `json:"durationSamples"`
	P50MS           *float64 `json:"p50Ms"`
	P95MS           *float64 `json:"p95Ms"`
}

type Overview struct {
	AsOf   time.Time      `json:"asOf"`
	Queue  QueueOverview  `json:"queue"`
	Recent RecentOverview `json:"recent"`
}

func (s *Store) overview(ctx context.Context) (Overview, error) {
	const hours = 24
	result := Overview{Recent: RecentOverview{Hours: hours}}
	// 队列与终态事件在同一条 SQL 中读取，共用数据库的快照与时钟。
	err := s.db.QueryRow(ctx, `
		WITH snapshot AS (SELECT statement_timestamp() AS as_of),
		queue AS (
			SELECT count(*) FILTER (WHERE r.status = 'queued') AS queued,
			       count(*) FILTER (WHERE r.status = 'running') AS running,
			       count(*) FILTER (WHERE r.status = 'running' AND r.lease_until <= snapshot.as_of) AS expired,
			       min(created.created_at) FILTER (WHERE r.status = 'queued') AS oldest
			FROM agent_runs r
			JOIN agent_events created ON created.run_id = r.id AND created.seq = 1
			CROSS JOIN snapshot
			WHERE r.status IN ('queued', 'running')
		), recent AS (
			SELECT terminal.kind,
			       (EXTRACT(EPOCH FROM terminal.created_at - created.created_at) * 1000)::double precision AS elapsed_ms
			FROM agent_events terminal
			JOIN agent_events created ON created.run_id = terminal.run_id AND created.seq = 1
			CROSS JOIN snapshot
			WHERE terminal.kind IN ('run_completed', 'run_failed', 'tool_denied', 'run_cancelled')
			  AND terminal.created_at > snapshot.as_of - $1::int * interval '1 hour'
			  AND terminal.created_at <= snapshot.as_of
		), outcomes AS (
			SELECT count(*) FILTER (WHERE kind = 'run_completed') AS succeeded,
			       count(*) FILTER (WHERE kind IN ('run_failed', 'tool_denied')) AS failed,
			       count(*) FILTER (WHERE kind = 'run_cancelled') AS cancelled,
			       count(*) FILTER (WHERE kind = 'run_completed' AND elapsed_ms >= 0) AS samples,
			       percentile_cont(ARRAY[0.50, 0.95]) WITHIN GROUP (ORDER BY elapsed_ms)
			           FILTER (WHERE kind = 'run_completed' AND elapsed_ms >= 0) AS latency
			FROM recent
		)
		SELECT snapshot.as_of, queue.queued, queue.running, queue.expired, queue.oldest,
		       outcomes.succeeded, outcomes.failed, outcomes.cancelled, outcomes.samples,
		       outcomes.latency[1], outcomes.latency[2]
		FROM snapshot CROSS JOIN queue CROSS JOIN outcomes`, hours).Scan(
		&result.AsOf, &result.Queue.Queued, &result.Queue.Running, &result.Queue.ExpiredLeases, &result.Queue.OldestQueuedAt,
		&result.Recent.Succeeded, &result.Recent.Failed, &result.Recent.Cancelled, &result.Recent.DurationSamples,
		&result.Recent.P50MS, &result.Recent.P95MS)
	return result, err
}
