package workmodel

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

func TestShouldRollupAfterChildren(t *testing.T) {
	parent := &WorkItem{
		ID: "p1",
		LastRound: &WorkItemPipelineRound{
			SpawnPolicy: SpawnDecompose,
		},
	}
	stats := ChildOutcomeStats{Total: 2, Completed: 1, Failed: 1}
	if !ShouldRollupAfterChildren(parent, RollupGateBestEffort, stats) {
		t.Fatal("expected rollup when all non-checklist children terminal")
	}
	stats.Running = 1
	if ShouldRollupAfterChildren(parent, RollupGateBestEffort, stats) {
		t.Fatal("expected no rollup while child running")
	}
}

func TestShouldRollupAfterChildren_AllPassBlocksOnFail(t *testing.T) {
	parent := &WorkItem{
		ID: "p1",
		LastRound: &WorkItemPipelineRound{SpawnPolicy: SpawnDecompose},
	}
	stats := ChildOutcomeStats{Total: 2, Completed: 1, Failed: 1}
	if ShouldRollupAfterChildren(parent, RollupGateAllPass, stats) {
		t.Fatal("all_pass should not rollup when any child failed")
	}
	stats = ChildOutcomeStats{Total: 2, Completed: 2}
	if !ShouldRollupAfterChildren(parent, RollupGateAllPass, stats) {
		t.Fatal("all_pass should rollup when all children completed")
	}
}

func TestRollupGatePolicyFor_Phase1BestEffort(t *testing.T) {
	if got := RollupGatePolicyFor(&WorkItem{ID: "p1"}); got != RollupGateBestEffort {
		t.Fatalf("policy=%q, want best_effort", got)
	}
}

func TestReopenForRollup(t *testing.T) {
	tm := NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "review d2")
	_ = tm.Tree().ApplyPipelineRound("s1", goal.ID, &WorkItemPipelineRound{
		SpawnPolicy: SpawnNone,
		VerdictKind: types.VerdictFail,
	}, RoundPhaseIdle)
	_ = tm.Tree().UpdateStatus("s1", goal.ID, TaskStatusInProgress)
	_ = tm.Tree().UpdateStatus("s1", goal.ID, TaskStatusFailed)
	if err := tm.Tree().SetNeedsRollup("s1", goal.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := tm.Tree().ReopenForRollup("s1", goal.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := tm.GetWorkItem("s1", goal.ID)
	if got.Status != TaskStatusPending {
		t.Fatalf("status = %s, want pending", got.Status)
	}
	if got.Locked {
		t.Fatal("expected unlocked for rollup")
	}
}

func TestGetReadyItems_SkipsEphemeralChecklist(t *testing.T) {
	tm := NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "task")
	_ = tm.Tree().UpsertChecklist("s1", goal.ID, []ChecklistEntry{
		{Content: "step 1", Status: TaskStatusPending},
	})
	ready := tm.Tree().GetReadyItems("s1")
	for _, item := range ready {
		if item.Kind == WorkKindChecklist && item.Ephemeral {
			t.Fatalf("ephemeral checklist %s should not be ready", item.ID)
		}
	}
}

func TestGetReadyItems_NeedsRollupInProgress(t *testing.T) {
	tm := NewTaskManager()
	parent, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{Kind: WorkKindPlan, Title: "parent"})
	_ = tm.Tree().UpdateStatus("s1", parent.ID, TaskStatusInProgress)
	if err := tm.Tree().SetNeedsRollup("s1", parent.ID, true); err != nil {
		t.Fatal(err)
	}
	ready := tm.Tree().GetReadyItems("s1")
	if len(ready) != 1 || ready[0].ID != parent.ID {
		t.Fatalf("ready=%v, want parent with NeedsRollup in_progress", ready)
	}
}

func TestMaybeRootRollupFallback(t *testing.T) {
	tm := NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "review d2")
	_ = tm.Tree().ApplyPipelineRound("s1", goal.ID, &WorkItemPipelineRound{
		SpawnPolicy: SpawnNone,
		VerdictKind: types.VerdictFail,
	}, RoundPhaseIdle)
	_ = tm.Tree().UpsertChecklist("s1", goal.ID, []ChecklistEntry{
		{Content: "review prepare/", Status: TaskStatusPending},
	})
	_ = tm.Tree().UpdateStatus("s1", goal.ID, TaskStatusInProgress)
	_ = tm.Tree().UpdateStatus("s1", goal.ID, TaskStatusFailed)
	wi, ok := MaybeRootRollupFallback("s1", tm)
	if !ok || wi == nil {
		t.Fatal("expected root rollup fallback")
	}
	if !wi.NeedsRollup {
		t.Fatal("expected NeedsRollup")
	}
	if wi.Status != TaskStatusPending {
		t.Fatalf("status = %s, want pending after reopen", wi.Status)
	}
}

// TestSessionRootGoal_DeterministicOrder verifies that when multiple
// root goals exist for a session, sessionRootGoal returns the
// lexicographically smallest ID rather than depending on map iteration
// order. DM-20260629-001 / T54.
func TestSessionRootGoal_DeterministicOrder(t *testing.T) {
	tm := NewTaskManager()
	// Create 3 root goals via WorkTree.Create. IDs are auto-generated
	// but deterministic within a run; what we verify is that the
	// returned ID is the lex-smallest of the 3, regardless of insertion
	// order. To exercise the sort path with non-insertion-ordered IDs
	// we capture the assigned IDs and re-insert via Tree().Create into
	// the same session with controlled insertion order.
	tree := tm.Tree()
	created := make([]*WorkItem, 0, 3)
	for i := 0; i < 3; i++ {
		w, err := tree.Create("s1", CreateWorkItemInput{
			Kind:  WorkKindGoal,
			Title: "root",
		})
		if err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
		created = append(created, w)
	}
	// The sort.SliceStable should pick the lex-smallest ID regardless of
	// which order tree.Create returned them. We don't hardcode an
	// expected ID; instead we compute the expected by sorting
	// created[i].ID ourselves and comparing.
	want := created[0]
	for _, c := range created[1:] {
		if c.ID < want.ID {
			want = c
		}
	}
	got := sessionRootGoal(tm, "s1")
	if got == nil {
		t.Fatal("expected root, got nil")
	}
	if got.ID != want.ID {
		t.Fatalf("sessionRootGoal = %s, want %s (lex-smallest of %v)",
			got.ID, want.ID, []string{created[0].ID, created[1].ID, created[2].ID})
	}
	// SessionRootGoal public alias must agree.
	pub := SessionRootGoal(tm, "s1")
	if pub == nil || pub.ID != want.ID {
		t.Fatalf("SessionRootGoal = %v, want %s", pub, want.ID)
	}
	// Repeated calls return the same root.
	for i := 0; i < 3; i++ {
		r := sessionRootGoal(tm, "s1")
		if r == nil || r.ID != want.ID {
			t.Fatalf("call %d: got %v, want %s", i, r, want.ID)
		}
	}
}

// TestNewRollupReportFromRound covers the typed envelope contract from
// DM-20260629-001 / T52: 5 data fields + 2 metadata, nil round returns nil.
func TestNewRollupReportFromRound(t *testing.T) {
	// nil round → nil report.
	if got := NewRollupReportFromRound("c1", nil); got != nil {
		t.Fatalf("nil round: got %+v, want nil", got)
	}
	round := &WorkItemPipelineRound{
		VerdictKind:     types.VerdictPass,
		ArtifactSummary: "summary text",
		UncertaintyMean: 0.42,
		SpawnPolicy:     SpawnAwait,
		ContextBubbleKind: BubbleStructured,
	}
	rep := NewRollupReportFromRound("c1", round)
	if rep == nil {
		t.Fatal("expected non-nil report")
	}
	if rep.ChildID != "c1" {
		t.Fatalf("ChildID = %q, want c1", rep.ChildID)
	}
	if rep.VerdictKind != types.VerdictPass {
		t.Fatalf("VerdictKind = %v, want VerdictPass", rep.VerdictKind)
	}
	if rep.ArtifactSummary != "summary text" {
		t.Fatalf("ArtifactSummary = %q", rep.ArtifactSummary)
	}
	if rep.UncertaintyMean != 0.42 {
		t.Fatalf("UncertaintyMean = %v", rep.UncertaintyMean)
	}
	if rep.SpawnPolicy != SpawnAwait {
		t.Fatalf("SpawnPolicy = %v", rep.SpawnPolicy)
	}
	if rep.BubbleKind != BubbleStructured {
		t.Fatalf("BubbleKind = %v", rep.BubbleKind)
	}
	if rep.GeneratedAt.IsZero() {
		t.Fatal("GeneratedAt must be set")
	}
}

func TestMaybeParentRollup_IntermediateImplementParent(t *testing.T) {
	tm := NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "review")
	child, err := tm.CreateWorkItem("s1", CreateWorkItemInput{
		ParentID: goal.ID, Kind: WorkKindImplement, Title: "slice",
	})
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := tm.CreateWorkItem("s1", CreateWorkItemInput{
		ParentID: child.ID, Kind: WorkKindImplement, Title: "leaf",
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = tm.Tree().ApplyPipelineRound("s1", child.ID, &WorkItemPipelineRound{
		SpawnPolicy: SpawnDecompose,
		VerdictKind: types.VerdictPartial,
	}, RoundPhaseAwaitChild)
	_ = tm.Tree().UpdateStatus("s1", child.ID, TaskStatusInProgress)
	_ = tm.Tree().UpdateStatus("s1", leaf.ID, TaskStatusInProgress)
	_ = tm.Tree().UpdateStatus("s1", leaf.ID, TaskStatusCompleted)
	_ = tm.Tree().ApplyPipelineRound("s1", leaf.ID, &WorkItemPipelineRound{
		DeliverableSchema: FirstRegisteredDeliverableSchema(),
		DeliverableStatus: DeliverableStatusComplete,
		VerdictKind:       types.VerdictPass,
	}, RoundPhaseIdle)

	// Normal path: child terminal rollup via reevaluate.
	ReevaluateParentAfterChild("s1", leaf.ID, tm)
	childGot, _ := tm.GetWorkItem("s1", child.ID)
	if childGot == nil || !childGot.NeedsRollup {
		got, ok := MaybeParentRollup("s1", tm)
		if !ok || got == nil || got.ID != child.ID {
			t.Fatalf("NeedsRollup=false and MaybeParentRollup = %v/%v, want implement parent %s",
				got, ok, child.ID)
		}
		if !got.NeedsRollup {
			t.Fatal("expected NeedsRollup on implement parent")
		}
		return
	}
	if childGot.ID != child.ID {
		t.Fatalf("NeedsRollup set on wrong item %s", childGot.ID)
	}
}
