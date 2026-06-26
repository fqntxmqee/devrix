package workmodel

import (
	"testing"
)

func TestCLIReviewApprove_HumanReviewItem(t *testing.T) {
	tm := NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "risky")
	round := &WorkItemPipelineRound{
		WorkItemID:     goal.ID,
		SpawnPolicy:    SpawnEscalateHuman,
		PlanID:         "p1",
		VerdictID:      "v1",
		ObservationIDs: []string{"o1"},
		SpawnRationale: "limit",
	}
	if err := ApplySpawnPolicy("s1", goal, round, tm); err != nil {
		t.Fatalf("ApplySpawnPolicy: %v", err)
	}
	children := tm.Tree().ListChildren("s1", goal.ID)
	if len(children) != 1 {
		t.Fatalf("children = %d", len(children))
	}
	reviewID := children[0].ID

	cli := NewCLICommands(tm)
	out := cli.Handle(&Command{Name: "review", Args: []string{"approve", reviewID}}, "s1")
	if out == "" || out[0] == 'E' {
		t.Fatalf("review approve output = %q", out)
	}

	got, _ := tm.GetWorkItem("s1", reviewID)
	if got.Status != TaskStatusCompleted {
		t.Fatalf("review status = %q, want completed", got.Status)
	}
}

func TestIsHumanReviewItem(t *testing.T) {
	if IsHumanReviewItem(&WorkItem{Kind: WorkKindVerify, Title: HumanReviewItemTitle}) != true {
		t.Fatal("expected human review item")
	}
	if IsHumanReviewItem(&WorkItem{Kind: WorkKindVerify, Title: "other"}) {
		t.Fatal("unexpected human review")
	}
}
