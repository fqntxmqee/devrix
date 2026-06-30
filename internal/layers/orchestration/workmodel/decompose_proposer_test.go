package workmodel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/shared/types"
)

// noTacticalHardcodingSources lists every Go source file in the
// orchestration layer that authors fallback proposers / directive
// builders. RH-D7-13 (DM-20260630-013 T-P2-11.4) extends the original
// decompose-only scan to all of these so a tactical hardcoding slipping
// into a sibling proposer still trips CI. The .cursor rule
// `orchestration-no-tactical-hardcoding.mdc` covers the same surface
// conceptually; this test is the executable regression.
//
// Paths are relative to the repo root. The repo-root-relative form
// keeps the test stable across `go test ./...` runs from any cwd
// because Go tests run with cwd == the package directory; we resolve
// from cwd upward looking for go.mod.
func noTacticalHardcodingSources(t *testing.T) []string {
	t.Helper()
	root, err := findRepoRoot()
	if err != nil {
		t.Fatalf("find repo root: %v", err)
	}
	return []string{
		filepath.Join(root, "internal/layers/orchestration/workmodel/decompose_proposer.go"),
		filepath.Join(root, "internal/layers/orchestration/workmodel/context_proposer.go"),
		filepath.Join(root, "internal/layers/orchestration/sessionorchestrator/observation_proposer.go"),
		filepath.Join(root, "internal/layers/orchestration/sessionorchestrator/llm_observation_proposer.go"),
		filepath.Join(root, "internal/layers/orchestration/sessionorchestrator/rollup_directive.go"),
		filepath.Join(root, "internal/layers/orchestration/sessionorchestrator/strategic_plan_proposer.go"),
	}
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 16; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", os.ErrNotExist
}

// Regression: orchestrator fallback paths must stay structural — no
// tactical NL sneaks into Go source. Original test scanned only
// decompose_proposer.go; RH-D7-13 (T-P2-11.4) extends to every
// proposer/directive file in workmodel/ + sessionorchestrator/.
func TestDefaultDecomposeProposer_NoTacticalHardcoding(t *testing.T) {
	for _, path := range noTacticalHardcodingSources(t) {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		src := string(raw)
		for _, forbidden := range []string{
			"聚焦",
			"只 read_file",
			"禁止探索",
			"P0/P1 清单",
			"contracts and API surface",
			"implementation and observability",
			"hypothesis",
		} {
			if strings.Contains(src, forbidden) {
				t.Fatalf("%s contains forbidden tactical string %q", path, forbidden)
			}
		}
	}
}

func TestDefaultDecomposeProposer_NoHypothesisLabels(t *testing.T) {
	item := &WorkItem{
		Kind:      WorkKindGoal,
		Directive: "review d2 kernel code",
	}
	round := &WorkItemPipelineRound{PlanKind: plan.ExplorationPlan}
	specs := DefaultDecomposeProposer(item, round)
	if len(specs) != 1 {
		t.Fatalf("specs = %d, want 1 pass-through without scope paths", len(specs))
	}
	if specs[0].Directive != item.Directive {
		t.Fatalf("directive = %q, want pass-through %q", specs[0].Directive, item.Directive)
	}
	if specs[0].ExpectedReturn == "" {
		t.Fatal("expected_return required")
	}
	if !strings.Contains(specs[0].ExpectedReturn, "deliverable_schema") {
		t.Fatalf("expected machine schema tag, got %q", specs[0].ExpectedReturn)
	}
}

func TestDefaultDecomposeProposer_SplitsScopePaths(t *testing.T) {
	item := &WorkItem{
		Kind:      WorkKindGoal,
		Directive: "review kernel",
		ScopeContract: &ScopeContract{
			InScope: []string{
				"internal/layers/contextengine/kernel/contracts.go",
				"internal/layers/contextengine/kernel/spans.go",
				"internal/layers/contextengine/kernel/observer_test.go",
			},
		},
	}
	round := &WorkItemPipelineRound{PlanKind: plan.ExplorationPlan}
	specs := DefaultDecomposeProposer(item, round)
	if len(specs) != 2 {
		t.Fatalf("specs = %d, want 2 scope slices", len(specs))
	}
	for _, s := range specs {
		if len(s.ScopeIn) == 0 {
			t.Fatal("expected scope_in on child spec")
		}
		if s.Directive != item.Directive {
			t.Fatalf("directive mutated tactically: %q", s.Directive)
		}
	}
}

func TestHasOpenWork_PendingRollupOnGoal(t *testing.T) {
	tm := NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "g")
	_ = tm.Tree().UpdateStatus("s1", goal.ID, TaskStatusCompleted)
	_ = tm.Tree().SetNeedsRollup("s1", goal.ID, true)
	if !tm.Tree().HasOpenWork("s1") {
		t.Fatal("expected open work while root needs_rollup")
	}
}

func TestSpawnPolicy_RollupFailInlines(t *testing.T) {
	round := &WorkItemPipelineRound{
		VerdictKind: types.VerdictFail,
		PlanKind:    plan.CommitmentPlan,
	}
	ctx := TreeEvalContext{RollupRound: true}
	if got := SpawnPolicyEvaluator(round, ctx); got != SpawnInline {
		t.Fatalf("got %q, want inline", got)
	}
}
