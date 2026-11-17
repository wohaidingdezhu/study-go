package agentruntime

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
)

func TestRunbookResumesFromPersistedToolRequest(t *testing.T) {
	engine := DefaultEngine()
	ctx := context.Background()
	input := "PostgreSQL 503 应该怎么查？"
	first := engine.Advance(ctx, input, 0, Checkpoint{})
	if first.Kind != "tool_requested" || first.State.Pending == nil {
		t.Fatalf("unexpected first transition: %+v", first)
	}

	// JSON 往返模拟退出再启动，从 PostgreSQL 只读回检查点继续推进。
	data, err := json.Marshal(first.State)
	if err != nil {
		t.Fatal(err)
	}
	var recovered Checkpoint
	if err := json.Unmarshal(data, &recovered); err != nil {
		t.Fatal(err)
	}
	second := DefaultEngine().Advance(ctx, input, 1, recovered)
	third := DefaultEngine().Advance(ctx, input, 2, second.State)
	if second.Kind != "tool_completed" || third.Status != Succeeded ||
		third.State.Answer == nil || !slices.Equal(third.State.Answer.Citations, []string{"database-readiness"}) {
		t.Fatalf("resumption failed: %+v %+v", second, third)
	}
}

func TestReleaseGateRejectsPolicyRegression(t *testing.T) {
	engine := DefaultEngine()
	if report := Evaluate(context.Background(), engine); !report.Passed {
		t.Fatalf("expected the shipped release to pass: %+v", report)
	}
	// 故意放开测试工具，验证发布评估能发现白名单退化。
	engine.Release.AllowedTools = append(engine.Release.AllowedTools, "write_system")
	if report := Evaluate(context.Background(), engine); report.Passed {
		t.Fatalf("policy regression was allowed: %+v", report)
	}
}
