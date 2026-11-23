package agentruntime

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// 任务状态描述一次运行的生命周期。queued 可被领取，running 表示已有 worker 领取。
// succeeded、failed、cancelled 是终态，分别表示正常结束、执行失败和取消完成。
const (
	Queued    = "queued"
	Running   = "running"
	Succeeded = "succeeded"
	Failed    = "failed"
	Cancelled = "cancelled"
)

// Release 保存运行版本及其执行限制。任务创建后绑定版本，恢复时按相同配置继续。
// MaxSteps 限制状态推进次数，AllowedTools 列出这个版本可以调用的工具。
type Release struct {
	ID           string   `json:"id"`
	Owner        string   `json:"owner"`
	Model        string   `json:"model"`
	AllowedTools []string `json:"allowedTools"`
	MaxSteps     int      `json:"maxSteps"`
}

// ToolCall 是待执行工具的名称与参数，类似一次结构化的函数调用请求。
// 本例只用 Query 表达参数。执行器还需检查工具存在、权限与参数有效性，随后才执行。
type ToolCall struct {
	Name  string `json:"name"`
	Query string `json:"query"`
}

// Evidence 是工具找到的一条来源资料。ID 用于引用核对，Text 是提供给模型的内容。
type Evidence struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// Observation 保存一次工具执行的结果，供下一轮模型决策读取，这一步称为结果回填。
type Observation struct {
	Tool     string     `json:"tool"`
	Evidence []Evidence `json:"evidence"`
}

type Answer struct {
	Text      string   `json:"text"`
	Citations []string `json:"citations"`
}

// Checkpoint（检查点）保存一次运行推进到了哪里，持久化后换 worker 也能继续。
// Pending 是待执行工具，Observations 是已取得的结果，Answer 和 Error 记录终止结果。
// *ToolCall 是 Go 指针，nil 表示当前没有待执行工具，Java 常用可空引用表达类似状态。
// json 标签规定 JSON 字段名，omitempty 让可选字段在零值时省略。
type Checkpoint struct {
	Pending      *ToolCall     `json:"pending,omitempty"`
	Observations []Observation `json:"observations,omitempty"`
	Answer       *Answer       `json:"answer,omitempty"`
	Error        string        `json:"error,omitempty"`
}

// Decision 表示模型提出的下一步。当前契约要求 Call 与 Answer 恰好有一项有值。
type Decision struct {
	Call   *ToolCall
	Answer *Answer
}

// Model 是执行循环使用的最小接口。Go 的类型实现 Next 方法即可满足这个接口。
// 可对照 Java 的 interface 与 implements。本例由 RunbookModel 实现，真实模型可另写适配器。
// Next 读取问题与检查点，返回下一步决策或 error。ctx 传递调用的取消信号与截止时间。
type Model interface {
	Next(context.Context, string, Checkpoint) (Decision, error)
}

// Tool 是函数类型，约定每个工具的输入输出，便于按名称放进同一个注册表。
// []Evidence 是 Go 切片，可类比 Java 的 List<Evidence>，error 交给调用方检查并处理。
type Tool func(context.Context, string) ([]Evidence, error)

// Engine 负责把模型决策变成受控执行，包含权限、参数、步骤预算和回答引用检查。
// Tools 是 map，将工具名映射到实现函数，承担“工具注册表”的作用。
type Engine struct {
	Release Release
	Model   Model
	Tools   map[string]Tool
}

// Transition 是一次状态推进的结果。Kind 用于记录事件，State 用于保存下一份检查点。
// 一个任务会产生多个事件，可以顺着事件序号还原它先请求了什么、再获得了什么。
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
		// append 将结果追加到上下文，下一次 Next 才能基于这次工具结果继续决定。
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

// grounded 做引用来源核对，确认回答引用的 ID 来自本次工具结果。
// 资料关联到回答的过程常称为 grounding。这里实现的是引用存在性检查，内容支持度需另行评估。
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
	// context 的取消需要被调用代码主动检查或传给支持它的库，才能及时结束工作。
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
	// 检索先给每条手册按关键词命中数打分，再排序并取前 limit 条，通常称为 top-k。
	// 本例用于学习“检索资料再组织回答”的过程，匹配依据是关键词，计数单位是手册条目。
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
