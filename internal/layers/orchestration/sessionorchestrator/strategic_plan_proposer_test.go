package sessionorchestrator

import (
	"errors"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

func TestParseStrategicPlanJSON_should_accept_single_mode(t *testing.T) {
	raw := `{"execution_mode":"single","scope_in":["internal/foo/"],"child_specs":[],"deliverable_schema":"p0_p1_file_line","react_iters_hint":3,"rationale":"one pass"}`
	prop, err := parseStrategicPlanJSON(raw, "review internal/foo/")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if prop.ExecutionMode != "single" {
		t.Fatalf("mode = %q", prop.ExecutionMode)
	}
	if prop.QuantizedKind != "intent_command" {
		t.Fatalf("quantized = %q", prop.QuantizedKind)
	}
	if len(prop.ChildSpecs) != 0 {
		t.Fatalf("expected no child specs for single, got %d", len(prop.ChildSpecs))
	}
}

func TestParseStrategicPlanJSON_should_map_decompose_child_specs(t *testing.T) {
	raw := `{"execution_mode":"decompose","child_specs":[{"title":"slice A","directive_suffix":"focus A","expected_return":"P0 list"}],"deliverable_schema":"p0_p1_file_line","react_iters_hint":2}`
	prop, err := parseStrategicPlanJSON(raw, "review kernel")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(prop.ChildSpecs) != 1 {
		t.Fatalf("child specs = %d", len(prop.ChildSpecs))
	}
	if !strings.Contains(prop.ChildSpecs[0].Directive, "focus A") {
		t.Fatalf("directive = %q", prop.ChildSpecs[0].Directive)
	}
}

func TestValidateStrategicPlan_should_reject_decompose_without_children(t *testing.T) {
	_, err := parseStrategicPlanJSON(`{"execution_mode":"decompose","child_specs":[]}`, "review")
	if err == nil {
		t.Fatal("expected error for decompose without child_specs")
	}
}

func TestAppendDeliverableExecuteHint_should_add_schema_tag_for_review_schema(t *testing.T) {
	got := AppendDeliverableExecuteHint("review code", workmodel.DeliverableSchemaP0P1FileLine)
	if !strings.Contains(got, "<deliverable_schema>p0_p1_file_line</deliverable_schema>") {
		t.Fatalf("missing schema tag: %q", got)
	}
}

func TestAppendDeliverableExecuteHint_should_skip_when_not_applicable(t *testing.T) {
	base := "implement feature"
	if got := AppendDeliverableExecuteHint(base, workmodel.DeliverableSchemaNotApplicable); got != base {
		t.Fatalf("unexpected mutation: %q", got)
	}
}

// T: D7-S5-A92-T04 (DM-20260701-001 T-P1-4 PlanPrompt snapshot)
//
// The Plan user prompt MUST surface all divergence budget fields so the
// LLM proposer can self-bound its proposal. Snapshot test asserts exact
// field set + ordering — reordering breaks downstream prompt tracking.
func TestBuildStrategicPlanUserPrompt_AllBudgetFields(t *testing.T) {
	in := StrategicPlanInput{
		WorkItemID: "wi_42",
		Directive:  "review d2 code",
		Budget: workmodel.DivergenceBudget{
			Depth:              1,
			MaxDepth:           3,
			ExistingChildren:   2,
			MaxChildren:        7,
			DecomposeUsedToday: 1,
			MaxDaily:           5,
			MaxIters:           5,
		},
		ParentScopeIn: []string{"internal/layers/contextengine/", "internal/layers/orchestration/"},
	}
	got := buildStrategicPlanUserPrompt(in)
	for _, want := range []string{
		"work_item_id: wi_42",
		"directive: review d2 code",
		"depth: 1",
		"max_depth: 3",
		"existing_children: 2",
		"remaining_children: 5",
		"max_children: 7",
		"decompose_used_today: 1",
		"remaining_daily: 4",
		"max_daily: 5",
		"max_iters: 5",
		"parent_scope_in: internal/layers/contextengine/,internal/layers/orchestration/",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Plan user prompt missing %q, got:\n%s", want, got)
		}
	}
}

// T: D7-S5-A92-T04 (DM-20260701-001 T-P1-4 PlanPrompt snapshot)
//
// When the parent has no in-scope paths and the budget is all-zero (the
// "no strategic-plan infrastructure" baseline), the prompt must NOT
// emit any of the budget lines. This keeps the legacy prompt identical
// for callers that don't wire the new fields.
func TestBuildStrategicPlanUserPrompt_EmptyBudgetOmits(t *testing.T) {
	got := buildStrategicPlanUserPrompt(StrategicPlanInput{
		WorkItemID: "wi_x",
		Directive:  "do x",
	})
	for _, banned := range []string{"depth:", "max_depth:", "remaining_children:", "max_iters:", "parent_scope_in:"} {
		if strings.Contains(got, banned) {
			t.Errorf("empty-budget prompt must not contain %q, got:\n%s", banned, got)
		}
	}
}

// T: D7-S5-A92-T01..T03 (DM-20260701-001 T-P1-1, T-P1-3 budget constants + structured reject)
//
// DivergenceBudget.RemainingChildren / RemainingDaily clamp at 0 when
// the parent's existing count exceeds the cap (e.g. a misconfigured
// tree). The cap function should never go negative; the LLM sees
// "remaining=0" and falls back to execution_mode=single.
func TestDivergenceBudget_RemainingClampsAtZero(t *testing.T) {
	cases := []struct {
		name            string
		budget          workmodel.DivergenceBudget
		wantRemainChild int
		wantRemainDaily int
	}{
		{
			name:            "nominal",
			budget:          workmodel.DivergenceBudget{ExistingChildren: 2, MaxChildren: 7, DecomposeUsedToday: 1, MaxDaily: 5},
			wantRemainChild: 5,
			wantRemainDaily: 4,
		},
		{
			name:            "over existing (clamps to 0)",
			budget:          workmodel.DivergenceBudget{ExistingChildren: 9, MaxChildren: 7, DecomposeUsedToday: 1, MaxDaily: 5},
			wantRemainChild: 0,
			wantRemainDaily: 4,
		},
		{
			name:            "over daily (clamps to 0)",
			budget:          workmodel.DivergenceBudget{ExistingChildren: 0, MaxChildren: 7, DecomposeUsedToday: 9, MaxDaily: 5},
			wantRemainChild: 7,
			wantRemainDaily: 0,
		},
		{
			name:            "all zero (clamps to 0)",
			budget:          workmodel.DivergenceBudget{},
			wantRemainChild: 0,
			wantRemainDaily: 0,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.budget.RemainingChildren(); got != c.wantRemainChild {
				t.Errorf("RemainingChildren = %d, want %d", got, c.wantRemainChild)
			}
			if got := c.budget.RemainingDaily(); got != c.wantRemainDaily {
				t.Errorf("RemainingDaily = %d, want %d", got, c.wantRemainDaily)
			}
		})
	}
}

// T: D7-S5-A92-T03 (DM-20260701-001 T-P1-3 structured reject)
//
// applyBudgetCap returns a *StrategicPlanReject (carrying Reason +
// MaxAllowed + Requested) when the LLM proposes more children than the
// budget allows. Without this, the silent CapChildSpecs truncation
// would hide the over-budget condition from the LLM.
func TestApplyBudgetCap_RejectsOverChildren(t *testing.T) {
	prop := &StrategicPlanProposal{
		ExecutionMode: "decompose",
		ChildSpecs: []workmodel.ChildSpec{
			{Kind: workmodel.WorkKindImplement, Title: "a"},
			{Kind: workmodel.WorkKindImplement, Title: "b"},
			{Kind: workmodel.WorkKindImplement, Title: "c"},
			{Kind: workmodel.WorkKindImplement, Title: "d"},
			{Kind: workmodel.WorkKindImplement, Title: "e"},
		},
	}
	budget := workmodel.DivergenceBudget{
		ExistingChildren: 5, MaxChildren: 7, // remaining=2
	}
	err := applyBudgetCap(prop, budget)
	if err == nil {
		t.Fatal("applyBudgetCap must reject 5 children on remaining=2")
	}
	var reject *StrategicPlanReject
	if !errors.As(err, &reject) {
		t.Fatalf("error must be *StrategicPlanReject, got %T: %v", err, err)
	}
	if reject.Reason != BudgetFieldChildren {
		t.Errorf("Reason = %q, want %q", reject.Reason, BudgetFieldChildren)
	}
	if reject.MaxAllowed != 7 {
		t.Errorf("MaxAllowed = %d, want 7", reject.MaxAllowed)
	}
	if reject.Requested != 5 {
		t.Errorf("Requested = %d, want 5", reject.Requested)
	}
}

func TestApplyBudgetCap_RejectsOverDaily(t *testing.T) {
	prop := &StrategicPlanProposal{
		ExecutionMode: "decompose",
		ChildSpecs: []workmodel.ChildSpec{
			{Kind: workmodel.WorkKindImplement, Title: "a"},
			{Kind: workmodel.WorkKindImplement, Title: "b"},
			{Kind: workmodel.WorkKindImplement, Title: "c"},
		},
	}
	budget := workmodel.DivergenceBudget{
		ExistingChildren: 0, MaxChildren: 7, // remaining=7 (children OK)
		DecomposeUsedToday: 4, MaxDaily: 5, // remaining=1 (daily over)
	}
	err := applyBudgetCap(prop, budget)
	if err == nil {
		t.Fatal("applyBudgetCap must reject 3 children on remaining_daily=1")
	}
	var reject *StrategicPlanReject
	if !errors.As(err, &reject) {
		t.Fatalf("error must be *StrategicPlanReject, got %T: %v", err, err)
	}
	if reject.Reason != BudgetFieldDaily {
		t.Errorf("Reason = %q, want %q", reject.Reason, BudgetFieldDaily)
	}
}

func TestApplyBudgetCap_AcceptsWithinBudget(t *testing.T) {
	prop := &StrategicPlanProposal{
		ExecutionMode: "decompose",
		ChildSpecs: []workmodel.ChildSpec{
			{Kind: workmodel.WorkKindImplement, Title: "a"},
			{Kind: workmodel.WorkKindImplement, Title: "b"},
		},
	}
	budget := workmodel.DivergenceBudget{
		ExistingChildren: 0, MaxChildren: 7, // remaining=7
		DecomposeUsedToday: 0, MaxDaily: 5, // remaining=5
	}
	if err := applyBudgetCap(prop, budget); err != nil {
		t.Errorf("within-budget proposal must pass, got %v", err)
	}
}

func TestApplyBudgetCap_SkipsNonDecompose(t *testing.T) {
	prop := &StrategicPlanProposal{
		ExecutionMode: "single",
		// Even if someone manually stuffed ChildSpecs in single mode,
		// applyBudgetCap is a no-op for non-decompose (the validator
		// already strips those above).
		ChildSpecs: []workmodel.ChildSpec{{Title: "x"}},
	}
	budget := workmodel.DivergenceBudget{ExistingChildren: 0, MaxChildren: 0}
	if err := applyBudgetCap(prop, budget); err != nil {
		t.Errorf("non-decompose mode must skip budget cap, got %v", err)
	}
}

