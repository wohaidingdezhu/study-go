package agentruntime

import "context"

type ReplayReport struct {
	Evaluations []EvalReport     `json:"evaluations"`
	Scenarios   []ReplayScenario `json:"scenarios"`
}

type ReplayScenario struct {
	ID    string      `json:"id"`
	Title string      `json:"title"`
	Input string      `json:"input"`
	Runs  []ReplayRun `json:"runs"`
}

type ReplayRun struct {
	Release string       `json:"release"`
	Status  string       `json:"status"`
	Answer  *Answer      `json:"answer,omitempty"`
	Error   string       `json:"error,omitempty"`
	Events  []ReplayStep `json:"events"`
}

type ReplayStep struct {
	Seq  int    `json:"seq"`
	Kind string `json:"kind"`
	Data any    `json:"data"`
}

// Trace 使用与 worker 相同的执行器，在内存中记录一次自选问题的工具请求、结果和答复。
// 模型仍是确定性适配器，便于先学习控制循环，再接真实模型。
// 传给 runToCompletion 的匿名函数是回调，每次推进结束都会收到一个 Transition 供记录。
func Trace(ctx context.Context, engine Engine, input string) ReplayRun {
	run := ReplayRun{Release: engine.Release.ID}
	result := runToCompletion(ctx, engine, input, func(step Transition) {
		run.Events = append(run.Events, ReplayStep{Seq: len(run.Events) + 1, Kind: step.Kind, Data: step.Data})
	})
	run.Status, run.Answer, run.Error = result.Status, result.State.Answer, result.State.Error
	return run
}

// BuildReplay 使用内置样本和代码生成公开回放。报告只包含离线执行器直接产生的步骤。
func BuildReplay(ctx context.Context) ReplayReport {
	engines := Catalog()
	report := ReplayReport{Scenarios: []ReplayScenario{
		{ID: "incident", Title: "跨组件故障", Input: "Redis、PostgreSQL 和 RabbitMQ 如何排查？"},
		{ID: "unmatched", Title: "手册无匹配", Input: "如何烤面包？"},
	}}
	for _, engine := range engines {
		report.Evaluations = append(report.Evaluations, Evaluate(ctx, engine))
	}
	for i := range report.Scenarios {
		scenario := &report.Scenarios[i]
		for _, engine := range engines {
			scenario.Runs = append(scenario.Runs, Trace(ctx, engine, scenario.Input))
		}
	}
	return report
}
