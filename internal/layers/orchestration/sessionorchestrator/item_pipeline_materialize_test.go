package sessionorchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/materialize"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/orchestration/hardening"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestMaterialize_SiblingPrivateChainIsolation(t *testing.T) {
	dir := t.TempDir()
	store, _ := materialize.NewPartitionStore(dir)
	mat := materialize.NewDefaultMaterializer(store, dir)

	secret := "SECRET_SIBLING_A_TOOL_OUTPUT_XYZ"
	_ = store.Append("s1", "wi_a", []types.Message{{
		SessionID: "s1", Role: types.MessageRoleTool, Content: secret,
	}})

	childB := &workmodel.WorkItem{ID: "wi_b", ParentID: "parent", Kind: workmodel.WorkKindImplement}
	tm := workmodel.NewTaskManager()
	_, _ = tm.EnsureGoal("s1", "goal")
	_ = tm.SetScopeContract("s1", "parent", &workmodel.ScopeContract{InScope: []string{"pkg/a"}})

	req := BuildMaterializeRequest("s1", childB, tm, "work on B", DefaultWorkItemTokenBudget)
	res, err := mat.Materialize(context.Background(), req)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	payload := res.SystemPrompt
	for _, m := range res.Messages {
		payload += m.Content
	}
	if strings.Contains(payload, secret) {
		t.Fatal("sibling A private chain leaked into B materialize payload")
	}
}

func TestMaterialize_ChildDownlinkScopeInPrompt(t *testing.T) {
	tm := workmodel.NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "implement feature")
	_, err := tm.DecomposeChildren("s1", goal.ID, []workmodel.ChildSpec{{
		Title: "child", Directive: "child work", ExpectedReturn: "patch",
		ScopeIn: []string{"internal/foo.go"},
	}})
	if err != nil {
		t.Fatalf("DecomposeChildren: %v", err)
	}
	children := tm.Tree().ListChildren("s1", goal.ID)
	if len(children) == 0 {
		t.Fatal("no children")
	}
	child := children[0]

	req := BuildMaterializeRequest("s1", child, tm, "child work", DefaultWorkItemTokenBudget)
	mat := materialize.NewDefaultMaterializer(nil, "")
	res, err := mat.Materialize(context.Background(), req)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if !strings.Contains(res.SystemPrompt, "internal/foo.go") {
		t.Fatalf("system prompt missing ScopeIn: %q", res.SystemPrompt)
	}
}

func TestBuildMaterializeRequest_UpstreamBlockedBy(t *testing.T) {
	tm := workmodel.NewTaskManager()
	blocker, _ := tm.CreateWorkItem("s1", workmodel.CreateWorkItemInput{
		Kind: workmodel.WorkKindImplement, Title: "upstream", Directive: "do A",
	})
	_ = tm.Tree().ApplyPipelineRound("s1", blocker.ID, &workmodel.WorkItemPipelineRound{
		WorkItemID:      blocker.ID,
		PlanID:            "plan_a",
		VerdictID:         "verdict_a",
		VerdictKind:       types.VerdictPass,
		ArtifactSummary:   "upstream result text",
		ContextBubbleKind: workmodel.BubbleStructured,
	}, workmodel.RoundPhaseIdle)
	_ = tm.UpdateStatus("s1", blocker.ID, workmodel.TaskStatusCompleted)

	dep, _ := tm.CreateWorkItem("s1", workmodel.CreateWorkItemInput{
		Kind: workmodel.WorkKindImplement, Title: "downstream", Directive: "do B",
	})
	_ = tm.Tree().AddDependency("s1", dep.ID, blocker.ID)
	dep, _ = tm.GetWorkItem("s1", dep.ID)

	req := BuildMaterializeRequest("s1", dep, tm, "do B", DefaultWorkItemTokenBudget)
	if req.Policy.Mode != materialize.ModeUpstream {
		t.Fatalf("mode = %q, want upstream", req.Policy.Mode)
	}
	if len(req.Signals.SignalLines) == 0 {
		t.Fatal("expected upstream signal lines")
	}
	payload := strings.Join(req.Signals.SignalLines, " ")
	if !strings.Contains(payload, "structured_child_bubble") {
		t.Fatalf("missing structured bubble: %q", payload)
	}
	if !strings.Contains(payload, "upstream result text") {
		t.Fatalf("missing artifact summary: %q", payload)
	}
	secret := "BLOCKER_PRIVATE_CHAIN_SECRET"
	dir := t.TempDir()
	store, _ := materialize.NewPartitionStore(dir)
	_ = store.Append("s1", blocker.ID, []types.Message{{Content: secret}})
	mat := materialize.NewDefaultMaterializer(store, dir)
	res, err := mat.Materialize(context.Background(), req)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if strings.Contains(res.SystemPrompt, secret) {
		t.Fatal("upstream private chain must not appear in BlockedBy materialize")
	}
}

func TestRunItemPipeline_ScopeContractOpenQuestionsAllowsDecompose(t *testing.T) {
	tm := workmodel.NewTaskManager()
	block := `<scope_contract>{"goal_statement":"compare caches","in_scope":["cache"],"open_questions":["Redis or Memcached?"]}</scope_contract>`
	exec := &contentWorkItemExecutor{content: block}
	runner, err := NewItemPipelineRunner(ItemPipelineDeps{Executor: exec, Tasks: tm})
	if err != nil {
		t.Fatalf("NewItemPipelineRunner: %v", err)
	}
	runner.Verify = func(_ *wavescheduler.Artifact) workmodel.Verdict {
		return workmodel.Verdict{
			Kind: types.VerdictPartial, SourceID: "v", Confidence: 0.4, Reason: "explore",
		}
	}
	sessionID := "sess-scope-decompose"
	goal, _ := tm.EnsureGoal(sessionID, "compare cache strategies")
	_ = tm.Tree().SetUncertainty(sessionID, goal.ID, 0.9)

	round, err := runner.Run(context.Background(), sessionID, goal, "", ItemPipelineRunOpts{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if round.SpawnPolicy != workmodel.SpawnDecompose {
		t.Fatalf("SpawnPolicy = %q, want decompose for open_questions exploration (got %+v)", round.SpawnPolicy, round)
	}
	got, _ := tm.GetWorkItem(sessionID, goal.ID)
	if got.ScopeContract == nil || !got.ScopeContract.HasOpenQuestions() {
		t.Fatalf("ScopeContract not persisted: %+v", got.ScopeContract)
	}
}

func TestRunItemPipeline_GoalPlan_DirectiveUnmodified(t *testing.T) {
	tm := workmodel.NewTaskManager()
	exec := &capturingWorkItemExecutor{}
	runner, err := NewItemPipelineRunner(ItemPipelineDeps{Executor: exec, Tasks: tm})
	if err != nil {
		t.Fatalf("NewItemPipelineRunner: %v", err)
	}
	const base = "explore architecture"
	goal, _ := tm.EnsureGoal("s1", base)
	_, _ = runner.Run(context.Background(), "s1", goal, "", ItemPipelineRunOpts{})
	if len(exec.calls) == 0 {
		t.Fatal("no execute call")
	}
	if exec.calls[0].Directive != base {
		t.Fatalf("Goal directive must not be mutated with tactical appendix: %q", exec.calls[0].Directive)
	}
}

func TestEmitContextMaterialize_SpanNoPanic(t *testing.T) {
	b := observability.NewBridge(observability.NewNoOp())
	hardening.SetBridge(b)
	t.Cleanup(func() { hardening.SetBridge(nil) })

	end := hardening.EmitContextMaterialize(context.Background(), "s1", "wi1", "fresh", 3, 120)
	if end == nil {
		t.Fatal("end func nil")
	}
	end(nil)
}

func TestMaterialize_DepthSubContextDiffersFromSessionPrepare(t *testing.T) {
	dir := t.TempDir()
	store, _ := materialize.NewPartitionStore(dir)
	mat := materialize.NewDefaultMaterializer(store, dir)
	tm := workmodel.NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "g")
	_, err := tm.DecomposeChildren("s1", goal.ID, []workmodel.ChildSpec{{
		Title: "c", Directive: "do work", ExpectedReturn: "result", ScopeIn: []string{"internal/x.go"},
	}})
	if err != nil {
		t.Fatalf("DecomposeChildren: %v", err)
	}
	children := tm.Tree().ListChildren("s1", goal.ID)
	if len(children) == 0 {
		t.Fatal("no children")
	}
	child := children[0]
	if tm.Tree().Depth("s1", child.ID) < 1 {
		t.Fatal("child depth should be >= 1")
	}
	ctx := WithWorkItemExecContext(context.Background(), WorkItemExecContext{Item: child, Tasks: tm})
	if !ShouldMaterializeWorkItem(ctx, "s1", child.ID) {
		t.Fatal("depth>=1 child should materialize by default")
	}
	req := BuildMaterializeRequest("s1", child, tm, "do work", DefaultWorkItemTokenBudget)
	res, err := mat.Materialize(ctx, req)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if res.SystemPrompt == "" || !strings.Contains(res.SystemPrompt, "internal/x.go") {
		t.Fatalf("subcontext prompt unexpected: %q", res.SystemPrompt)
	}
}

func TestMaterialize_NoObsTaxonomyInPrivateChainTemplate(t *testing.T) {
	mat := materialize.NewDefaultMaterializer(nil, "")
	res, err := mat.Materialize(context.Background(), materialize.Request{
		Partition: materialize.Partition{SessionID: "s1", WorkItemID: "wi1"},
		Policy:    materialize.Policy{TokenBudget: 1000},
		Signals:   materialize.InboundSignals{Directive: "work"},
	})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	// Locale-aware footer (default zh-CN); Obs taxonomy names are English in both locales.
	if !strings.Contains(res.SystemPrompt, "ObsFact/ObsSignal") {
		t.Fatal("must forbid Execute-side Obs taxonomy")
	}
}

func TestShouldMaterializeWorkItem_L0GoalUsesLegacyPrepare(t *testing.T) {
	tm := workmodel.NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "g")
	ctx := WithWorkItemExecContext(context.Background(), WorkItemExecContext{Item: goal, Tasks: tm})
	if ShouldMaterializeWorkItem(ctx, "s1", goal.ID) {
		t.Fatal("L0 Goal should use legacy Prepare, not Materialize")
	}
}
