package workmodel

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

// aliases keep test bodies short and signal that the test is just
// wiring the canonical types.VerdictKind values into WorkItemPipelineRound.
var (
	types_VerdictPass    = types.VerdictPass
	types_VerdictPartial = types.VerdictPartial
)

// T: D7-S16-A94-T16..T18 (DM-20260701-001 T-P2-1, T-P2-2 scope contract)
//
// ValidateChildScopes enforces:
//   1. Each child's scope_in is a (non-strict) subset of parent's scope_in.
//   2. Each child's scope_in is non-empty (when parent has scope).
//   3. Coverage hint: children union should cover parent's scope.
//
// OK=true means no hard violations (Missing/NotSubset); coverage gaps
// surface as UncoveredPaths for the prompt guidance.
func TestValidateChildScopes(t *testing.T) {
	parent := &WorkItem{ID: "p", ScopeContract: &ScopeContract{
		InScope: []string{"a/", "b/", "c/"},
	}}
	cases := []struct {
		name           string
		children       []*WorkItem
		wantOK         bool
		wantUncovered  []string
		wantViolations int
	}{
		{
			name: "all children proper subset",
			children: []*WorkItem{
				{ID: "c1", ScopeContract: &ScopeContract{InScope: []string{"a/"}}},
				{ID: "c2", ScopeContract: &ScopeContract{InScope: []string{"b/", "c/"}}},
			},
			wantOK:         true,
			wantViolations: 0,
		},
		{
			name: "child missing scope",
			children: []*WorkItem{
				{ID: "c1", ScopeContract: &ScopeContract{InScope: []string{"a/"}}},
				{ID: "c2", ScopeContract: nil}, // missing
			},
			wantOK:         false,
			wantViolations: 1,
		},
		{
			name: "child scope outside parent",
			children: []*WorkItem{
				{ID: "c1", ScopeContract: &ScopeContract{InScope: []string{"a/", "x/"}}},
			},
			wantOK:         false,
			wantViolations: 1,
		},
		{
			name: "under-coverage: no child covers c/",
			children: []*WorkItem{
				{ID: "c1", ScopeContract: &ScopeContract{InScope: []string{"a/", "b/"}}},
			},
			wantOK:         true, // no hard violation, but uncovered
			wantUncovered:  []string{"c/"},
			wantViolations: 0,
		},
		{
			name: "parent unbounded + child unbounded",
			children: []*WorkItem{
				{ID: "c1"}, // no scope at all
			},
			wantOK:         false,
			wantViolations: 1, // parent-unbounded + child-empty = Missing
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := ValidateChildScopes(parent, c.children)
			if res.OK != c.wantOK {
				t.Errorf("OK = %v, want %v (violations=%+v)", res.OK, c.wantOK, res.Violations)
			}
			if len(res.Violations) != c.wantViolations {
				t.Errorf("violations = %d, want %d (%+v)", len(res.Violations), c.wantViolations, res.Violations)
			}
			if len(c.wantUncovered) > 0 {
				if len(res.UncoveredPaths) == 0 {
					t.Errorf("expected uncovered paths, got none")
				} else if res.UncoveredPaths[0] != c.wantUncovered[0] {
					t.Errorf("first uncovered = %q, want %q", res.UncoveredPaths[0], c.wantUncovered[0])
				}
			}
		})
	}
}

// T: D7-S16-A94-T16 (DM-20260701-001 T-P2-1 DefaultChildDownlink fix)
//
// The "blind inherit" bug: when a child spec has no ScopeIn but the
// parent has a bounded scope, the previous DefaultChildDownlink
// silently inherited the parent scope. The new contract: the spec
// scope is authoritative; the parent scope is only inherited when
// the parent has NO ScopeContract (i.e. parent is unbounded).
func TestDefaultChildDownlink_NoBlindInherit(t *testing.T) {
	parentBounded := &WorkItem{
		ID: "p",
		ScopeContract: &ScopeContract{
			InScope: []string{"a/", "b/", "c/"},
		},
	}
	parentUnbounded := &WorkItem{ID: "p"} // no ScopeContract

	t.Run("bounded parent + empty spec scope = empty child scope", func(t *testing.T) {
		child := &WorkItem{ID: "c"}
		spec := ChildSpec{Directive: "review a", ExpectedReturn: "x"}
		dl := DefaultChildDownlink(parentBounded, child, spec)
		if len(dl.ScopeIn) != 0 {
			t.Errorf("bounded parent + empty spec must produce empty ScopeIn, got %v", dl.ScopeIn)
		}
	})

	t.Run("bounded parent + non-empty spec scope = spec scope", func(t *testing.T) {
		child := &WorkItem{ID: "c"}
		spec := ChildSpec{
			Directive:      "review a",
			ExpectedReturn: "x",
			ScopeIn:        []string{"a/"},
		}
		dl := DefaultChildDownlink(parentBounded, child, spec)
		if len(dl.ScopeIn) != 1 || dl.ScopeIn[0] != "a/" {
			t.Errorf("spec scope must pass through, got %v", dl.ScopeIn)
		}
	})

	t.Run("unbounded parent + empty spec scope = empty child scope", func(t *testing.T) {
		child := &WorkItem{ID: "c"}
		spec := ChildSpec{Directive: "review a", ExpectedReturn: "x"}
		dl := DefaultChildDownlink(parentUnbounded, child, spec)
		if len(dl.ScopeIn) != 0 {
			t.Errorf("unbounded parent + empty spec must produce empty ScopeIn, got %v", dl.ScopeIn)
		}
	})
}

// T: D7-S8-A95-T22 (DM-20260701-001 T-P2-3 ChildUncertaintyBubble)
//
// CollectChildUncertaintyBubbles emits one entry per terminal child
// with UncertaintyMean ≥ ChildUncertaintyBubbleThreshold. The
// statement format mirrors StructuredBubbleStatement so downstream
// consumers apply a single parser.
func TestCollectChildUncertaintyBubbles(t *testing.T) {
	tm := NewTaskManager()
	parent, _ := tm.EnsureGoal("s1", "review")
	low, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{ParentID: parent.ID, Kind: WorkKindImplement, Title: "low"})
	high, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{ParentID: parent.ID, Kind: WorkKindImplement, Title: "high"})
	_ = tm.Tree().ApplyPipelineRound("s1", low.ID, &WorkItemPipelineRound{
		WorkItemID: low.ID, VerdictKind: types_VerdictPass, UncertaintyMean: 0.1,
	}, RoundPhaseIdle)
	_ = tm.UpdateStatus("s1", low.ID, TaskStatusInProgress)
	_ = tm.UpdateStatus("s1", low.ID, TaskStatusCompleted)
	_ = tm.Tree().ApplyPipelineRound("s1", high.ID, &WorkItemPipelineRound{
		WorkItemID: high.ID, VerdictKind: types_VerdictPartial, UncertaintyMean: 0.8, ExitReason: "incomplete",
	}, RoundPhaseIdle)
	_ = tm.UpdateStatus("s1", high.ID, TaskStatusInProgress)
	_ = tm.UpdateStatus("s1", high.ID, TaskStatusCompleted)

	bubbles := CollectChildUncertaintyBubbles(tm, "s1", parent.ID)
	if len(bubbles) != 1 {
		t.Fatalf("bubbles = %d, want 1 (only high-u)", len(bubbles))
	}
	if bubbles[0].ChildID != high.ID {
		t.Errorf("ChildID = %q, want %q", bubbles[0].ChildID, high.ID)
	}
	if bubbles[0].Uncertainty != 0.8 {
		t.Errorf("Uncertainty = %v, want 0.8", bubbles[0].Uncertainty)
	}
	stmt := ChildUncertaintyBubbleStatement(bubbles[0])
	if !strings.Contains(stmt, "child_uncertainty_bubble:") {
		t.Errorf("statement must start with marker, got: %q", stmt)
	}
	for _, want := range []string{high.ID, "uncertainty=0.800", "verdict=partial", "reason="} {
		if !strings.Contains(stmt, want) {
			t.Errorf("statement missing %q, got: %q", want, stmt)
		}
	}
}
