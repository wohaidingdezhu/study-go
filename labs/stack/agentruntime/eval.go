package agentruntime

import (
	"context"
	"slices"
)

// EvalCase 记录一个评估样本的通过情况，便于定位是哪类输入或约束退化了。
type EvalCase struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Result string `json:"result"`
}

// EvalReport 汇总固定评估集的结果，Dataset 标识样本集，Release 标识被评估的运行版本。
// 固定输入与预期行为，让两个版本的结果可以在同一标准下比较。
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
		// 此处按引用列表做精确断言，适用于确定性手册样例。
		// 接真实模型后，还需评估回答是否得到资料支持、任务是否完成，以及耗时与使用量。
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
