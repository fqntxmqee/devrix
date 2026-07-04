package workmodel

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

func TestTryPromoteSingleChildDeliverable_promotesStructuredFindings(t *testing.T) {
	tm := NewTaskManager()
	sessionID := "sess-promote"
	goal, err := tm.EnsureGoal(sessionID, "review plan")
	if err != nil {
		t.Fatal(err)
	}
	child, err := tm.CreateWorkItem(sessionID, CreateWorkItemInput{
		ParentID: goal.ID, Kind: WorkKindExplore, Title: "slice", Directive: "review",
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = tm.Tree().ApplyPipelineRound(sessionID, goal.ID, &WorkItemPipelineRound{
		WorkItemID: goal.ID, VerdictKind: types.VerdictPartial, SpawnPolicy: SpawnDecompose,
	}, RoundPhaseExecute)
	childRound := &WorkItemPipelineRound{
		WorkItemID:        child.ID,
		DeliverableStatus: DeliverableStatusComplete,
		DeliverableContract: DeliverableContract{
			Citation: DeliverableCitationFileLine, Severity: DeliverableSeverityP0P1,
			Structure: DeliverableStructureFindingsJSON,
		},
		StructuredDeliverable: &DeliverablePayload{
			Findings: []DeliverableFinding{{Severity: "P1", Title: "aliasing bug", File: "plan.go"}},
		},
	}
	if err := tm.Tree().ApplyPipelineRound(sessionID, child.ID, childRound, RoundPhaseIdle); err != nil {
		t.Fatal(err)
	}
	_ = tm.Tree().UpdateStatus(sessionID, child.ID, TaskStatusInProgress)
	_ = tm.Tree().UpdateStatus(sessionID, child.ID, TaskStatusCompleted)

	if !TryPromoteSingleChildDeliverable(sessionID, tm, goal) {
		t.Fatal("expected promote")
	}
	got, ok := tm.GetWorkItem(sessionID, goal.ID)
	if !ok || got.Status != TaskStatusCompleted || got.NeedsRollup {
		t.Fatalf("parent = %+v", got)
	}
	if got.LastRound == nil || got.LastRound.ExitReason != ExitReasonChildPromoted {
		t.Fatalf("round = %+v", got.LastRound)
	}
	if formatted := ExtractSessionDeliverable(tm, sessionID); !containsAll(formatted, "aliasing bug", "plan.go") {
		t.Fatalf("deliverable = %q", formatted)
	}
}

func TestTryPromoteSingleChildDeliverable_falseWithMultipleChildren(t *testing.T) {
	tm := NewTaskManager()
	sessionID := "sess-multi"
	goal, err := tm.EnsureGoal(sessionID, "review")
	if err != nil {
		t.Fatal(err)
	}
	_ = tm.Tree().ApplyPipelineRound(sessionID, goal.ID, &WorkItemPipelineRound{
		SpawnPolicy: SpawnDecompose,
	}, RoundPhaseExecute)
	for i := 0; i < 2; i++ {
		child, err := tm.CreateWorkItem(sessionID, CreateWorkItemInput{
			ParentID: goal.ID, Kind: WorkKindExplore, Title: "slice", Directive: "review",
		})
		if err != nil {
			t.Fatal(err)
		}
		_ = tm.Tree().ApplyPipelineRound(sessionID, child.ID, &WorkItemPipelineRound{
			DeliverableStatus: DeliverableStatusComplete,
			StructuredDeliverable: &DeliverablePayload{
				Findings: []DeliverableFinding{{Severity: "P1", Title: "x", File: "a.go"}},
			},
		}, RoundPhaseIdle)
		_ = tm.Tree().UpdateStatus(sessionID, child.ID, TaskStatusInProgress)
		_ = tm.Tree().UpdateStatus(sessionID, child.ID, TaskStatusCompleted)
	}
	if TryPromoteSingleChildDeliverable(sessionID, tm, goal) {
		t.Fatal("expected no promote with two children")
	}
}

func TestReevaluateParentAfterChild_promotesInsteadOfNeedsRollup(t *testing.T) {
	tm := NewTaskManager()
	sessionID := "sess-reeval-promote"
	goal, err := tm.EnsureGoal(sessionID, "review")
	if err != nil {
		t.Fatal(err)
	}
	child, err := tm.CreateWorkItem(sessionID, CreateWorkItemInput{
		ParentID: goal.ID, Kind: WorkKindExplore, Title: "slice", Directive: "review",
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = tm.Tree().ApplyPipelineRound(sessionID, goal.ID, &WorkItemPipelineRound{
		WorkItemID: goal.ID, VerdictKind: types.VerdictPartial, SpawnPolicy: SpawnDecompose,
	}, RoundPhaseExecute)
	_ = tm.Tree().ApplyPipelineRound(sessionID, child.ID, &WorkItemPipelineRound{
		WorkItemID: child.ID, DeliverableStatus: DeliverableStatusComplete,
		StructuredDeliverable: &DeliverablePayload{
			Findings: []DeliverableFinding{{Severity: "P0", Title: "race", File: "x.go"}},
		},
	}, RoundPhaseIdle)
	_ = tm.Tree().UpdateStatus(sessionID, child.ID, TaskStatusInProgress)
	_ = tm.Tree().UpdateStatus(sessionID, child.ID, TaskStatusCompleted)

	ReevaluateParentAfterChild(sessionID, child.ID, tm)
	got, ok := tm.GetWorkItem(sessionID, goal.ID)
	if !ok || got.NeedsRollup || got.Status != TaskStatusCompleted {
		t.Fatalf("parent after reevaluate = %+v", got)
	}
	if !tm.Tree().HasOpenWork(sessionID) {
		return
	}
	t.Fatal("expected no open work after promote")
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if p != "" && !strings.Contains(s, p) {
			return false
		}
	}
	return true
}
