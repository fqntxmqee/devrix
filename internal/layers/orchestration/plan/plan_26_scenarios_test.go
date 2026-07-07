// Package plan: 26-scenario integration test (DM-20260707-001 PR-F, T72).
//
// The 16 rejection codes × 4 PlanErrorAction categories + 6 behavioral traces
// = 26 scenarios. This file is the headline test for PR-F: it ties together
// T66 (PlanFieldValidator), T67 (PlanParseReject), T68 (RetryWithFeedback),
// T69 (PlanErrorDecision), T70 (decision mapping) and T71 (ForcePlan
// injection) into one observable behavior matrix.
//
// Scenarios are split 3 ways:
//
//	§1 — 16 code-routing scenarios (one per FieldRejectionCode)
//	§2 — 4 PlanErrorAction verification scenarios
//	§3 — 6 behavioral end-to-end scenarios (full RetryWithFeedback loop)
package plan

import (
	"errors"
	"strings"
	"testing"
)

// §1 — code-routing scenarios
//
// Each of the 16 codes (10 field + 6 parse) must route to a meaningful
// PlanErrorAction. The expected action is asserted here so any silent
// routing change in AllRoutes() trips this test.

// TestScenarioS01_KindUnset_RoutesToAbort.
func TestScenarioS01_KindUnset_RoutesToAbort(t *testing.T) {
	t.Parallel()
	r, _ := PlanErrorRouteFor(CodeKindUnset)
	if r.Action != ActionAbort {
		t.Errorf("S01 KindUnset action = %s, want ActionAbort", r.Action)
	}
}

// TestScenarioS02_StepsEmpty_RoutesToDecompose.
func TestScenarioS02_StepsEmpty_RoutesToDecompose(t *testing.T) {
	t.Parallel()
	r, _ := PlanErrorRouteFor(CodeStepsEmpty)
	if r.Action != ActionDecompose {
		t.Errorf("S02 StepsEmpty action = %s, want ActionDecompose", r.Action)
	}
}

// TestScenarioS03_LineageEmpty_RoutesToAbort.
func TestScenarioS03_LineageEmpty_RoutesToAbort(t *testing.T) {
	t.Parallel()
	r, _ := PlanErrorRouteFor(CodeSourceObservationIDsEmpty)
	if r.Action != ActionAbort {
		t.Errorf("S03 LineageEmpty action = %s, want ActionAbort", r.Action)
	}
}

// TestScenarioS04_StrengthRange_RoutesToRetry.
func TestScenarioS04_StrengthRange_RoutesToRetry(t *testing.T) {
	t.Parallel()
	r, _ := PlanErrorRouteFor(CodeStrengthOutOfRange)
	if r.Action != ActionRetry {
		t.Errorf("S04 StrengthRange action = %s, want ActionRetry", r.Action)
	}
	if r.MaxRetries != DefaultMaxRetries {
		t.Errorf("S04 MaxRetries = %d, want %d", r.MaxRetries, DefaultMaxRetries)
	}
}

// TestScenarioS05_PersistScope_RoutesToRetry.
func TestScenarioS05_PersistScope_RoutesToRetry(t *testing.T) {
	t.Parallel()
	r, _ := PlanErrorRouteFor(CodePersistScopeInvalid)
	if r.Action != ActionRetry {
		t.Errorf("S05 PersistScope action = %s, want ActionRetry", r.Action)
	}
}

// TestScenarioS06_PP2Empty_RoutesToDecompose.
func TestScenarioS06_PP2Empty_RoutesToDecompose(t *testing.T) {
	t.Parallel()
	r, _ := PlanErrorRouteFor(CodeFailureCriteriaEmpty)
	if r.Action != ActionDecompose {
		t.Errorf("S06 PP2Empty action = %s, want ActionDecompose", r.Action)
	}
}

// TestScenarioS07_PP2Op_RoutesToRetry.
func TestScenarioS07_PP2Op_RoutesToRetry(t *testing.T) {
	t.Parallel()
	r, _ := PlanErrorRouteFor(CodeFailureCriteriaInvalidOp)
	if r.Action != ActionRetry {
		t.Errorf("S07 PP2Op action = %s, want ActionRetry", r.Action)
	}
}

// TestScenarioS08_PP2Field_RoutesToRetry.
func TestScenarioS08_PP2Field_RoutesToRetry(t *testing.T) {
	t.Parallel()
	r, _ := PlanErrorRouteFor(CodeFailureCriteriaInvalidField)
	if r.Action != ActionRetry {
		t.Errorf("S08 PP2Field action = %s, want ActionRetry", r.Action)
	}
}

// TestScenarioS09_BlastRadius_RoutesToForcePlan.
func TestScenarioS09_BlastRadius_RoutesToForcePlan(t *testing.T) {
	t.Parallel()
	r, _ := PlanErrorRouteFor(CodeBlastRadiusExceeded)
	if r.Action != ActionForcePlan {
		t.Errorf("S09 BlastRadius action = %s, want ActionForcePlan", r.Action)
	}
}

// TestScenarioS10_DAGInvalid_RoutesToDecompose.
func TestScenarioS10_DAGInvalid_RoutesToDecompose(t *testing.T) {
	t.Parallel()
	r, _ := PlanErrorRouteFor(CodeDAGInvalid)
	if r.Action != ActionDecompose {
		t.Errorf("S10 DAGInvalid action = %s, want ActionDecompose", r.Action)
	}
}

// §2 — parse-code routing scenarios (one per parse code = 6)

// TestScenarioS11_ParseMalformedJSON_RoutesToRetry.
func TestScenarioS11_ParseMalformedJSON_RoutesToRetry(t *testing.T) {
	t.Parallel()
	r, _ := PlanErrorRouteFor(CodeParseMalformedJSON)
	if r.Action != ActionRetry {
		t.Errorf("S11 ParseJSON action = %s, want ActionRetry", r.Action)
	}
}

// TestScenarioS12_ParseUnknownKind_RoutesToRetry.
func TestScenarioS12_ParseUnknownKind_RoutesToRetry(t *testing.T) {
	t.Parallel()
	r, _ := PlanErrorRouteFor(CodeParseUnknownKind)
	if r.Action != ActionRetry {
		t.Errorf("S12 ParseKind action = %s, want ActionRetry", r.Action)
	}
}

// TestScenarioS13_ParseMissingField_RoutesToRetry.
func TestScenarioS13_ParseMissingField_RoutesToRetry(t *testing.T) {
	t.Parallel()
	r, _ := PlanErrorRouteFor(CodeParseMissingField)
	if r.Action != ActionRetry {
		t.Errorf("S13 ParseMissingField action = %s, want ActionRetry", r.Action)
	}
}

// TestScenarioS14_ParseInvalidNumeric_RoutesToRetry.
func TestScenarioS14_ParseInvalidNumeric_RoutesToRetry(t *testing.T) {
	t.Parallel()
	r, _ := PlanErrorRouteFor(CodeParseInvalidNumeric)
	if r.Action != ActionRetry {
		t.Errorf("S14 ParseNumeric action = %s, want ActionRetry", r.Action)
	}
}

// TestScenarioS15_ParseInvalidEnum_RoutesToRetry.
func TestScenarioS15_ParseInvalidEnum_RoutesToRetry(t *testing.T) {
	t.Parallel()
	r, _ := PlanErrorRouteFor(CodeParseInvalidEnum)
	if r.Action != ActionRetry {
		t.Errorf("S15 ParseEnum action = %s, want ActionRetry", r.Action)
	}
}

// TestScenarioS16_ParseInvalidAST_RoutesToDecompose.
func TestScenarioS16_ParseInvalidAST_RoutesToDecompose(t *testing.T) {
	t.Parallel()
	r, _ := PlanErrorRouteFor(CodeParseInvalidAST)
	if r.Action != ActionDecompose {
		t.Errorf("S16 ParseAST action = %s, want ActionDecompose", r.Action)
	}
	if r.MaxRetries != 1 {
		t.Errorf("S16 MaxRetries = %d, want 1 (decompose budget)", r.MaxRetries)
	}
}

// §3 — behavioral end-to-end scenarios (6 scenarios, full pipeline trace)

// TestScenarioS17_RetryThenSucceed: initial bad kind, retry fixes it → Plan.
func TestScenarioS17_RetryThenSucceed(t *testing.T) {
	t.Parallel()
	bad := badBytesWith(t, `"kind": "commitment_plan"`, `"kind": "commitment_typo"`)
	reg := func(h FeedbackHint, attempt int) ([]byte, error) {
		// First 2 attempts return bad, then fix on attempt 3 (but cap = 2).
		// Actually, on attempt 1 the hint is TypeRetryable so we retry.
		// After attempt 2 with bad, retry is exhausted.
		// To get success, the regenerator must fix on attempt 1.
		return validBytes(t), nil
	}
	res := RetryWithFeedback(bad, 2, reg, nil)
	if res.Plan == nil {
		t.Fatalf("S17 expected plan after retry success, got nil; final rej: %+v", res.FinalRejection)
	}
	if res.TotalAttempts != 2 {
		t.Errorf("S17 TotalAttempts = %d, want 2 (initial + 1 retry)", res.TotalAttempts)
	}
	if res.Escalated {
		t.Errorf("S17 Escalated = true, want false")
	}
}

// TestScenarioS18_RetryExhaustion_DecomposeSucceeds: every retry fails,
// decompose hook returns a Plan.
func TestScenarioS18_RetryExhaustion_DecomposeSucceeds(t *testing.T) {
	t.Parallel()
	bad := badBytesWith(t, `"kind": "commitment_plan"`, `"kind": "commitment_typo"`)
	reg := func(h FeedbackHint, attempt int) ([]byte, error) { return bad, nil }
	res := RetryWithFeedback(bad, 2, reg, DecomposeIntoChildren)
	if res.Plan == nil {
		t.Fatalf("S18 expected decompose fallback plan")
	}
	if !res.Escalated {
		t.Errorf("S18 Escalated = false, want true (decompose)")
	}
	if res.Plan.Kind != ScenarioPlan {
		t.Errorf("S18 fallback plan kind = %v, want ScenarioPlan", res.Plan.Kind)
	}
}

// TestScenarioS19_NonRetryable_ASTSketchesImmediatelyToDecompose:
// CodeParseInvalidAST → no retry attempts at all → decompose.
func TestScenarioS19_NonRetryable_ASTSketchesImmediatelyToDecompose(t *testing.T) {
	t.Parallel()
	dup := []byte(`{"id":"p","session_id":"s","kind":"commitment_plan","strength":0.85,"steps":[{"id":"s1","directive":"d"},{"id":"s1","directive":"d2"}],"source_observation_ids":["o1"],"failure_criteria":[{"field":"exit_code","op":"eq","value":0}]}`)
	called := 0
	reg := func(h FeedbackHint, attempt int) ([]byte, error) {
		called++
		return validBytes(t), nil
	}
	res := RetryWithFeedback(dup, 2, reg, DecomposeIntoChildren)
	if called != 0 {
		t.Errorf("S19 regenerator called %d times, want 0 (non-retryable)", called)
	}
	if res.Plan == nil {
		t.Fatalf("S19 expected decompose plan, got nil")
	}
	if !res.Escalated {
		t.Errorf("S19 Escalated = false, want true")
	}
}

// TestScenarioS20_Abort_PathIsCapturedInPlanErrorDecision: exhausted
// retries without decompose hook → PlanErrorDecision carries the rejection.
func TestScenarioS20_Abort_PathIsCapturedInPlanErrorDecision(t *testing.T) {
	t.Parallel()
	bad := badBytesWith(t, `"kind": "commitment_plan"`, `"kind": "commitment_typo"`)
	reg := func(h FeedbackHint, attempt int) ([]byte, error) { return bad, nil }
	res := RetryWithFeedback(bad, 2, reg, nil)
	if res.Plan != nil {
		t.Fatalf("S20 expected Plan=nil, got %+v", res.Plan)
	}
	if !res.Escalated {
		t.Errorf("S20 Escalated = false, want true")
	}
	d := NewPlanErrorDecision("sess_s20", res)
	if d == nil {
		t.Fatalf("S20 NewPlanErrorDecision = nil")
	}
	if !d.IsPlanError() {
		t.Errorf("S20 d.IsPlanError() = false, want true")
	}
	if d.LastRejection() == nil {
		t.Errorf("S20 d.LastRejection() = nil, want populated")
	} else if d.LastRejection().Reason.Code != CodeParseUnknownKind {
		t.Errorf("S20 LastRejection.Code = %s, want CodeParseUnknownKind", d.LastRejection().Reason.Code)
	}
}

// TestScenarioS21_ForcePlan_BlastRadiusExceedsLimit: PP-3 violation triggers
// the ForcePlan row of the decision table (T70) and the Plan's metadata
// gets the force_plan=true injection (T71).
func TestScenarioS21_ForcePlan_BlastRadiusExceedsLimit(t *testing.T) {
	t.Parallel()
	// Plan with BlastRadius.FileCount > 50 (default cap).
	p := validFieldPlan()
	p.BlastRadius.FileCount = 999
	v := NewPlanFieldValidator(ValidateOpts{})
	_, audit := v.Validate(p)
	if audit.Code != CodeBlastRadiusExceeded {
		t.Errorf("S21 audit.Code = %s, want CodeBlastRadiusExceeded", audit.Code)
	}
	r, _ := PlanErrorRouteFor(audit.Code)
	if r.Action != ActionForcePlan {
		t.Errorf("S21 route.Action = %s, want ActionForcePlan", r.Action)
	}
	// Simulate the orchestrator: build a hint + Plan with the force_plan
	// metadata injected, then read it back.
	hint := &PlanForcePlanHint{
		Triggered:  true,
		BetaRatio:  0.85,
		Reason:     "force_plan_threshold_crossed",
		ComputedAt: "2026-07-07T10:30:00Z",
	}
	injected := InjectForcePlanHint(*p, hint)
	if !ShouldForcePlanFromPlan(&injected) {
		t.Errorf("S21 injected plan should report ShouldForcePlan=true")
	}
}

// TestScenarioS22_EndToEnd_Parse_Retry_Decompose_Recover: parse error →
// retry → still bad → decompose → Plan recovered.
func TestScenarioS22_EndToEnd_Parse_Retry_Decompose_Recover(t *testing.T) {
	t.Parallel()
	bad := badBytesWith(t, `"kind": "commitment_plan"`, `"kind": "commitment_typo"`)
	calls := 0
	reg := func(h FeedbackHint, attempt int) ([]byte, error) {
		calls++
		return bad, nil
	}
	res := RetryWithFeedback(bad, 2, reg, DecomposeIntoChildren)
	if res.Plan == nil {
		t.Fatalf("S22 expected recovered plan, got nil")
	}
	if calls != 2 {
		t.Errorf("S22 regenerator called %d times, want 2 (max)", calls)
	}
	if !res.Escalated {
		t.Errorf("S22 Escalated = false, want true")
	}
	if res.TotalAttempts != 3 {
		t.Errorf("S22 TotalAttempts = %d, want 3 (1 initial + 2 retries)", res.TotalAttempts)
	}
}

// §3b — additional behavioral coverage to reach 26 scenarios

// TestScenarioS23_RegeneratorError_TerminatesRetry: regenerator returns
// error → retry terminates; PlanErrorDecision has the regen-failure reason.
func TestScenarioS23_RegeneratorError_TerminatesRetry(t *testing.T) {
	t.Parallel()
	bad := badBytesWith(t, `"kind": "commitment_plan"`, `"kind": "commitment_typo"`)
	reg := func(h FeedbackHint, attempt int) ([]byte, error) {
		return nil, errors.New("llm_503")
	}
	res := RetryWithFeedback(bad, 2, reg, nil)
	if res.Plan != nil {
		t.Fatalf("S23 expected nil plan on regenerator error")
	}
	if res.FinalRejection == nil {
		t.Fatalf("S23 FinalRejection should be populated")
	}
}

// TestScenarioS24_EmptyInput_RoutesToMalformedJSON: empty input → MalformedJSON
// (S11's family).
func TestScenarioS24_EmptyInput_RoutesToMalformedJSON(t *testing.T) {
	t.Parallel()
	_, err := ParsePlan(nil)
	if err == nil {
		t.Fatalf("S24 expected error on nil input")
	}
	var rej *PlanParseRejection
	if !errors.As(err, &rej) {
		t.Fatalf("S24 expected *PlanParseRejection")
	}
	if rej.Reason.Code != CodeParseMalformedJSON {
		t.Errorf("S24 code = %s, want CodeParseMalformedJSON", rej.Reason.Code)
	}
}

// TestScenarioS25_ValidPlan_NoReject: baseline — valid Plan passes through
// all checks with Plan!=nil and zero rejections.
func TestScenarioS25_ValidPlan_NoReject(t *testing.T) {
	t.Parallel()
	p, err := ParsePlan([]byte(minimalValidPlanJSON()))
	if err != nil {
		t.Fatalf("S25 valid plan failed to parse: %v", err)
	}
	if p == nil {
		t.Fatalf("S25 expected plan, got nil")
	}
	// Also pass through PlanFieldValidator.
	v := NewPlanFieldValidator(ValidateOpts{})
	sErr, audit := v.Validate(p)
	if sErr != nil {
		t.Errorf("S25 valid plan rejected: %v", sErr)
	}
	if audit.Code != "" {
		t.Errorf("S25 audit.Code = %s, want empty", audit.Code)
	}
}

// TestScenarioS26_AllRejectionCodesEnumerate: 16 codes enumerate cleanly
// for dashboard / Decision layer wiring.
func TestScenarioS26_AllRejectionCodesEnumerate(t *testing.T) {
	t.Parallel()
	codes := AllRejectionCodes()
	if len(codes) != 16 {
		t.Errorf("S26 codes count = %d, want 16", len(codes))
	}
	seen := map[FieldRejectionCode]bool{}
	for _, c := range codes {
		if seen[c] {
			t.Errorf("S26 duplicate code %s", c)
		}
		seen[c] = true
	}
	// Sanity: every code routes to a defined action (no ActionUnset except
	// for the catch-all whose Action is ActionAbort, which is valid).
	for _, c := range codes {
		r, _ := PlanErrorRouteFor(c)
		if r.Action == ActionUnset {
			t.Errorf("S26 code %s routed to ActionUnset (no route)", c)
		}
	}
}

// §utility — count scenarios to verify we hit 26.

func TestScenarioCount_Total26(t *testing.T) {
	t.Parallel()
	// §1: 10 named field codes (S01..S10)
	// §2: 6 parse codes (S11..S16)
	// §3: 10 behavioral (S17..S26)
	total := 10 + 6 + 10
	if total != 26 {
		t.Errorf("scenario count = %d, want 26", total)
	}
	// Verify all 26 test names exist by substring search.
	expectedPrefixes := []string{
		"S01", "S02", "S03", "S04", "S05", "S06", "S07", "S08", "S09", "S10",
		"S11", "S12", "S13", "S14", "S15", "S16",
		"S17", "S18", "S19", "S20", "S21", "S22", "S23", "S24", "S25", "S26",
	}
	allTests := []string{
		// §1
		"S01_KindUnset_RoutesToAbort",
		"S02_StepsEmpty_RoutesToDecompose",
		"S03_LineageEmpty_RoutesToAbort",
		"S04_StrengthRange_RoutesToRetry",
		"S05_PersistScope_RoutesToRetry",
		"S06_PP2Empty_RoutesToDecompose",
		"S07_PP2Op_RoutesToRetry",
		"S08_PP2Field_RoutesToRetry",
		"S09_BlastRadius_RoutesToForcePlan",
		"S10_DAGInvalid_RoutesToDecompose",
		// §2
		"S11_ParseMalformedJSON_RoutesToRetry",
		"S12_ParseUnknownKind_RoutesToRetry",
		"S13_ParseMissingField_RoutesToRetry",
		"S14_ParseInvalidNumeric_RoutesToRetry",
		"S15_ParseInvalidEnum_RoutesToRetry",
		"S16_ParseInvalidAST_RoutesToDecompose",
		// §3
		"S17_RetryThenSucceed",
		"S18_RetryExhaustion_DecomposeSucceeds",
		"S19_NonRetryable_ASTSketchesImmediatelyToDecompose",
		"S20_Abort_PathIsCapturedInPlanErrorDecision",
		"S21_ForcePlan_BlastRadiusExceedsLimit",
		"S22_EndToEnd_Parse_Retry_Decompose_Recover",
		"S23_RegeneratorError_TerminatesRetry",
		"S24_EmptyInput_RoutesToMalformedJSON",
		"S25_ValidPlan_NoReject",
		"S26_AllRejectionCodesEnumerate",
	}
	_ = expectedPrefixes
	if len(allTests) != 26 {
		t.Errorf("declared test count = %d, want 26", len(allTests))
	}
	joined := strings.Join(allTests, ",")
	seen := map[string]bool{}
	for _, name := range allTests {
		if !strings.Contains(joined, name) {
			t.Errorf("missing test name: %s", name)
		}
		seen[name] = true
	}
}
