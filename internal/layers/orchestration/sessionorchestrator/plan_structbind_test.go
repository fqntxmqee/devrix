package sessionorchestrator

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/prompttags"
)

// planFrameBody extracts the user-prompt frame (after the i18n guide header) for
// field-level assertions. The header + frame are joined by "\n\n" in
// buildStrategicPlanUserPrompt.
func planFrameBody(s string) string {
	if i := strings.Index(s, "\n\n"); i >= 0 {
		return s[i+2:]
	}
	return s
}

// T: D7-S5-A100-T01 / L5-MUPS-GSD-11 -- init() in this package must have registered
// StrategicPlanFrame with LineFrameRegistry[FramePlanUser] without panic.
func TestStrategicPlanFrame_RegisteredAtInit(t *testing.T) {
	spec, ok := prompttags.LineFrameRegistry[prompttags.FramePlanUser]
	if !ok {
		t.Fatal("FramePlanUser missing from LineFrameRegistry")
	}
	if len(spec.Fields) != 16 {
		t.Fatalf("FramePlanUser has %d fields, want 16: %v", len(spec.Fields), spec.Fields)
	}
	expected := []prompttags.TagName{
		prompttags.TagWorkItemID, prompttags.TagDirective, prompttags.TagPriorParseReject,
		prompttags.TagObservationIDs, prompttags.TagObservationSummary,
		prompttags.TagDepth, prompttags.TagMaxDepth,
		prompttags.TagExistingChildren, prompttags.TagRemainingChildren, prompttags.TagMaxChildren,
		prompttags.TagDecomposeUsedToday, prompttags.TagRemainingDaily, prompttags.TagMaxDaily,
		prompttags.TagMaxIters, prompttags.TagParentScopeIn, prompttags.TagUncertaintyMean,
	}
	for i, want := range expected {
		if spec.Fields[i] != want {
			t.Errorf("field[%d] = %q, want %q", i, spec.Fields[i], want)
		}
	}
}

// T: D7-S5-A100-T02 / L5-MUPS-GSD-12 -- buildStrategicPlanUserPrompt output
// contains the expected lines for a fully-populated StrategicPlanInput with Budget>0.
// Byte-equivalence contract with the prior 35-line manual map.
func TestBuildStrategicPlanUserPrompt_FullInput(t *testing.T) {
	in := StrategicPlanInput{
		SessionID:        "s1",
		WorkItemID:       "wi_plan_1",
		Directive:        "ship auth refactor",
		ObservationIDs:   []string{"obs_a", "obs_b"},
		ReportSummary:    "prior round observed login regression",
		Budget: workmodel.DivergenceBudget{
			Depth: 2, MaxDepth: 4,
			ExistingChildren: 1, MaxChildren: 7,
			DecomposeUsedToday: 1, MaxDaily: 5,
			MaxIters: 5,
		},
		ParentScopeIn:   []string{"internal/auth/"},
		UncertaintyMean: 0.32,
		PriorParseReject: `{"code":"budget_over","field":"child_specs"}`,
	}
	body := planFrameBody(buildStrategicPlanUserPrompt(in, i18n.LocaleEN))
	checks := []string{
		"work_item_id: wi_plan_1",
		"directive: ship auth refactor",
		"prior_parse_reject:",
		"observation_ids:",
		"observation_summary: prior round observed login regression",
		"depth: 2",
		"max_depth: 4",
		"existing_children: 1",
		"remaining_children: 6", // 7 - 1
		"max_children: 7",
		"decompose_used_today: 1",
		"remaining_daily: 4", // 5 - 1
		"max_daily: 5",
		"max_iters: 5",
		"parent_scope_in:",
		"uncertainty_mean: 0.32",
	}
	for _, c := range checks {
		if !strings.Contains(body, c) {
			t.Errorf("expected body to contain %q, got:\n%s", c, body)
		}
	}
}

// T: D7-S5-A100-T03 / L5-MUPS-GSD-13 -- buildStrategicPlanFrame flattens
// nested DivergenceBudget 9 fields into the LLM-view frame. The 0-budget
// guard must suppress all 9 Budget fields.
func TestBuildStrategicPlanFrame_FlattensBudget(t *testing.T) {
	// Case 1: Budget=0 -> frame Budget fields all zero/empty (omit_zero suppresses).
	in0 := StrategicPlanInput{WorkItemID: "wi_x", Directive: "d"}
	frame0 := buildStrategicPlanFrame(in0)
	if frame0.MaxChildren != nil || frame0.Depth != nil || frame0.MaxIters != nil {
		t.Errorf("Budget=0: expected zero Budget fields, got %+v", frame0)
	}

	// Case 2: Budget>0 -> frame Budget fields populated.
	in1 := StrategicPlanInput{
		WorkItemID: "wi_y", Directive: "d",
		Budget: workmodel.DivergenceBudget{
			Depth: 1, MaxDepth: 3,
			ExistingChildren: 2, MaxChildren: 7,
			DecomposeUsedToday: 0, MaxDaily: 5,
			MaxIters: 5,
		},
	}
	frame1 := buildStrategicPlanFrame(in1)
	if frame1.Depth == nil || *frame1.Depth != 1 || frame1.MaxDepth == nil || *frame1.MaxDepth != 3 {
		t.Errorf("Depth/MaxDepth not flattened: %+v", frame1)
	}
	if frame1.ExistingChildren == nil || *frame1.ExistingChildren != 2 || frame1.RemainingChildren == nil || *frame1.RemainingChildren != 5 || frame1.MaxChildren == nil || *frame1.MaxChildren != 7 {
		t.Errorf("Children not flattened: %+v", frame1)
	}
	if frame1.DecomposeUsedToday == nil || *frame1.DecomposeUsedToday != 0 || frame1.RemainingDaily == nil || *frame1.RemainingDaily != 5 || frame1.MaxDaily == nil || *frame1.MaxDaily != 5 {
		t.Errorf("Daily not flattened: %+v", frame1)
	}
	if frame1.MaxIters == nil || *frame1.MaxIters != 5 {
		t.Errorf("MaxIters not flattened: %+v", frame1)
	}
}

// T: D7-S5-A100-T04 / L5-MUPS-GSD-14 -- when Budget.MaxChildren == 0,
// the 9 Budget fields must be omitted from the rendered user prompt.
func TestBuildStrategicPlanUserPrompt_ZeroBudget(t *testing.T) {
	in := StrategicPlanInput{
		WorkItemID: "wi_zero",
		Directive:  "narrow scope",
		// Budget.MaxChildren == 0 (default), so Budget fields must NOT appear.
	}
	body := planFrameBody(buildStrategicPlanUserPrompt(in, i18n.LocaleEN))
	for _, banned := range []string{"depth:", "max_depth:", "existing_children:", "remaining_children:",
		"max_children:", "decompose_used_today:", "remaining_daily:", "max_daily:", "max_iters:"} {
		if strings.Contains(body, banned) {
			t.Errorf("Budget=0 should omit %q, got:\n%s", banned, body)
		}
	}
	// WorkItemID and Directive must still be present.
	if !strings.Contains(body, "work_item_id: wi_zero") {
		t.Errorf("missing work_item_id line in:\n%s", body)
	}
	if !strings.Contains(body, "directive: narrow scope") {
		t.Errorf("missing directive line in:\n%s", body)
	}
}

// T: D7-S5-A100-T05 / L5-MUPS-GSD-15 -- 0 behavior change: golden snapshot
// for a fully-populated StrategicPlanInput (en + zh). The exact string
// must match the prior 35-line manual map output (T-P1-4 invariant).
func TestBuildStrategicPlanUserPrompt_GoldenEN(t *testing.T) {
	in := StrategicPlanInput{
		WorkItemID: "wi_g",
		Directive:  "ship it",
		Budget: workmodel.DivergenceBudget{
			Depth: 1, MaxDepth: 3,
			ExistingChildren: 0, MaxChildren: 7,
			DecomposeUsedToday: 0, MaxDaily: 5,
			MaxIters: 5,
		},
		UncertaintyMean: 0.4,
	}
	body := planFrameBody(buildStrategicPlanUserPrompt(in, i18n.LocaleEN))
	// Each line must match exactly (field: value, no extra whitespace).
	wantLines := []string{
		"[control] work_item_id: wi_g",
		"[data] directive: ship it",
		"[control] depth: 1",
		"[control] max_depth: 3",
		"[control] existing_children: 0",
		"[control] remaining_children: 7",
		"[control] max_children: 7",
		"[control] decompose_used_today: 0",
		"[control] remaining_daily: 5",
		"[control] max_daily: 5",
		"[control] max_iters: 5",
		"[control] uncertainty_mean: 0.400",
	}
	gotLines := strings.Split(strings.TrimSpace(body), "\n")
	if len(gotLines) != len(wantLines) {
		t.Fatalf("line count mismatch: got %d, want %d\nbody:\n%s", len(gotLines), len(wantLines), body)
	}
	for i, want := range wantLines {
		if gotLines[i] != want {
			t.Errorf("line[%d] = %q, want %q", i, gotLines[i], want)
		}
	}
}

// T: D7-S5-A100-T06 / L5-MUPS-GSD-15 (extended) -- verify the
// planFrameToMap helper produces a map that excludes omitted fields
// (matches the prior guide-only-when-set semantics for RenderFrameFieldGuideForFields).
func TestPlanFrameToMap_OmitsEmptyAndZero(t *testing.T) {
	frame := StrategicPlanFrame{
		WorkItemID: "wi_map",
		Directive:  "d",
		// Everything else zero/empty.
	}
	m := planFrameToMap(frame)
	if _, ok := m[prompttags.TagDepth]; ok {
		t.Errorf("Depth (zero) should be omitted, got %v", m[prompttags.TagDepth])
	}
	if _, ok := m[prompttags.TagPriorParseReject]; ok {
		t.Errorf("PriorParseReject (empty) should be omitted")
	}
	if v, ok := m[prompttags.TagWorkItemID]; !ok || v != "wi_map" {
		t.Errorf("WorkItemID = %v, want wi_map", v)
	}
}
