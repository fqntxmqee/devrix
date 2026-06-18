package workmodel

import (
	"strings"
	"testing"
)

func TestDecomposeChildren_DepthLimit(t *testing.T) {
	ResetDecomposeLimits()
	tm := NewTaskManager()
	tree := tm.Tree()
	tree.maxDecomposeDepth = 2
	goal, _ := tm.EnsureGoal("s1", "g")
	l1, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{ParentID: goal.ID, Kind: WorkKindPlan, Title: "l1", Directive: "l1"})
	l2, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{ParentID: l1.ID, Kind: WorkKindImplement, Title: "l2", Directive: "l2"})
	_, err := tm.DecomposeChildren("s1", l2.ID, []ChildSpec{{Title: "too deep", Directive: "x"}})
	if err != ErrDecomposeDepthExceeded {
		t.Fatalf("err = %v, want depth exceeded", err)
	}
}

func TestDecomposeChildren_DailyLimit(t *testing.T) {
	ResetDecomposeLimits()
	tm := NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "g")
	parent, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{ParentID: goal.ID, Kind: WorkKindPlan, Title: "p", Directive: "p"})
	for i := 0; i < 5; i++ {
		_, _ = tm.DecomposeChildren("s1", parent.ID, []ChildSpec{{Title: "c", Directive: "d"}})
	}
	_, err := tm.DecomposeChildren("s1", parent.ID, []ChildSpec{{Title: "one more", Directive: "d"}})
	if err != ErrDecomposeDailyLimit {
		t.Fatalf("err = %v, want daily limit", err)
	}
}

func TestResolveHint_HighUncertainty(t *testing.T) {
	tm := NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "g")
	_ = tm.Tree().SetUncertainty("s1", goal.ID, 0.8)
	goal, _ = tm.GetWorkItem("s1", goal.ID)
	hint := ResolveHint("s1", tm, goal)
	if hint == "" || !strings.Contains(hint, "decompose") {
		t.Fatalf("hint = %q", hint)
	}
}
