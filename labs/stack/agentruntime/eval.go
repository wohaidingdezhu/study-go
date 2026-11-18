package agentruntime

import (
	"context"
	"slices"
)

type EvalCase struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Result string `json:"result"`
}

type EvalReport struct {
	Release string     `json:"release"`
	Dataset string     `json:"dataset"`
	Passed  bool       `json:"passed"`
	Cases   []EvalCase `json:"cases"`
}

// Evaluate 使用执行器的真实代码跑固定样本。API 和 worker 启动时都检查这份报告。
func Evaluate(ctx context.Context, engine Engine) EvalReport {
	report := EvalReport{Release: engine.Release.ID, Dataset: "runbook-smoke-v1", Passed: true}
	record := func(name string, ok bool, result string) {
		report.Cases = append(report.Cases, EvalCase{Name: name, Passed: ok, Result: result})
		report.Passed = report.Passed && ok
	}
	for _, sample := range []struct {
		name     string
		input    string
		expected []string
	}{
		{"redis_evidence", "Redis 下线后商品查询怎么办？", []string{"cache-fallback"}},
		{"queue_evidence", "RabbitMQ 通知积压，怎样看 outbox？", []string{"outbox-backlog"}},
		{"no_matching_evidence", "如何烤面包？", []string{}},
	} {
		result := runToCompletion(ctx, engine, sample.input, nil)
		record(sample.name, result.Status == Succeeded && result.State.Answer != nil &&
			slices.Equal(result.State.Answer.Citations, sample.expected), result.Kind)
	}

	// 注册一个测试用工具，但不把它放进发布清单，确认权限判断发生在执行前。
	invoked := false
	blocked := engine
	blocked.Tools = map[string]Tool{"write_system": func(context.Context, string) ([]Evidence, error) {
		invoked = true
		return nil, nil
	}}
	result := blocked.Advance(ctx, "probe", 1, Checkpoint{
		Pending: &ToolCall{Name: "write_system", Query: "probe"},
	})
	record("tool_allowlist", result.Kind == "tool_denied" && result.Status == Failed && !invoked, result.Kind)
	return report
}

// runToCompletion 为离线评估和回放提供同一条运行路径，worker 仍逐步持久化 Advance 的结果。
func runToCompletion(ctx context.Context, engine Engine, input string, observe func(Transition)) Transition {
	result := Transition{}
	for step := 0; step < engine.Release.MaxSteps; step++ {
		result = engine.Advance(ctx, input, step, result.State)
		if observe != nil {
			observe(result)
		}
		if result.Status != Queued {
			break
		}
	}
	return result
}
