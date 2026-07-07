package plan

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	ifaces "github.com/devrix/devrix/internal/layers/orchestration/interfaces"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// =====================================================================
// Stage 3 (T03) — Plan 字段扩展:IntentSegmentSet + DAG
//
// PR-A1 codex consensus 2026-07-07:
//   - 加 2 字段 ≠ 改 Plan.Validate()(boundary note 已在 struct doc 说明)
//   - 用 immutable builder pattern(返回新副本)
//   - 22-test 回归必须 100% PASS
//   - coverage ≥ 80%
// =====================================================================

func makeValidPlan(t *testing.T) Plan {
	t.Helper()
	p := NewPlan(
		"plan_test_id",
		"sess_test",
		CommitmentPlan,
		[]string{"obs_1"},
		[]Step{{ID: "s1", Directive: "noop"}},
		0.5,
	)
	p2 := p.WithFailureCriteria([]FailureCriterion{
		{Field: "exit_code", Op: "eq", Value: "0"},
	})
	p3 := p2.WithBlastRadius(BlastRadius{
		FileCount: 0, APICallCount: 0, TokenCost: 0,
		PersistScope: PersistTransient,
	})
	return p3
}

func TestPlan_WithIntentSegmentSet_ImmutableCopy(t *testing.T) {
	p := makeValidPlan(t)
	if p.IntentSegmentSet != nil {
		t.Fatalf("baseline Plan.IntentSegmentSet = %+v, want nil", p.IntentSegmentSet)
	}
	segSet := &ifaces.IntentSegmentSet{
		SourceDirective: "1+1=几? 巴黎时区?",
		DetectedAt:      time.Now(),
		Segments: []ifaces.IntentSegment{
			ifaces.NewIntentSegment("seg_a", "1+1=几?", ifaces.IntentSegmentKindDeterministic),
		},
	}
	p2 := p.WithIntentSegmentSet(segSet)
	if p.IntentSegmentSet != nil {
		t.Errorf("immutability: original Plan.IntentSegmentSet changed to non-nil")
	}
	if p2.IntentSegmentSet == nil {
		t.Fatalf("copy should have IntentSegmentSet")
	}
	if p2.IntentSegmentSet.SourceDirective != "1+1=几? 巴黎时区?" {
		t.Errorf("SourceDirective = %q", p2.IntentSegmentSet.SourceDirective)
	}
	// mutate the input; copy must not see it
	segSet.SourceDirective = "MUTATED"
	if p2.IntentSegmentSet.SourceDirective == "MUTATED" {
		t.Errorf("builder did not deep-copy")
	}
}

func TestPlan_WithIntentSegmentSet_NilClears(t *testing.T) {
	p := makeValidPlan(t).WithIntentSegmentSet(&ifaces.IntentSegmentSet{
		Segments: []ifaces.IntentSegment{
			ifaces.NewIntentSegment("seg_a", "x", ifaces.IntentSegmentKindExplore),
		},
	})
	if p.IntentSegmentSet == nil {
		t.Fatalf("precondition failed")
	}
	cleared := p.WithIntentSegmentSet(nil)
	if cleared.IntentSegmentSet != nil {
		t.Errorf("WithIntentSegmentSet(nil) did not clear, got %+v", cleared.IntentSegmentSet)
	}
}

func TestPlan_WithDAG_ImmutableCopy(t *testing.T) {
	p := makeValidPlan(t)
	if p.DAG != nil {
		t.Fatalf("baseline Plan.DAG = %+v, want nil", p.DAG)
	}
	dag := &PlanDAG{
		Nodes: []PlanNode{
			{ID: "n_a", SegmentID: "seg_a", WorkerHint: "explorer"},
		},
		Priorities: map[string]int{"n_a": 50},
	}
	p2 := p.WithDAG(dag)
	if p.DAG != nil {
		t.Errorf("immutability: original Plan.DAG changed to non-nil")
	}
	if p2.DAG == nil {
		t.Fatalf("copy should have DAG")
	}
	if len(p2.DAG.Nodes) != 1 || p2.DAG.Nodes[0].ID != "n_a" {
		t.Errorf("DAG.Nodes mismatch: %+v", p2.DAG.Nodes)
	}
	// mutate input map
	dag.Priorities["n_a"] = 999
	if p2.DAG.Priorities["n_a"] == 999 {
		t.Errorf("builder did not deep-copy Priorities")
	}
}

func TestPlan_WithDAG_NilClears(t *testing.T) {
	p := makeValidPlan(t).WithDAG(&PlanDAG{Nodes: []PlanNode{{ID: "n_a", SegmentID: "s1"}}})
	if p.DAG == nil {
		t.Fatalf("precondition failed")
	}
	cleared := p.WithDAG(nil)
	if cleared.DAG != nil {
		t.Errorf("WithDAG(nil) did not clear, got %+v", cleared.DAG)
	}
}

func TestPlan_Validate_StillPassesAfterNewFields(t *testing.T) {
	// 回归测试:加 2 字段后,Plan.Validate() 必须仍能通过 (PP-1/2/3 不读新字段)
	p := makeValidPlan(t)
	if err := p.Validate(); err != nil {
		t.Errorf("Validate() failed on baseline Plan: %v", err)
	}
	p2 := p.WithIntentSegmentSet(&ifaces.IntentSegmentSet{
		SourceDirective: "x",
		DetectedAt:      time.Now(),
		Segments: []ifaces.IntentSegment{
			ifaces.NewIntentSegment("seg_a", "x", ifaces.IntentSegmentKindExplore),
		},
	})
	if err := p2.Validate(); err != nil {
		t.Errorf("Validate() failed on Plan with IntentSegmentSet: %v", err)
	}
	p3 := p2.WithDAG(&PlanDAG{
		Nodes: []PlanNode{{ID: "n_a", SegmentID: "seg_a"}},
	})
	if err := p3.Validate(); err != nil {
		t.Errorf("Validate() failed on Plan with IntentSegmentSet+DAG: %v", err)
	}
}

func TestPlan_JSON_RoundTrip_DAG(t *testing.T) {
	p := makeValidPlan(t).
		WithIntentSegmentSet(&ifaces.IntentSegmentSet{
			SourceDirective: "dir ctx",
			DetectedAt:      time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC),
			Segments: []ifaces.IntentSegment{
				ifaces.NewIntentSegment("seg_a", "1+1=几?", ifaces.IntentSegmentKindDeterministic),
				ifaces.NewIntentSegment("seg_b", "巴黎时区?", ifaces.IntentSegmentKindDeterministic),
			},
		}).
		WithDAG(&PlanDAG{
			Nodes: []PlanNode{
				{ID: "n_a", SegmentID: "seg_a", WorkerHint: "fast_path"},
				{ID: "n_b", SegmentID: "seg_b", WorkerHint: "fast_path"},
			},
			Priorities: map[string]int{"n_a": 90, "n_b": 50},
		})
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out Plan
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.IntentSegmentSet == nil {
		t.Fatalf("IntentSegmentSet lost in round-trip")
	}
	if out.DAG == nil {
		t.Fatalf("DAG lost in round-trip")
	}
	if len(out.DAG.Nodes) != 2 {
		t.Errorf("DAG.Nodes len = %d", len(out.DAG.Nodes))
	}
	if out.DAG.Priorities["n_a"] != 90 {
		t.Errorf("Priorities[n_a] = %d, want 90", out.DAG.Priorities["n_a"])
	}
}

// =====================================================================
// Stage 4 (T13) — Plan.Validate() integrates validateDAG
//
// When Plan.DAG is non-nil, Plan.Validate() must invoke validateDAG and
// surface the 5 DAG errors with their ORCH_DAG_*_72xx codes. nil DAG is
// the PR-B1 4-channel path and must remain a valid state.
// =====================================================================

func TestPlan_Validate_NilDAG_StillValid(t *testing.T) {
	// nil DAG is the PR-B1 4-channel path; must not trip Plan.Validate().
	p := makeValidPlan(t)
	if err := p.Validate(); err != nil {
		t.Errorf("Validate with nil DAG should pass, got %v", err)
	}
}

func TestPlan_Validate_ValidDAG_Passes(t *testing.T) {
	p := makeValidPlan(t).WithDAG(&PlanDAG{
		Nodes: []PlanNode{
			{ID: "n_a", SegmentID: "seg_a"},
			{ID: "n_b", SegmentID: "seg_b"},
		},
	})
	if err := p.Validate(); err != nil {
		t.Errorf("Validate with valid DAG should pass, got %v", err)
	}
}

func TestPlan_Validate_DAGCycle_PropagatesError(t *testing.T) {
	p := makeValidPlan(t).WithDAG(&PlanDAG{
		Nodes: []PlanNode{
			{ID: "n_a", SegmentID: "seg_a"},
			{ID: "n_b", SegmentID: "seg_b"},
		},
		Edges: []DataEdge{
			{From: "n_a", To: "n_b"},
			{From: "n_b", To: "n_a"}, // 2-cycle
		},
	})
	err := p.Validate()
	if err == nil {
		t.Fatalf("Plan.Validate with cyclic DAG: expected error, got nil")
	}
	if !errors.Is(err, ErrPlanDAGContainsCycle) {
		t.Errorf("errors.Is(err, ErrPlanDAGContainsCycle) = false, err=%v", err)
	}
	var sErr *sharederrors.SentinelError
	if !errors.As(err, &sErr) {
		t.Fatalf("not *sharederrors.SentinelError: %T", err)
	}
	if sErr.Code != "ORCH_DAG_CYCLE_7205" {
		t.Errorf("code = %q, want ORCH_DAG_CYCLE_7205", sErr.Code)
	}
}

func TestPlan_Validate_DAGDanglingEdge_PropagatesError(t *testing.T) {
	p := makeValidPlan(t).WithDAG(&PlanDAG{
		Nodes: []PlanNode{
			{ID: "n_a", SegmentID: "seg_a"},
		},
		Edges: []DataEdge{
			{From: "n_a", To: "n_ghost"},
		},
	})
	err := p.Validate()
	if !errors.Is(err, ErrPlanDAGDanglingEdge) {
		t.Errorf("dangling edge: expected ErrPlanDAGDanglingEdge, got %v", err)
	}
}

func TestPlan_Validate_DAGDuplicateNode_PropagatesError(t *testing.T) {
	p := makeValidPlan(t).WithDAG(&PlanDAG{
		Nodes: []PlanNode{
			{ID: "n_a", SegmentID: "seg_a"},
			{ID: "n_a", SegmentID: "seg_a_dup"},
		},
	})
	err := p.Validate()
	if !errors.Is(err, ErrPlanDAGDuplicateNodeID) {
		t.Errorf("duplicate node: expected ErrPlanDAGDuplicateNodeID, got %v", err)
	}
}

func TestPlan_Validate_DAGTooManyNodes_PropagatesError(t *testing.T) {
	nodes := make([]PlanNode, 11)
	for i := range nodes {
		nodes[i] = PlanNode{ID: "n" + string(rune('a'+i))}
	}
	p := makeValidPlan(t).WithDAG(&PlanDAG{Nodes: nodes})
	err := p.Validate()
	if !errors.Is(err, ErrPlanDAGTooManyNodes) {
		t.Errorf("too-many-nodes: expected ErrPlanDAGTooManyNodes, got %v", err)
	}
}

func TestPlan_Validate_DAGEmpty_PropagatesError(t *testing.T) {
	// An empty DAG (no nodes) is structurally invalid. This guards against
	// a future author writing `&PlanDAG{}` and silently passing Validate.
	p := makeValidPlan(t).WithDAG(&PlanDAG{})
	err := p.Validate()
	if !errors.Is(err, ErrPlanDAGEmpty) {
		t.Errorf("empty DAG: expected ErrPlanDAGEmpty, got %v", err)
	}
}

func TestPlan_Validate_DAGCustomCap(t *testing.T) {
	// 3-node DAG with MaxDAGNodes=2 must fail; default cap (10) would pass.
	p := makeValidPlan(t).WithDAG(&PlanDAG{
		Nodes: []PlanNode{
			{ID: "a", SegmentID: "s"},
			{ID: "b", SegmentID: "s"},
			{ID: "c", SegmentID: "s"},
		},
	})
	if err := p.Validate(); err != nil {
		t.Fatalf("default cap (10): should pass, got %v", err)
	}
	if err := p.ValidateWithOpts(ValidateOpts{MaxDAGNodes: 2}); !errors.Is(err, ErrPlanDAGTooManyNodes) {
		t.Errorf("MaxDAGNodes=2: expected ErrPlanDAGTooManyNodes, got %v", err)
	}
}
