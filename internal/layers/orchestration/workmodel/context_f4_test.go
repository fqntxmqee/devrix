package workmodel

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestDefaultContextProposer_ShareSummary(t *testing.T) {
	ResetContextGraphState()
	tm := NewTaskManager()
	parent, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{Kind: WorkKindPlan, Title: "parent"})
	_ = tm.Tree().ApplyPipelineRound("s1", parent.ID, &WorkItemPipelineRound{
		SpawnPolicy: SpawnDecompose,
	}, RoundPhaseAwaitChild)
	a, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{ParentID: parent.ID, Kind: WorkKindExplore, Title: "a"})
	b, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{ParentID: parent.ID, Kind: WorkKindExplore, Title: "b"})
	_ = tm.UpdateStatus("s1", a.ID, TaskStatusInProgress)
	_ = tm.UpdateStatus("s1", a.ID, TaskStatusCompleted)

	out := DefaultContextProposer{}.ProposeContext("s1", a, &WorkItemPipelineRound{
		WorkItemID: a.ID, VerdictKind: types.VerdictPass, PlanKind: plan.ExplorationPlan,
	}, tm)
	if len(out.ContextLinkSpecs) != 1 || out.ContextLinkSpecs[0].ToWorkItemID != b.ID {
		t.Fatalf("specs=%+v", out.ContextLinkSpecs)
	}
}

func TestApplyAcceptedContextLinks_Dependency(t *testing.T) {
	ResetContextGraphState()
	t.Setenv(FeatureWorkItemContextGraphEnv, "1")
	tm := NewTaskManager()
	up, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{Kind: WorkKindImplement, Title: "up"})
	dep, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{Kind: WorkKindImplement, Title: "dep"})
	_ = tm.Tree().AddDependency("s1", dep.ID, up.ID)

	ApplyAcceptedContextLinks("s1", dep, nil, tm)
	links := tm.ListContextLinks("s1")
	if len(links) != 1 || links[0].Kind != LinkUpstream {
		t.Fatalf("links=%+v", links)
	}
	got, _ := tm.GetWorkItem("s1", dep.ID)
	if got.ContextPolicy != LinkUpstream {
		t.Fatalf("policy=%q", got.ContextPolicy)
	}
	if got.ContextScopeID == "" || up.ContextScopeID == "" {
		t.Fatal("expected context scopes on dependency chain")
	}
}

func TestContextResolveHint(t *testing.T) {
	ResetContextGraphState()
	t.Setenv(FeatureWorkItemContextGraphEnv, "1")
	tm := NewTaskManager()
	item, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{Kind: WorkKindImplement, Title: "x"})
	tm.EnsureContextScope("s1", item)
	hint := ContextResolveHint("s1", tm, item)
	if hint == "" || !containsSubstring(hint, "sidechain") {
		t.Fatalf("hint=%q", hint)
	}
}

func TestCLIContextShow(t *testing.T) {
	ResetContextGraphState()
	t.Setenv(FeatureWorkItemContextGraphEnv, "1")
	tm := NewTaskManager()
	cli := NewCLICommands(tm)
	item, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{Kind: WorkKindImplement, Title: "wi"})
	out := cli.Handle(&Command{Name: "context", Args: []string{"show", item.ID}}, "s1")
	if out == "" || !containsSubstring(out, ContextScopeSidechainKey(item.ID)) {
		t.Fatalf("out=%q", out)
	}
}
