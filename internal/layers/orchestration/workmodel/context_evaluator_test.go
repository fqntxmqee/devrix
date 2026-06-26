package workmodel

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/shared/types"
)

func linkCtx(from, to *WorkItem, parentRound *WorkItemPipelineRound) ContextLinkEvalContext {
	fromScope := NewContextScope("s1", from.ID, plan.PersistSession)
	toScope := NewContextScope("s1", to.ID, plan.PersistSession)
	return ContextLinkEvalContext{
		SessionID:   "s1",
		FromItem:    from,
		ToItem:      to,
		FromScope:   fromScope,
		ToScope:     toScope,
		ParentRound: parentRound,
		ShareTokenCap: 1024,
	}
}

func TestEvaluateContextLinkSpec_CL0_Dependency(t *testing.T) {
	up := &WorkItem{ID: "up", ParentID: "p", Status: TaskStatusCompleted}
	dep := &WorkItem{ID: "dep", ParentID: "p", BlockedBy: []string{"up"}}
	ctx := linkCtx(up, dep, nil)
	spec := ContextLinkSpec{FromWorkItemID: "up", ToWorkItemID: "dep", Kind: LinkUpstream}
	dec := EvaluateContextLinkSpec(spec, ctx)
	if !dec.Accepted || dec.Record == nil || dec.Record.Kind != LinkUpstream {
		t.Fatalf("dec=%+v", dec)
	}
}

func TestEvaluateContextLinkSpec_CL1_ShareSummary(t *testing.T) {
	from := &WorkItem{ID: "a", ParentID: "p", Status: TaskStatusCompleted}
	to := &WorkItem{ID: "b", ParentID: "p", Status: TaskStatusPending}
	round := &WorkItemPipelineRound{SpawnPolicy: SpawnDecompose}
	ctx := linkCtx(from, to, round)
	spec := ContextLinkSpec{FromWorkItemID: "a", ToWorkItemID: "b", Kind: LinkShareSummary, MaxTokens: 512}
	dec := EvaluateContextLinkSpec(spec, ctx)
	if !dec.Accepted || dec.Record.Kind != LinkShareSummary {
		t.Fatalf("dec=%+v", dec)
	}
}

func TestEvaluateContextLinkSpec_CL1_RequiresDecompose(t *testing.T) {
	from := &WorkItem{ID: "a", ParentID: "p", Status: TaskStatusCompleted}
	to := &WorkItem{ID: "b", ParentID: "p", Status: TaskStatusPending}
	ctx := linkCtx(from, to, &WorkItemPipelineRound{SpawnPolicy: SpawnNone})
	spec := ContextLinkSpec{FromWorkItemID: "a", ToWorkItemID: "b", Kind: LinkShareSummary}
	dec := EvaluateContextLinkSpec(spec, ctx)
	if dec.Accepted || dec.RejectRule != "CL1_requires_spawn_decompose" {
		t.Fatalf("dec=%+v", dec)
	}
}

func TestEvaluateContextLinkSpec_CL1_CompletedToPendingOnly(t *testing.T) {
	from := &WorkItem{ID: "a", ParentID: "p", Status: TaskStatusPending}
	to := &WorkItem{ID: "b", ParentID: "p", Status: TaskStatusPending}
	round := &WorkItemPipelineRound{SpawnPolicy: SpawnDecompose}
	ctx := linkCtx(from, to, round)
	spec := ContextLinkSpec{FromWorkItemID: "a", ToWorkItemID: "b", Kind: LinkShareSummary}
	dec := EvaluateContextLinkSpec(spec, ctx)
	if dec.Accepted || dec.RejectRule != "CL1_from_not_completed" {
		t.Fatalf("dec=%+v", dec)
	}
}

func TestEvaluateContextLinkSpec_CL4_ParallelBatch(t *testing.T) {
	a := &WorkItem{ID: "a", ParentID: "p", Policy: ExecPolicyParallelOK}
	b := &WorkItem{ID: "b", ParentID: "p", Policy: ExecPolicyParallelOK}
	ctx := linkCtx(a, b, nil)
	spec := ContextLinkSpec{FromWorkItemID: "a", ToWorkItemID: "b", Kind: LinkShareSummary}
	dec := EvaluateContextLinkSpec(spec, ctx)
	if dec.Accepted || dec.RejectRule != "CL4_parallel_batch" {
		t.Fatalf("dec=%+v", dec)
	}
}

func TestEvaluateContextLinkSpec_CL8_Cycle(t *testing.T) {
	a := &WorkItem{ID: "a", ParentID: "p", Status: TaskStatusCompleted}
	b := &WorkItem{ID: "b", ParentID: "p", Status: TaskStatusPending}
	ctx := linkCtx(a, b, &WorkItemPipelineRound{SpawnPolicy: SpawnDecompose})
	ctx.ExistingLinks = []ContextLinkRecord{{FromWorkItemID: "b", ToWorkItemID: "a", Kind: LinkShareSummary}}
	spec := ContextLinkSpec{FromWorkItemID: "a", ToWorkItemID: "b", Kind: LinkShareSummary}
	dec := EvaluateContextLinkSpec(spec, ctx)
	if dec.Accepted || dec.RejectRule != "CL8_cycle" {
		t.Fatalf("dec=%+v", dec)
	}
}

func TestInferDependencyContextLink(t *testing.T) {
	up := &WorkItem{ID: "up", ParentID: "p"}
	dep := &WorkItem{ID: "dep", ParentID: "p", BlockedBy: []string{"up"}}
	upScope := NewContextScope("s1", up.ID, plan.PersistSession)
	depScope := NewContextScope("s1", dep.ID, plan.PersistSession)
	rec := InferDependencyContextLink(up, dep, upScope, depScope)
	if rec == nil || rec.Kind != LinkUpstream || rec.ProposedBy != ContextLinkProposedByRule+"R2_dependency" {
		t.Fatalf("rec=%+v", rec)
	}
}

func TestContextBubbleEvaluator_CB0_StructuredMinimum(t *testing.T) {
	dec := ContextBubbleEvaluator(&ContextBubbleSpec{Kind: BubbleNone}, ContextBubbleEvalContext{})
	if dec.Kind != BubbleStructured {
		t.Fatalf("got %q", dec.Kind)
	}
}

func TestContextBubbleEvaluator_CB1_Transient(t *testing.T) {
	dec := ContextBubbleEvaluator(
		&ContextBubbleSpec{Kind: BubbleFullTail},
		ContextBubbleEvalContext{PersistScope: plan.PersistTransient},
	)
	if dec.Kind != BubbleStructured || !dec.Downgraded {
		t.Fatalf("dec=%+v", dec)
	}
}

func TestContextBubbleEvaluator_CB4_FailExploratory(t *testing.T) {
	dec := ContextBubbleEvaluator(
		&ContextBubbleSpec{Kind: BubbleKeyMessages, MaxTokens: 100},
		ContextBubbleEvalContext{
			TokenBudget: 512,
			Round: &WorkItemPipelineRound{
				VerdictKind: types.VerdictFail,
				PlanKind:    plan.ExplorationPlan,
			},
		},
	)
	if dec.Kind != BubbleKeyMessages {
		t.Fatalf("dec=%+v", dec)
	}
}

func TestContextBubbleEvaluator_CB5_HumanReview(t *testing.T) {
	child := &WorkItem{Kind: WorkKindVerify, Title: HumanReviewItemTitle}
	dec := ContextBubbleEvaluator(nil, ContextBubbleEvalContext{Child: child})
	if dec.Kind != BubbleNone {
		t.Fatalf("dec=%+v", dec)
	}
}
