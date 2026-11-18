package agentruntime

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

type RunSummary struct {
	// 游标和列表 ID 用十进制字符串，避免 JavaScript Number 舍入 bigint。
	ID        int64     `json:"id,string"`
	Input     string    `json:"input"`
	Release   string    `json:"release"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	RetryOf   *int64    `json:"retryOf,omitempty,string"`
}

type RunPage struct {
	Items      []RunSummary `json:"items"`
	NextCursor *int64       `json:"nextCursor,string"`
}

type historyQuery struct {
	limit  int
	before int64
	status string
}

func parseHistoryQuery(rawQuery string) (historyQuery, bool) {
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return historyQuery{}, false
	}
	query := historyQuery{limit: 10}
	for key, entries := range values {
		if len(entries) != 1 {
			return historyQuery{}, false
		}
		value := entries[0]
		switch key {
		case "limit", "before":
			number, err := strconv.ParseUint(value, 10, 63)
			if err != nil || number == 0 || (key == "limit" && number > 50) {
				return historyQuery{}, false
			}
			if key == "limit" {
				query.limit = int(number)
			} else {
				query.before = int64(number)
			}
		case "status":
			switch value {
			case Queued, Running, Succeeded, Failed, Cancelled:
				query.status = value
			default:
				return historyQuery{}, false
			}
		default:
			return historyQuery{}, false
		}
	}
	return query, true
}

func (s *Store) listRuns(ctx context.Context, query historyQuery) (RunPage, error) {
	// 创建任务时第一条事件同事务写入。使用它的时间，不需要为旧任务补字段。
	statement := `SELECT r.id, r.input, r.release_id, r.status, e.created_at, r.retry_of
		FROM agent_runs r JOIN agent_events e ON e.run_id = r.id AND e.seq = 1
		WHERE true`
	args := []any{query.limit + 1}
	if query.before > 0 {
		args = append(args, query.before)
		statement += fmt.Sprintf(" AND r.id < $%d", len(args))
	}
	if query.status != "" {
		args = append(args, query.status)
		statement += fmt.Sprintf(" AND r.status = $%d", len(args))
	}
	// 只有固定 SQL 和占位符进入语句，筛选值作为参数发送。多读一行确定是否还能继续。
	rows, err := s.db.Query(ctx, statement+" ORDER BY r.id DESC LIMIT $1", args...)
	if err != nil {
		return RunPage{}, err
	}
	defer rows.Close()

	page := RunPage{Items: make([]RunSummary, 0, query.limit)}
	for rows.Next() {
		var item RunSummary
		if err := rows.Scan(&item.ID, &item.Input, &item.Release, &item.Status, &item.CreatedAt, &item.RetryOf); err != nil {
			return RunPage{}, err
		}
		page.Items = append(page.Items, item)
	}
	if len(page.Items) > query.limit {
		page.Items = page.Items[:query.limit]
		cursor := page.Items[query.limit-1].ID
		page.NextCursor = &cursor
	}
	return page, rows.Err()
}
