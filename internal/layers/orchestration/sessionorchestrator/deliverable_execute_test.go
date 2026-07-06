package sessionorchestrator

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

func TestRollupSynthesisTurnExecuteHint_nonEmpty(t *testing.T) {
	got := RollupSynthesisTurnExecuteHint()
	if got == "" {
		t.Fatal("expected non-empty rollup synthesis hint")
	}
	if !strings.Contains(got, "tools are disabled") {
		t.Fatalf("hint=%q want tools disabled guidance", got)
	}
}

func TestPriorDeliverableRetryHint_ScopeAndReason(t *testing.T) {
	contract := workmodel.DeliverableContract{
		Citation:  workmodel.DeliverableCitationFileLine,
		Severity:  workmodel.DeliverableSeverityP0P1,
		Structure: workmodel.DeliverableStructureFindingsJSON,
		Reject:    []workmodel.DeliverableReject{workmodel.DeliverableRejectPlanningMeta},
	}
	item := &workmodel.WorkItem{
		ScopeContract: &workmodel.ScopeContract{
			InScope: []string{"internal/layers/orchestration/plan/"},
		},
		LastRound: &workmodel.WorkItemPipelineRound{
			DeliverableStatus: workmodel.DeliverableStatusIncomplete,
			ArtifactSummary:   "Let me read openspec files first.",
		},
	}
	got := PriorDeliverableRetryHint(item, contract)
	for _, want := range []string{
		"ScopeIn: internal/layers/orchestration/plan/",
		"PriorDeliverableFailure: planning_meta",
		"synthesize findings_json",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("hint missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "scope_disjoint") {
		t.Fatalf("must not include spawn rationale: %q", got)
	}
}

func TestEffectiveExecuteMaxIters_FindingsJSONFloor(t *testing.T) {
	c := workmodel.DeliverableContract{Structure: workmodel.DeliverableStructureFindingsJSON}
	if got := workmodel.EffectiveExecuteMaxIters(3, 5, c); got != 5 {
		t.Fatalf("got %d want 5", got)
	}
	if got := workmodel.EffectiveExecuteMaxIters(0, 5, c); got != 5 {
		t.Fatalf("default got %d want 5", got)
	}
	free := workmodel.DeliverableContract{Structure: workmodel.DeliverableStructureFreeText}
	if got := workmodel.EffectiveExecuteMaxIters(3, 5, free); got != 3 {
		t.Fatalf("free text got %d want 3", got)
	}
}

func TestUncertaintyReportSummary_IncludesObservationText(t *testing.T) {
	// DM-20260706-007: when Observe emits a high-strength ObsUncertainty
	// (the d7-path-unknown case from sess_1783333760211_6000), the
	// question must reach the Plan LLM via observation_summary. Previously
	// uncertaintyReportSummary only emitted "intent=fast; anomalies=N",
	// forcing Plan to guess the path.
	uq, err := orchtypes.NewObservation(
		orchtypes.ObsUncertainty, orchtypes.CatBusiness, 0.82,
		orchtypes.UncertaintyPayload{Question: "d7 领域的具体路径是什么?", Confidence: 0.18, RequiresMore: true},
		"observe_proposer",
	)
	if err != nil {
		t.Fatalf("setup uncertainty obs: %v", err)
	}
	rep, err := orchtypes.NewUncertaintyReport("sess_test", []orchtypes.Observation{uq})
	if err != nil {
		t.Fatalf("setup report: %v", err)
	}
	got := uncertaintyReportSummary(rep, "fast")
	for _, want := range []string{"intent=fast", "d7 领域的具体路径是什么?"} {
		if !strings.Contains(got, want) {
			t.Fatalf("observation_summary missing %q:\n%s", want, got)
		}
	}
}

func TestUncertaintyReportSummary_EmptyAll(t *testing.T) {
	rep, err := orchtypes.NewUncertaintyReport("sess_test", nil)
	if err != nil {
		t.Fatalf("setup report: %v", err)
	}
	got := uncertaintyReportSummary(rep, "")
	if got != "" {
		t.Fatalf("expected empty when no observations and no intent, got %q", got)
	}
}

func TestUncertaintyReportSummary_IntentOnlyNoObs(t *testing.T) {
	rep, err := orchtypes.NewUncertaintyReport("sess_test", nil)
	if err != nil {
		t.Fatalf("setup report: %v", err)
	}
	got := uncertaintyReportSummary(rep, "fast")
	if got != "intent=fast" {
		t.Fatalf("intent-only got %q want intent=fast", got)
	}
}

func TestUncertaintyReportSummary_TruncatesLongQuestion(t *testing.T) {
	long := strings.Repeat("x", 200)
	uq, err := orchtypes.NewObservation(
		orchtypes.ObsUncertainty, orchtypes.CatBusiness, 0.8,
		orchtypes.UncertaintyPayload{Question: long, Confidence: 0.2, RequiresMore: true},
		"observe_proposer",
	)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	rep, err := orchtypes.NewUncertaintyReport("sess_test", []orchtypes.Observation{uq})
	if err != nil {
		t.Fatalf("setup report: %v", err)
	}
	got := uncertaintyReportSummary(rep, "")
	// Hard cap on per-question length so observation_summary stays bounded
	// even if the LLM emits a 1kB question (which would blow up the LLM
	// request frame). 120 chars is the same ceiling used elsewhere for
	// ObsUncertainty.Question summaries.
	if strings.Contains(got, long) {
		t.Fatalf("question not truncated; got len=%d", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("truncated question should end with ellipsis, got %q", got)
	}
}

func TestUncertaintyReportSummary_FiltersLowStrengthUncertainty(t *testing.T) {
	// Below-threshold ObsUncertainty should be suppressed so Plan isn't
	// flooded with weak noise (matches obsUncertaintyAnomalyThreshold).
	weak, err := orchtypes.NewObservation(
		orchtypes.ObsUncertainty, orchtypes.CatBusiness, 0.5,
		orchtypes.UncertaintyPayload{Question: "weak question", Confidence: 0.5, RequiresMore: true},
		"observe_proposer",
	)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	rep, err := orchtypes.NewUncertaintyReport("sess_test", []orchtypes.Observation{weak})
	if err != nil {
		t.Fatalf("setup report: %v", err)
	}
	got := uncertaintyReportSummary(rep, "fast")
	if strings.Contains(got, "weak question") {
		t.Fatalf("low-strength uncertainty should be filtered, got %q", got)
	}
}

// DM-20260706-009: regression test for the 1+1=几? failure. Previously
// ObsFact.statement was silently dropped from observation_summary, so a
// high-strength fact ("1+1=2") wouldn't reach the Plan LLM. With the fix,
// the statement must appear as `fact=...` so Plan knows the question is
// already answered.
func TestUncertaintyReportSummary_IncludesObsFactStatement(t *testing.T) {
	fact, err := orchtypes.NewObservation(
		orchtypes.ObsFact, orchtypes.CatBusiness, 0.99,
		orchtypes.FactPayload{Statement: "在标准算术下，1+1=2。", Evidence: nil},
		"observe_proposer",
	)
	if err != nil {
		t.Fatalf("setup fact: %v", err)
	}
	rep, err := orchtypes.NewUncertaintyReport("sess_test", []orchtypes.Observation{fact})
	if err != nil {
		t.Fatalf("setup report: %v", err)
	}
	got := uncertaintyReportSummary(rep, "fast")
	for _, want := range []string{"intent=fast", "fact=在标准算术下，1+1=2。"} {
		if !strings.Contains(got, want) {
			t.Fatalf("observation_summary missing %q:\n%s", want, got)
		}
	}
}

// DM-20260706-009: CatSystem ObsFact (e.g. routing metadata) should NOT
// reach the Plan summary. Only CatBusiness facts change the Plan decision;
// system facts are infrastructure noise from the Plan LLM's perspective.
func TestUncertaintyReportSummary_FiltersSystemFact(t *testing.T) {
	sysFact, err := orchtypes.NewObservation(
		orchtypes.ObsFact, orchtypes.CatSystem, 0.99,
		orchtypes.FactPayload{Statement: "D7 bootstrap active", Evidence: nil},
		"observe_proposer",
	)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	rep, err := orchtypes.NewUncertaintyReport("sess_test", []orchtypes.Observation{sysFact})
	if err != nil {
		t.Fatalf("setup report: %v", err)
	}
	got := uncertaintyReportSummary(rep, "fast")
	if strings.Contains(got, "D7 bootstrap") {
		t.Fatalf("CatSystem fact should not surface in Plan summary, got %q", got)
	}
}

// DM-20260706-009: long ObsFact.statement must be truncated to fit
// uncertaintyQuestionMaxLen (120 chars), matching the ObsUncertainty /
// ObsDeviation treatment.
func TestUncertaintyReportSummary_TruncatesLongFact(t *testing.T) {
	long := strings.Repeat("y", 200)
	fact, err := orchtypes.NewObservation(
		orchtypes.ObsFact, orchtypes.CatBusiness, 0.95,
		orchtypes.FactPayload{Statement: long, Evidence: nil},
		"observe_proposer",
	)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	rep, err := orchtypes.NewUncertaintyReport("sess_test", []orchtypes.Observation{fact})
	if err != nil {
		t.Fatalf("setup report: %v", err)
	}
	got := uncertaintyReportSummary(rep, "")
	if strings.Contains(got, long) {
		t.Fatalf("fact not truncated; got len=%d", len(got))
	}
	if !strings.Contains(got, "fact=") {
		t.Fatalf("expected fact= prefix, got %q", got)
	}
}

// DM-20260706-009: hasHighStrengthFact helper used by the Plan single-mode
// gate. Threshold semantics: ≥ threshold (inclusive). CatSystem ObsFact is
// never high-strength (filtered out).
func TestHasHighStrengthFact(t *testing.T) {
	bizFact, _ := orchtypes.NewObservation(
		orchtypes.ObsFact, orchtypes.CatBusiness, 0.99,
		orchtypes.FactPayload{Statement: "1+1=2"},
		"observe_proposer",
	)
	sysFact, _ := orchtypes.NewObservation(
		orchtypes.ObsFact, orchtypes.CatSystem, 0.99,
		orchtypes.FactPayload{Statement: "sys"},
		"observe_proposer",
	)
	weakBizFact, _ := orchtypes.NewObservation(
		orchtypes.ObsFact, orchtypes.CatBusiness, 0.5,
		orchtypes.FactPayload{Statement: "weak"},
		"observe_proposer",
	)
	repBiz, _ := orchtypes.NewUncertaintyReport("sess", []orchtypes.Observation{bizFact})
	repSys, _ := orchtypes.NewUncertaintyReport("sess", []orchtypes.Observation{sysFact})
	repWeak, _ := orchtypes.NewUncertaintyReport("sess", []orchtypes.Observation{weakBizFact})
	repEmpty, _ := orchtypes.NewUncertaintyReport("sess", nil)

	if !hasHighStrengthFact(repBiz, 0.9) {
		t.Fatalf("CatBusiness ObsFact @0.99 should be high-strength, got false")
	}
	if hasHighStrengthFact(repSys, 0.9) {
		t.Fatalf("CatSystem ObsFact must not trigger fast-path, got true")
	}
	if hasHighStrengthFact(repWeak, 0.9) {
		t.Fatalf("CatBusiness ObsFact @0.5 below threshold must not trigger fast-path")
	}
	if hasHighStrengthFact(repEmpty, 0.9) {
		t.Fatalf("empty report must not trigger fast-path")
	}
}
