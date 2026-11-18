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
			run := ReplayRun{Release: engine.Release.ID}
			result := runToCompletion(ctx, engine, scenario.Input, func(step Transition) {
				run.Events = append(run.Events, ReplayStep{Seq: len(run.Events) + 1, Kind: step.Kind, Data: step.Data})
			})
			run.Status, run.Answer, run.Error = result.Status, result.State.Answer, result.State.Error
			scenario.Runs = append(scenario.Runs, run)
		}
	}
	return report
}
