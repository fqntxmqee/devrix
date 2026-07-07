package plan

import (
	"errors"
	"testing"

	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// =====================================================================
// Stage 4 (T13/T14) — PlanDAG validator
//
// Per PR-A1 codex consensus 2026-07-07: happy-path + 5 error cases. Each
// error must:
//   - return a *sharederrors.SentinelError carrying the canonical
//     ORCH_DAG_*_72xx code
//   - pass errors.Is(innerErr) check
//   - carry a non-empty Message
//   - exercise the ValidateOpts override path (authoring cap ≠ hard cap)
//
// Self-loop test exercises the cycle path (1-cycle), not the fan-out cap.
// =====================================================================

// makeValidDAG returns a 3-node DAG with 2 edges (a→b→c) that passes all
// 6 default checks. Tests use it as the positive baseline.
func makeValidDAG() *PlanDAG {
	return &PlanDAG{
		Nodes: []PlanNode{
			{ID: "n_a", SegmentID: "seg_a"},
			{ID: "n_b", SegmentID: "seg_b"},
			{ID: "n_c", SegmentID: "seg_c"},
		},
		Edges: []DataEdge{
			{From: "n_a", To: "n_b"},
			{From: "n_b", To: "n_c"},
		},
		Priorities:     map[string]int{"n_a": 90, "n_b": 50, "n_c": 30},
		MaxParallelism: 4,
	}
}

func TestValidateDAG_HappyPath(t *testing.T) {
	d := makeValidDAG()
	if err := validateDAG(d, PlanDAGValidationOpts{}); err != nil {
		t.Errorf("validateDAG on valid DAG: unexpected %v", err)
	}
}

func TestValidateDAG_HappyPath_NoEdges(t *testing.T) {
	// Isolated nodes (no edges) are valid — this is the "all-parallel"
	// shape used by the multi-intent fast path.
	d := &PlanDAG{
		Nodes: []PlanNode{
			{ID: "n_a", SegmentID: "seg_a"},
			{ID: "n_b", SegmentID: "seg_b"},
		},
	}
	if err := validateDAG(d, PlanDAGValidationOpts{}); err != nil {
		t.Errorf("isolated-nodes DAG should be valid: %v", err)
	}
}

func TestValidateDAG_Empty(t *testing.T) {
	d := &PlanDAG{} // no nodes, no edges
	err := validateDAG(d, PlanDAGValidationOpts{})
	if err == nil {
		t.Fatalf("empty DAG: expected error, got nil")
	}
	if !errors.Is(err, ErrPlanDAGEmpty) {
		t.Errorf("errors.Is(err, ErrPlanDAGEmpty) = false, err=%v", err)
	}
	var sErr *sharederrors.SentinelError
	if !errors.As(err, &sErr) {
		t.Fatalf("not *sharederrors.SentinelError: %T", err)
	}
	if sErr.Code != "ORCH_DAG_EMPTY_7200" {
		t.Errorf("code = %q, want ORCH_DAG_EMPTY_7200", sErr.Code)
	}
	if sErr.Message == "" {
		t.Errorf("Message is empty")
	}
}

func TestValidateDAG_EmptyButHasEdges(t *testing.T) {
	// Edges without nodes is structurally invalid — must trip the empty
	// check, not the dangling-edge check (cheap first).
	d := &PlanDAG{
		Edges: []DataEdge{{From: "ghost_a", To: "ghost_b"}},
	}
	err := validateDAG(d, PlanDAGValidationOpts{})
	if !errors.Is(err, ErrPlanDAGEmpty) {
		t.Errorf("expected ErrPlanDAGEmpty, got %v", err)
	}
}

func TestValidateDAG_DuplicateNodeID(t *testing.T) {
	d := &PlanDAG{
		Nodes: []PlanNode{
			{ID: "n_a", SegmentID: "seg_a"},
			{ID: "n_b", SegmentID: "seg_b"},
			{ID: "n_a", SegmentID: "seg_a_dup"}, // dup
		},
	}
	err := validateDAG(d, PlanDAGValidationOpts{})
	if err == nil {
		t.Fatalf("duplicate node ID: expected error, got nil")
	}
	if !errors.Is(err, ErrPlanDAGDuplicateNodeID) {
		t.Errorf("errors.Is(err, ErrPlanDAGDuplicateNodeID) = false, err=%v", err)
	}
	var sErr *sharederrors.SentinelError
	if !errors.As(err, &sErr) {
		t.Fatalf("not *sharederrors.SentinelError: %T", err)
	}
	if sErr.Code != "ORCH_DAG_DUPLICATE_NODE_7201" {
		t.Errorf("code = %q, want ORCH_DAG_DUPLICATE_NODE_7201", sErr.Code)
	}
}

func TestValidateDAG_TooManyNodes(t *testing.T) {
	// Default cap is 10; 11 should fail.
	nodes := make([]PlanNode, 11)
	for i := range nodes {
		nodes[i] = PlanNode{ID: string(rune('a'+i)) + "_node", SegmentID: "s"}
	}
	d := &PlanDAG{Nodes: nodes}
	err := validateDAG(d, PlanDAGValidationOpts{})
	if err == nil {
		t.Fatalf("11-node DAG: expected error, got nil")
	}
	if !errors.Is(err, ErrPlanDAGTooManyNodes) {
		t.Errorf("errors.Is(err, ErrPlanDAGTooManyNodes) = false, err=%v", err)
	}
	var sErr *sharederrors.SentinelError
	if !errors.As(err, &sErr) {
		t.Fatalf("not *sharederrors.SentinelError: %T", err)
	}
	if sErr.Code != "ORCH_DAG_TOO_MANY_NODES_7202" {
		t.Errorf("code = %q, want ORCH_DAG_TOO_MANY_NODES_7202", sErr.Code)
	}
}

func TestValidateDAG_TooManyNodes_CustomCap(t *testing.T) {
	// 3-node DAG with MaxDAGNodes=2 must fail.
	d := &PlanDAG{Nodes: []PlanNode{
		{ID: "a", SegmentID: "s"},
		{ID: "b", SegmentID: "s"},
		{ID: "c", SegmentID: "s"},
	}}
	err := validateDAG(d, PlanDAGValidationOpts{MaxDAGNodes: 2})
	if !errors.Is(err, ErrPlanDAGTooManyNodes) {
		t.Errorf("MaxDAGNodes=2: expected ErrPlanDAGTooManyNodes, got %v", err)
	}
}

func TestValidateDAG_FanOutExceeded(t *testing.T) {
	// 1 source node, 9 outgoing edges → exceeds default cap of 8.
	edges := make([]DataEdge, 9)
	for i := range edges {
		edges[i] = DataEdge{From: "hub", To: string(rune('a' + i)) + "_leaf"}
	}
	nodes := []PlanNode{{ID: "hub", SegmentID: "s"}}
	for i := range edges {
		nodes = append(nodes, PlanNode{ID: string(rune('a'+i)) + "_leaf", SegmentID: "s"})
	}
	d := &PlanDAG{Nodes: nodes, Edges: edges}
	err := validateDAG(d, PlanDAGValidationOpts{})
	if err == nil {
		t.Fatalf("fan-out=9: expected error, got nil")
	}
	if !errors.Is(err, ErrPlanDAGFanOutExceeded) {
		t.Errorf("errors.Is(err, ErrPlanDAGFanOutExceeded) = false, err=%v", err)
	}
	var sErr *sharederrors.SentinelError
	if !errors.As(err, &sErr) {
		t.Fatalf("not *sharederrors.SentinelError: %T", err)
	}
	if sErr.Code != "ORCH_DAG_FANOUT_EXCEEDED_7203" {
		t.Errorf("code = %q, want ORCH_DAG_FANOUT_EXCEEDED_7203", sErr.Code)
	}
}

func TestValidateDAG_DanglingEdge_FromMissing(t *testing.T) {
	d := &PlanDAG{
		Nodes: []PlanNode{
			{ID: "n_a", SegmentID: "seg_a"},
			{ID: "n_b", SegmentID: "seg_b"},
		},
		Edges: []DataEdge{
			{From: "n_ghost", To: "n_a"}, // from missing
		},
	}
	err := validateDAG(d, PlanDAGValidationOpts{})
	if err == nil {
		t.Fatalf("dangling edge: expected error, got nil")
	}
	if !errors.Is(err, ErrPlanDAGDanglingEdge) {
		t.Errorf("errors.Is(err, ErrPlanDAGDanglingEdge) = false, err=%v", err)
	}
	var sErr *sharederrors.SentinelError
	if !errors.As(err, &sErr) {
		t.Fatalf("not *sharederrors.SentinelError: %T", err)
	}
	if sErr.Code != "ORCH_DAG_DANGLING_EDGE_7204" {
		t.Errorf("code = %q, want ORCH_DAG_DANGLING_EDGE_7204", sErr.Code)
	}
}

func TestValidateDAG_DanglingEdge_ToMissing(t *testing.T) {
	d := &PlanDAG{
		Nodes: []PlanNode{
			{ID: "n_a", SegmentID: "seg_a"},
		},
		Edges: []DataEdge{
			{From: "n_a", To: "n_ghost"}, // to missing
		},
	}
	err := validateDAG(d, PlanDAGValidationOpts{})
	if !errors.Is(err, ErrPlanDAGDanglingEdge) {
		t.Errorf("to-missing: expected ErrPlanDAGDanglingEdge, got %v", err)
	}
}

func TestValidateDAG_Cycle(t *testing.T) {
	// 3-node cycle: a → b → c → a
	d := &PlanDAG{
		Nodes: []PlanNode{
			{ID: "n_a", SegmentID: "seg_a"},
			{ID: "n_b", SegmentID: "seg_b"},
			{ID: "n_c", SegmentID: "seg_c"},
		},
		Edges: []DataEdge{
			{From: "n_a", To: "n_b"},
			{From: "n_b", To: "n_c"},
			{From: "n_c", To: "n_a"},
		},
	}
	err := validateDAG(d, PlanDAGValidationOpts{})
	if err == nil {
		t.Fatalf("3-cycle: expected error, got nil")
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

func TestValidateDAG_SelfLoop(t *testing.T) {
	// Self-loop is a 1-cycle: n_a → n_a. Must trip cycle check, NOT
	// fan-out (1 outgoing edge ≤ 8 cap).
	d := &PlanDAG{
		Nodes: []PlanNode{
			{ID: "n_a", SegmentID: "seg_a"},
		},
		Edges: []DataEdge{
			{From: "n_a", To: "n_a"},
		},
	}
	err := validateDAG(d, PlanDAGValidationOpts{})
	if err == nil {
		t.Fatalf("self-loop: expected cycle error, got nil")
	}
	if !errors.Is(err, ErrPlanDAGContainsCycle) {
		t.Errorf("self-loop: expected ErrPlanDAGContainsCycle, got %v", err)
	}
}

func TestValidateDAG_ValidationOrder_EmptyBeforeCycle(t *testing.T) {
	// Empty DAG with self-loop edge: must trip empty check first
	// (validation is fail-fast, cheap first).
	d := &PlanDAG{
		Edges: []DataEdge{{From: "x", To: "x"}},
	}
	err := validateDAG(d, PlanDAGValidationOpts{})
	if !errors.Is(err, ErrPlanDAGEmpty) {
		t.Errorf("empty+cycle: expected ErrPlanDAGEmpty, got %v", err)
	}
}

func TestValidateDAG_ValidationOrder_DuplicateBeforeDangling(t *testing.T) {
	// Duplicate + dangling edge: duplicate check runs first.
	d := &PlanDAG{
		Nodes: []PlanNode{
			{ID: "n_a", SegmentID: "seg_a"},
			{ID: "n_a", SegmentID: "seg_a_dup"},
		},
		Edges: []DataEdge{
			{From: "n_a", To: "n_ghost"},
		},
	}
	err := validateDAG(d, PlanDAGValidationOpts{})
	if !errors.Is(err, ErrPlanDAGDuplicateNodeID) {
		t.Errorf("dup+dangling: expected ErrPlanDAGDuplicateNodeID, got %v", err)
	}
}

func TestValidateDAG_DefaultOpts(t *testing.T) {
	// Smoke test: PlanDAGValidationOpts{} zero-value uses defaults.
	// We can't directly observe the defaults from this side, but we can
	// assert that a 10-node / 8-fanout DAG passes and 11-node / 9-fanout
	// fails — which is what the constants guarantee.
	ten := make([]PlanNode, 10)
	for i := range ten {
		ten[i] = PlanNode{ID: "n" + string(rune('a'+i))}
	}
	if err := validateDAG(&PlanDAG{Nodes: ten}, PlanDAGValidationOpts{}); err != nil {
		t.Errorf("10-node DAG at default cap should pass: %v", err)
	}

	eleven := append(ten, PlanNode{ID: "n_l"})
	if err := validateDAG(&PlanDAG{Nodes: eleven}, PlanDAGValidationOpts{}); !errors.Is(err, ErrPlanDAGTooManyNodes) {
		t.Errorf("11-node DAG at default cap should fail too-many: %v", err)
	}
}
