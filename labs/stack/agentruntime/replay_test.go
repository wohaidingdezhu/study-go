package agentruntime

import (
	"context"
	"slices"
	"testing"
)

func TestReplayCapturesReleaseDifferences(t *testing.T) {
	report := BuildReplay(context.Background())
	incident := report.Scenarios[0]
	v1, v2, regression := incident.Runs[0], incident.Runs[1], incident.Runs[2]
	for _, comparison := range []struct {
		run  ReplayRun
		want []string
	}{
		{v1, []string{"cache-fallback", "database-readiness"}},
		{v2, []string{"cache-fallback", "database-readiness", "outbox-backlog"}},
	} {
		if comparison.run.Status != Succeeded || comparison.run.Answer == nil || !slices.Equal(comparison.run.Answer.Citations, comparison.want) {
			t.Fatalf("unexpected result for %s: %+v", comparison.run.Release, comparison.run)
		}
	}
	// 拒绝发生在读取证据前，回放不应包含工具结果或最终答复。
	if regression.Status != Failed || regression.Answer != nil || len(regression.Events) != 2 ||
		regression.Events[1].Kind != "tool_denied" || report.Evaluations[2].Passed {
		t.Fatalf("regression unexpectedly executed: %+v", regression)
	}
	unmatched := report.Scenarios[1].Runs[0]
	if unmatched.Status != Succeeded || unmatched.Answer == nil || len(unmatched.Answer.Citations) != 0 {
		t.Fatalf("unmatched input unexpectedly has citations: %+v", unmatched)
	}
}
