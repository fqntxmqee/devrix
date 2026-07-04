package workmodel

import "testing"

func TestIsDeliverableFormatRollupSynth_leafWithFindingsContract(t *testing.T) {
	tm := NewTaskManager()
	sessionID := "sess-synth"
	parent, err := tm.EnsureGoal(sessionID, "review plan")
	if err != nil {
		t.Fatal(err)
	}
	child, err := tm.CreateWorkItem(sessionID, CreateWorkItemInput{
		ParentID:  parent.ID,
		Kind:      WorkKindExplore,
		Title:     "slice",
		Directive: "review plan",
	})
	if err != nil {
		t.Fatal(err)
	}
	contract := DeliverableContract{
		Citation:  DeliverableCitationFileLine,
		Severity:  DeliverableSeverityP0P1,
		Structure: DeliverableStructureFindingsJSON,
		Reject:    []DeliverableReject{DeliverableRejectPlanningMeta},
	}
	round := &WorkItemPipelineRound{
		WorkItemID:            child.ID,
		DeliverableContract:   contract,
		DeliverableStatus:     DeliverableStatusIncomplete,
		RollupSynthRequested:  true,
		ArtifactSummary:       "planning prose",
	}
	if err := tm.Tree().ApplyPipelineRound(sessionID, child.ID, round, RoundPhaseIdle); err != nil {
		t.Fatal(err)
	}
	if err := tm.Tree().SetNeedsRollup(sessionID, child.ID, true); err != nil {
		t.Fatal(err)
	}
	got, ok := tm.GetWorkItem(sessionID, child.ID)
	if !ok {
		t.Fatal("child missing")
	}
	if !IsDeliverableFormatRollupSynth(tm, sessionID, got) {
		t.Fatal("expected deliverable format rollup synth")
	}
}

func TestIsDeliverableFormatRollupSynth_falseWhenChildrenExist(t *testing.T) {
	tm := NewTaskManager()
	sessionID := "sess-parent-rollup"
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
	grand, err := tm.CreateWorkItem(sessionID, CreateWorkItemInput{
		ParentID: child.ID, Kind: WorkKindExplore, Title: "sub", Directive: "review sub",
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = grand
	if err := tm.Tree().SetNeedsRollup(sessionID, child.ID, true); err != nil {
		t.Fatal(err)
	}
	got, ok := tm.GetWorkItem(sessionID, child.ID)
	if !ok {
		t.Fatal("child missing")
	}
	if IsDeliverableFormatRollupSynth(tm, sessionID, got) {
		t.Fatal("parent rollup with children should not be deliverable synth")
	}
}
