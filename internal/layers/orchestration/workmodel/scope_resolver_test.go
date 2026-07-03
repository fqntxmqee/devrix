package workmodel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirectiveScopeCandidates_PlanDirectory(t *testing.T) {
	got := DirectiveScopeCandidates("review d7 领域 plan目录下的代码")
	want := "internal/layers/orchestration/plan/"
	found := false
	for _, p := range got {
		if p == want {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("candidates = %v, want prefix %q", got, want)
	}
}

func TestPrepareStrategicScopeIn_FallbackWhenDisjoint(t *testing.T) {
	dir := t.TempDir()
	planDir := filepath.Join(dir, "internal/layers/orchestration/plan")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatal(err)
	}
	directive := "review d7 领域 plan目录下的代码"
	bad := []string{"internal/layers/orchestration/sessionorchestrator/"}
	got, ok, reason := PrepareStrategicScopeIn(directive, bad, dir)
	if ok {
		t.Fatal("expected fallback, not ok")
	}
	if reason == "" {
		t.Fatal("expected reason")
	}
	if len(got) != 1 || got[0] != "internal/layers/orchestration/plan/" {
		t.Fatalf("got scope %v", got)
	}
}

func TestPrepareStrategicScopeIn_AcceptsMatchingScope(t *testing.T) {
	dir := t.TempDir()
	planDir := filepath.Join(dir, "internal/layers/orchestration/plan")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatal(err)
	}
	proposed := []string{"internal/layers/orchestration/plan/"}
	got, ok, reason := PrepareStrategicScopeIn("review d7 plan", proposed, dir)
	if !ok || reason != "" {
		t.Fatalf("ok=%v reason=%q got=%v", ok, reason, got)
	}
	if len(got) != 1 || got[0] != proposed[0] {
		t.Fatalf("got %v", got)
	}
}
