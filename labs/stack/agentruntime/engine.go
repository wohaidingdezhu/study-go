package agentruntime

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

const (
	Queued    = "queued"
	Running   = "running"
	Succeeded = "succeeded"
	Failed    = "failed"
	Cancelled = "cancelled"
)

type Release struct {
	ID           string   `json:"id"`
	Owner        string   `json:"owner"`
	Model        string   `json:"model"`
	AllowedTools []string `json:"allowedTools"`
	MaxSteps     int      `json:"maxSteps"`
}

type ToolCall struct {
	Name  string `json:"name"`
	Query string `json:"query"`
}

type Evidence struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type Observation struct {
	Tool     string     `json:"tool"`
	Evidence []Evidence `json:"evidence"`
}

type Answer struct {
	Text      string   `json:"text"`
	Citations []string `json:"citations"`
}

// Checkpoint 是一次运行的可恢复状态。持久化待执行工具和已完成工具的结果，换 worker 后也能继续。
type Checkpoint struct {
	Pending      *ToolCall     `json:"pending,omitempty"`
	Observations []Observation `json:"observations,omitempty"`
	Answer       *Answer       `json:"answer,omitempty"`
	Error        string        `json:"error,omitempty"`
}

type Decision struct {
	Call   *ToolCall
	Answer *Answer
}

// Model 是执行循环使用的最小接口。worker 从持久化检查点重新构造调用。
type Model interface {
	Next(context.Context, string, Checkpoint) (Decision, error)
}

type Tool func(context.Context, string) ([]Evidence, error)

type Engine struct {
	Release Release
	Model   Model
	Tools   map[string]Tool
}

type Transition struct {
	Kind   string
	Status string
	State  Checkpoint
	Data   any
}

func DefaultEngine() Engine {
	return Engine{
		Release: Release{
			ID: BaselineRelease, Owner: "study-go", Model: "deterministic-v1",
			AllowedTools: []string{"retrieve_runbook"}, MaxSteps: 6,
		},
		Model: RunbookModel{},
		Tools: map[string]Tool{"retrieve_runbook": retrieveRunbook},
	}
}

// Advance 只推进一个步骤，数据库写入由外层 worker 完成，评估也能调用同一个执行器。
func (e Engine) Advance(ctx context.Context, input string, step int, state Checkpoint) Transition {
	// 留出最后一个步骤记录终止原因。
	if step >= e.Release.MaxSteps-1 {
		return failure(state, "run_failed", "step_budget_exceeded", nil)
	}

	if call := state.Pending; call != nil {
		// 注册表表示工具是否存在，发布清单表示该版本是否有权调用它。
		tool, registered := e.Tools[call.Name]
		if !registered || !slices.Contains(e.Release.AllowedTools, call.Name) {
			return failure(state, "tool_denied", "tool_not_allowed", call)
		}
		if strings.TrimSpace(call.Query) == "" || len(call.Query) > 512 {
			return failure(state, "tool_denied", "invalid_tool_input", call)
		}
		evidence, err := tool(ctx, call.Query)
		if err != nil {
			return failure(state, "run_failed", "tool_execution_failed", call)
		}
		observation := Observation{Tool: call.Name, Evidence: evidence}
		state.Pending = nil
		state.Observations = append(state.Observations, observation)
		return Transition{Kind: "tool_completed", Status: Queued, State: state, Data: observation}
	}

	decision, err := e.Model.Next(ctx, input, state)
	if err != nil {
		return failure(state, "run_failed", "model_execution_failed", nil)
	}
	if (decision.Call == nil) == (decision.Answer == nil) {
		return failure(state, "run_failed", "invalid_model_decision", nil)
	}
	if decision.Call != nil {
		// 先记录模型请求。下一个 worker 步骤再执行工具，崩溃后可以看出进度停在哪里。
		state.Pending = decision.Call
		return Transition{Kind: "tool_requested", Status: Queued, State: state, Data: decision.Call}
	}
	if !grounded(decision.Answer, state.Observations) {
		return failure(state, "run_failed", "invalid_evidence_citation", nil)
	}
	state.Answer = decision.Answer
	return Transition{Kind: "run_completed", Status: Succeeded, State: state, Data: decision.Answer}
}

func failure(state Checkpoint, kind, reason string, call *ToolCall) Transition {
	state.Pending = nil
	state.Error = reason
	return Transition{Kind: kind, Status: Failed, State: state, Data: struct {
		Code string    `json:"code"`
		Call *ToolCall `json:"call,omitempty"`
	}{Code: reason, Call: call}}
}

// grounded 核对引用 ID 是否来自本次检索。引用存在性和手册内容的正确性需要各自验证。
func grounded(answer *Answer, observations []Observation) bool {
	if strings.TrimSpace(answer.Text) == "" {
		return false
	}
	available := map[string]bool{}
	for _, observation := range observations {
		for _, evidence := range observation.Evidence {
			available[evidence.ID] = true
		}
	}
	for _, id := range answer.Citations {
		if !available[id] {
			return false
		}
	}
	return len(available) == 0 || len(answer.Citations) > 0
}

// RunbookModel 用固定规则模拟模型决策和结果回填，每次执行都可以离线复现。
type RunbookModel struct{}

func (RunbookModel) Next(ctx context.Context, input string, state Checkpoint) (Decision, error) {
	if err := ctx.Err(); err != nil {
		return Decision{}, err
	}
	if len(state.Observations) == 0 {
		return Decision{Call: &ToolCall{Name: "retrieve_runbook", Query: input}}, nil
	}
	evidence := state.Observations[len(state.Observations)-1].Evidence
	if len(evidence) == 0 {
		return Decision{Answer: &Answer{Text: "运行手册里没有匹配的条目。", Citations: []string{}}}, nil
	}

	var text []string
	citations := make([]string, 0, len(evidence))
	for _, item := range evidence {
		text = append(text, fmt.Sprintf("[%s] %s", item.ID, item.Text))
		citations = append(citations, item.ID)
	}
	return Decision{Answer: &Answer{Text: strings.Join(text, "\n"), Citations: citations}}, nil
}

var runbooks = []struct {
	keywords []string
	evidence Evidence
}{
	{[]string{"redis", "缓存", "回源"}, Evidence{"cache-fallback", "Redis 不可用时先查 /readyz，再检查商品读取日志中的 BYPASS。PostgreSQL 可用时，商品读取会回源。"}},
	{[]string{"postgres", "数据库", "503", "readiness", "readyz"}, Evidence{"database-readiness", "先比较 /livez 和 /readyz。若进程能响应而就绪返回 503，检查 PostgreSQL 连接。恢复后确认就绪返回 200，再查订单。"}},
	{[]string{"rabbitmq", "outbox", "通知", "积压", "队列"}, Evidence{"outbox-backlog", "用订单 ID 对照 outbox 的 published_at。未发布时检查 relay 与 RabbitMQ；已发布时检查队列积压、消费者和通知表。"}},
	{[]string{"重复", "幂等", "重放", "超时重试"}, Evidence{"order-idempotency", "使用相同 Idempotency-Key 和原参数重试下单。核对两个响应中的订单 ID，并检查库存只扣一次。"}},
}

func retrieveRunbook(ctx context.Context, query string) ([]Evidence, error) {
	return searchRunbook(ctx, query, 2)
}

func searchRunbook(ctx context.Context, query string, limit int) ([]Evidence, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	query = strings.ToLower(query)
	type match struct {
		score    int
		evidence Evidence
	}
	var hits []match
	for _, doc := range runbooks {
		score := 0
		for _, keyword := range doc.keywords {
			if strings.Contains(query, keyword) {
				score++
			}
		}
		if score > 0 {
			hits = append(hits, match{score: score, evidence: doc.evidence})
		}
	}
	// 按关键词命中数排序，同分保留手册顺序，让评估的期望可复现。
	slices.SortStableFunc(hits, func(a, b match) int { return b.score - a.score })
	items := make([]Evidence, 0, min(limit, len(hits)))
	for _, hit := range hits[:min(limit, len(hits))] {
		items = append(items, hit.evidence)
	}
	return items, nil
}
