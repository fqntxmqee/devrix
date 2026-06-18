package workmodel

import "testing"

func TestComputeUncertainty_EmptyEvidenceFallback(t *testing.T) {
	item := NewWorkItem(WorkKindImplement, "t", "d")
	u := ComputeUncertainty(item, ChildOutcomeStats{}, 0, 0)
	if u <= 0 || u > 1 {
		t.Fatalf("expected fallback uncertainty in (0,1], got %v", u)
	}
}

func TestComputeUncertainty_WithLLMClaim(t *testing.T) {
	item := NewWorkItem(WorkKindPlan, "plan", "plan it")
	stats := ChildOutcomeStats{Total: 2, Failed: 1, Running: 1}
	u := ComputeUncertainty(item, stats, 0.8, 3)
	if u < 0.3 {
		t.Fatalf("expected higher uncertainty with failures, got %v", u)
	}
}

func TestAdaptiveThreshold_ColdStart(t *testing.T) {
	a := &AdaptiveThreshold{GlobalDefault: 0.55}
	if got := a.ThresholdFor("unknown"); got != 0.55 {
		t.Fatalf("cold start threshold = %v, want 0.55", got)
	}
}

func TestReevaluateParentAfterChild_PartialFailure(t *testing.T) {
	tm := NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "goal")
	parent, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{ParentID: goal.ID, Kind: WorkKindPlan, Title: "p", Directive: "p"})
	okChild, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{ParentID: parent.ID, Kind: WorkKindImplement, Title: "ok", Directive: "ok"})
	failChild, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{ParentID: parent.ID, Kind: WorkKindImplement, Title: "bad", Directive: "bad"})
	_ = tm.UpdateStatus("s1", okChild.ID, TaskStatusInProgress)
	_ = tm.UpdateStatus("s1", okChild.ID, TaskStatusCompleted)
	_ = tm.UpdateStatus("s1", failChild.ID, TaskStatusInProgress)
	_ = tm.UpdateStatus("s1", failChild.ID, TaskStatusFailed)
	ReevaluateParentAfterChild("s1", failChild.ID, tm)
	p, _ := tm.GetWorkItem("s1", parent.ID)
	if p.Status != TaskStatusFailed {
		t.Fatalf("parent status = %s, want failed", p.Status)
	}
}
