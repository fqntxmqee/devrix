package workmodel

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestApplyContextBubbleDecision_DefaultStructured(t *testing.T) {
	round := &WorkItemPipelineRound{WorkItemID: "c1"}
	child := &WorkItem{ID: "c1", ParentID: "p1"}
	parent := &WorkItem{ID: "p1"}
	dec := ApplyContextBubbleDecision(round, &ContextBubbleSpec{Kind: BubbleNone}, DefaultContextBubbleEvalContext(child, parent, round, nil, ""))
	if round.ContextBubbleKind != BubbleStructured {
		t.Fatalf("round kind=%q, dec=%+v", round.ContextBubbleKind, dec)
	}
}

func TestCollectStructuredChildBubbles(t *testing.T) {
	tm := NewTaskManager()
	parent, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{Kind: WorkKindPlan, Title: "parent"})
	child, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{
		ParentID: parent.ID, Kind: WorkKindImplement, Title: "child",
	})
	_ = tm.Tree().ApplyPipelineRound("s1", child.ID, &WorkItemPipelineRound{
		WorkItemID:        child.ID,
		VerdictID:         "v1",
		PlanID:            "p1",
		VerdictKind:       types.VerdictPass,
		ContextBubbleKind: BubbleStructured,
	}, RoundPhaseIdle)
	_ = tm.UpdateStatus("s1", child.ID, TaskStatusInProgress)
	_ = tm.UpdateStatus("s1", child.ID, TaskStatusCompleted)

	bubbles := CollectStructuredChildBubbles(tm, "s1", parent.ID)
	if len(bubbles) != 1 || bubbles[0].ChildID != child.ID {
		t.Fatalf("bubbles=%+v", bubbles)
	}
	stmt := StructuredBubbleStatement(child.ID, bubbles[0].Round)
	if stmt == "" || !containsSubstring(stmt, "structured_child_bubble") {
		t.Fatalf("stmt=%q", stmt)
	}
}

func TestWaveContextPolicyForItem_NoBlockedByFresh(t *testing.T) {
	item := &WorkItem{}
	if got := WaveContextPolicyForItem(item); got != wavescheduler.ContextFresh {
		t.Fatalf("got %q", got)
	}
}

func TestWaveContextPolicyForItem_BlockedByUpstream(t *testing.T) {
	item := &WorkItem{BlockedBy: []string{"up"}}
	if got := WaveContextPolicyForItem(item); got != wavescheduler.ContextUpstream {
		t.Fatalf("got %q", got)
	}
}

func TestProjectWaveTaskNode_UpstreamID(t *testing.T) {
	item := &WorkItem{
		ID: "dep", Title: "t", Directive: "d", BlockedBy: []string{"blocker"},
	}
	node := ProjectWaveTaskNode(item)
	if node.ContextPolicy != wavescheduler.ContextUpstream || node.UpstreamTaskID != "blocker" {
		t.Fatalf("node=%+v", node)
	}
}

func TestCollectChecklistChildBubbles(t *testing.T) {
	tm := NewTaskManager()
	parent, _ := tm.EnsureGoal("s1", "review d2")
	_ = tm.Tree().UpsertChecklist("s1", parent.ID, []ChecklistEntry{
		{Content: "Review prepare/ pipeline", Status: TaskStatusPending},
	})
	bubbles := CollectChecklistChildBubbles(tm, "s1", parent.ID)
	if len(bubbles) != 1 {
		t.Fatalf("bubbles=%d, want 1", len(bubbles))
	}
}

func TestChecklistBubbleStatement_CB3Truncate(t *testing.T) {
	long := strings.Repeat("z", 3000)
	item := &WorkItem{Directive: long, Status: TaskStatusPending}
	stmt := ChecklistBubbleStatement("cl1", item)
	if !strings.Contains(stmt, "checklist_child_bubble") {
		t.Fatalf("stmt=%q", stmt)
	}
	// directive preview inside quotes should be ≤2048 runes + ellipsis
	idx := strings.Index(stmt, "directive=")
	if idx < 0 {
		t.Fatalf("missing directive in %q", stmt)
	}
	quoted := stmt[idx+len("directive="):]
	quoted = strings.Trim(quoted, "\"")
	if utf8.RuneCountInString(quoted) > DefaultShareSummaryMaxTokens+1 {
		t.Fatalf("directive preview too long: %d runes", utf8.RuneCountInString(quoted))
	}
}

func TestSummaryBubbleStatement_CB3Truncate(t *testing.T) {
	long := strings.Repeat("s", 3000)
	stmt := SummaryBubbleStatement("c1", long)
	if stmt == "" {
		t.Fatal("expected summary bubble statement")
	}
	idx := strings.Index(stmt, "preview=")
	if idx < 0 {
		t.Fatalf("stmt=%q", stmt)
	}
	preview := strings.Trim(stmt[idx+len("preview="):], "\"")
	if utf8.RuneCountInString(preview) > DefaultShareSummaryMaxTokens+1 {
		t.Fatalf("preview too long: %d runes", utf8.RuneCountInString(preview))
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexSubstring(s, sub) >= 0)
}

func indexSubstring(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
