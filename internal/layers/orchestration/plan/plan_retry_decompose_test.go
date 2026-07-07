// Package plan: RetryWithFeedback + DecomposeIntoChildren + PlanErrorDecision
// tests (DM-20260707-001 PR-F, T68+T69).
//
// Coverage matrix:
//
//   1. TestFeedbackForRejection_Retryable: MalformedJSON / UnknownKind /
//      MissingField / InvalidNumeric / InvalidEnum all map to Retryable=true.
//   2. TestFeedbackForRejection_NonRetryable: InvalidAST → Retryable=false.
//   3. TestRetryWithFeedback_HappyPath_NoRetry: valid input succeeds
//      without invoking regenerator.
//   4. TestRetryWithFeedback_RetriesUpToMax: regenerator returns valid bytes
//      on attempt 2; result.TotalAttempts = 3.
//   5. TestRetryWithFeedback_RetryExhaustion_WithDecompose: every attempt
//      fails → decompose hook fires once → Plan returned with Escalated=true.
//   6. TestRetryWithFeedback_RetryExhaustion_NoDecompose: every attempt
//      fails, no hook → Plan=nil, Escalated=true.
//   7. TestRetryWithFeedback_NonRetryableHint: stops at attempt 1.
//   8. TestRetryWithFeedback_RegeneratorError: regenerator returns error
//      → retry terminates; fallback path runs.
//   9. TestDecomposeIntoChildren_DedupesAndSorts: deduplicates by Code;
//      sorts by ID for determinism.
//  10. TestDecomposeIntoChildren_EmptyRejections: returns ErrDecomposeFailed.
//  11. TestPlanErrorDecision_IsPlanError: duck-type marker.
//  12. TestPlanErrorDecision_LastRejection: returns most recent.
//  13. TestNewPlanErrorDecision_NilOnSuccess: nil retry → nil decision.
//  14. TestNewPlanErrorDecision_BuildsContext: rejected retry → populated
//      decision with ≥1 rejection.
package plan

import (
	"errors"
	"strings"
	"testing"
)

// TestFeedbackForRejection_Retryable: the 5 retryable codes.
func TestFeedbackForRejection_Retryable(t *testing.T) {
	t.Parallel()
	cases := []FieldRejectionCode{
		CodeParseMalformedJSON,
		CodeParseUnknownKind,
		CodeParseMissingField,
		CodeParseInvalidNumeric,
		CodeParseInvalidEnum,
	}
	for _, c := range cases {
		c := c
		t.Run(string(c), func(t *testing.T) {
			t.Parallel()
			hint := FeedbackForRejection(&PlanParseRejection{Reason: ParseRejectReason{Code: c, Message: "x"}})
			if !hint.Retryable {
				t.Errorf("Code %s should be retryable", c)
			}
			if hint.Suggestion == "" {
				t.Errorf("Code %s should have a suggestion", c)
			}
			if hint.Code != c {
				t.Errorf("hint.Code = %s, want %s", hint.Code, c)
			}
		})
	}
}

// TestFeedbackForRejection_NonRetryable: InvalidAST only.
func TestFeedbackForRejection_NonRetryable(t *testing.T) {
	t.Parallel()
	hint := FeedbackForRejection(&PlanParseRejection{Reason: ParseRejectReason{Code: CodeParseInvalidAST, Message: "duplicate"}})
	if hint.Retryable {
		t.Errorf("InvalidAST should NOT be retryable")
	}
	if hint.Suggestion != "" {
		t.Errorf("InvalidAST should have empty Suggestion, got %q", hint.Suggestion)
	}
}

// TestFeedbackForRejection_Nil: defensive — nil input returns a sensible hint.
func TestFeedbackForRejection_Nil(t *testing.T) {
	t.Parallel()
	hint := FeedbackForRejection(nil)
	if hint.Code != CodeParseMalformedJSON {
		t.Errorf("nil hint.Code = %s, want CodeParseMalformedJSON", hint.Code)
	}
	if hint.Retryable {
		t.Errorf("nil hint should be retryable (defensive default)")
	}
}

// validBytes returns a clean minimal valid Plan JSON for tests.
func validBytes(t *testing.T) []byte {
	t.Helper()
	return []byte(minimalValidPlanJSON())
}

// TestRetryWithFeedback_HappyPath_NoRetry: initial parse succeeds.
func TestRetryWithFeedback_HappyPath_NoRetry(t *testing.T) {
	t.Parallel()
	calls := 0
	reg := func(h FeedbackHint, attempt int) ([]byte, error) {
		calls++
		return nil, nil
	}
	res := RetryWithFeedback(validBytes(t), 2, reg, nil)
	if res.Plan == nil {
		t.Fatalf("expected plan, got nil")
	}
	if res.TotalAttempts != 1 {
		t.Errorf("TotalAttempts = %d, want 1", res.TotalAttempts)
	}
	if calls != 0 {
		t.Errorf("regenerator calls = %d, want 0", calls)
	}
	if res.Escalated {
		t.Errorf("Escalated = true, want false")
	}
}

// badBytesWith returns minimalValidPlanJSON with one mutated field.
func badBytesWith(t *testing.T, search, replace string) []byte {
	t.Helper()
	out := strings.Replace(minimalValidPlanJSON(), search, replace, 1)
	if out == minimalValidPlanJSON() {
		t.Fatalf("badBytesWith: search %q not found", search)
	}
	return []byte(out)
}

// TestRetryWithFeedback_RetriesUpToMax: regenerator fixes on attempt 2.
func TestRetryWithFeedback_RetriesUpToMax(t *testing.T) {
	t.Parallel()
	bad := badBytesWith(t, `"kind": "commitment_plan"`, `"kind": "commitment_typo"`)
	calls := 0
	reg := func(h FeedbackHint, attempt int) ([]byte, error) {
		calls++
		if attempt >= 2 {
			return validBytes(t), nil
		}
		return bad, nil // keep rejecting
	}
	res := RetryWithFeedback(bad, 2, reg, nil)
	if res.Plan == nil {
		t.Fatalf("expected plan after retries, got nil; final rej: %+v", res.FinalRejection)
	}
	if res.TotalAttempts < 3 {
		t.Errorf("TotalAttempts = %d, want ≥3", res.TotalAttempts)
	}
	if calls != 2 {
		t.Errorf("regenerator calls = %d, want 2", calls)
	}
}

// TestRetryWithFeedback_RetryExhaustion_WithDecompose: every attempt fails,
// decompose hook returns a Plan.
func TestRetryWithFeedback_RetryExhaustion_WithDecompose(t *testing.T) {
	t.Parallel()
	bad := badBytesWith(t, `"kind": "commitment_plan"`, `"kind": "commitment_typo"`)
	reg := func(h FeedbackHint, attempt int) ([]byte, error) { return bad, nil }
	decomposeCalls := 0
	dec := func(rejections []*PlanParseRejection) (*Plan, error) {
		decomposeCalls++
		return DecomposeIntoChildren(rejections)
	}
	res := RetryWithFeedback(bad, 2, reg, dec)
	if res.Plan == nil {
		t.Fatalf("expected decompose plan, got nil")
	}
	if !res.Escalated {
		t.Errorf("Escalated = false, want true (decompose fallback used)")
	}
	if decomposeCalls != 1 {
		t.Errorf("decomposeCalls = %d, want 1", decomposeCalls)
	}
}

// TestRetryWithFeedback_RetryExhaustion_NoDecompose: every attempt fails,
// no hook → Plan=nil, Escalated=true.
func TestRetryWithFeedback_RetryExhaustion_NoDecompose(t *testing.T) {
	t.Parallel()
	bad := badBytesWith(t, `"kind": "commitment_plan"`, `"kind": "commitment_typo"`)
	reg := func(h FeedbackHint, attempt int) ([]byte, error) { return bad, nil }
	res := RetryWithFeedback(bad, 2, reg, nil)
	if res.Plan != nil {
		t.Errorf("expected Plan=nil, got %+v", res.Plan)
	}
	if !res.Escalated {
		t.Errorf("Escalated = false, want true")
	}
	if res.FinalRejection == nil {
		t.Errorf("FinalRejection = nil, want populated")
	}
	if res.Err != nil {
		t.Errorf("Err = %v, want nil (no decompose hook)", res.Err)
	}
}

// TestRetryWithFeedback_NonRetryableHint: AST violation → stop at attempt 1
// (no retry attempts even when maxRetries=2).
func TestRetryWithFeedback_NonRetryableHint(t *testing.T) {
	t.Parallel()
	// Duplicate step IDs → CodeParseInvalidAST
	dupSteps := `{
  "id": "p_test",
  "session_id": "s_test",
  "kind": "commitment_plan",
  "strength": 0.85,
  "steps": [{"id": "s1", "directive": "step 1"}, {"id": "s1", "directive": "step 2"}],
  "source_observation_ids": ["obs_1"],
  "failure_criteria": [{"field": "exit_code", "op": "eq", "value": 0}],
  "blast_radius": {"file_count": 1, "api_call_count": 1, "token_cost": 100, "persist_scope": "transient"}
}`
	regCalls := 0
	reg := func(h FeedbackHint, attempt int) ([]byte, error) {
		regCalls++
		return []byte(dupSteps), nil
	}
	res := RetryWithFeedback([]byte(dupSteps), 2, reg, nil)
	if regCalls != 0 {
		t.Errorf("regenerator calls = %d, want 0 (non-retryable)", regCalls)
	}
	if res.Plan != nil {
		t.Errorf("Plan should be nil after non-retryable failure")
	}
	if res.FinalRejection == nil || res.FinalRejection.Reason.Code != CodeParseInvalidAST {
		t.Errorf("FinalRejection.Code = %v, want CodeParseInvalidAST", res.FinalRejection)
	}
}

// TestRetryWithFeedback_RegeneratorError: regenerator itself fails.
func TestRetryWithFeedback_RegeneratorError(t *testing.T) {
	t.Parallel()
	bad := badBytesWith(t, `"kind": "commitment_plan"`, `"kind": "commitment_typo"`)
	reg := func(h FeedbackHint, attempt int) ([]byte, error) {
		return nil, errors.New("llm 503")
	}
	res := RetryWithFeedback(bad, 2, reg, nil)
	if res.Plan != nil {
		t.Errorf("expected Plan=nil after regenerator error")
	}
	if res.FinalRejection == nil {
		t.Errorf("FinalRejection should be populated (regenerator_failure)")
	}
}

// TestRetryWithFeedback_RegeneratorNil: no regenerator → no retries.
func TestRetryWithFeedback_RegeneratorNil(t *testing.T) {
	t.Parallel()
	bad := badBytesWith(t, `"kind": "commitment_plan"`, `"kind": "commitment_typo"`)
	res := RetryWithFeedback(bad, 2, nil, nil)
	if res.Plan != nil {
		t.Errorf("expected Plan=nil with nil regenerator")
	}
	if res.TotalAttempts != 1 {
		t.Errorf("TotalAttempts = %d, want 1", res.TotalAttempts)
	}
	if !res.Escalated {
		t.Errorf("Escalated = false, want true (no retry possible)")
	}
}

// TestDecomposeIntoChildren_DedupesAndSorts.
func TestDecomposeIntoChildren_DedupesAndSorts(t *testing.T) {
	t.Parallel()
	rejs := []*PlanParseRejection{
		{Reason: ParseRejectReason{Code: CodeParseUnknownKind, Message: "bad kind 1"}},
		{Reason: ParseRejectReason{Code: CodeParseMissingField, Message: "missing field"}},
		{Reason: ParseRejectReason{Code: CodeParseUnknownKind, Message: "bad kind 2 (dup)"}},
		{Reason: ParseRejectReason{Code: CodeParseInvalidEnum, Message: "bad enum"}},
	}
	p, err := DecomposeIntoChildren(rejs)
	if err != nil {
		t.Fatalf("DecomposeIntoChildren: %v", err)
	}
	// Dedupe by code → 3 unique codes → 3 steps.
	if len(p.Steps) != 3 {
		t.Errorf("Steps = %d, want 3 (deduped)", len(p.Steps))
	}
	// Verify ascending sort.
	for i := 1; i < len(p.Steps); i++ {
		if p.Steps[i-1].ID > p.Steps[i].ID {
			t.Errorf("Steps not sorted: %v", p.Steps)
			break
		}
	}
	if p.Kind != ScenarioPlan {
		t.Errorf("Kind = %v, want ScenarioPlan (decompose fallback)", p.Kind)
	}
}

// TestDecomposeIntoChildren_EmptyRejections: returns ErrDecomposeFailed.
func TestDecomposeIntoChildren_EmptyRejections(t *testing.T) {
	t.Parallel()
	_, err := DecomposeIntoChildren(nil)
	if !errors.Is(err, ErrDecomposeFailed) {
		t.Errorf("DecomposeIntoChildren(nil): err = %v, want ErrDecomposeFailed", err)
	}
	_, err = DecomposeIntoChildren([]*PlanParseRejection{})
	if !errors.Is(err, ErrDecomposeFailed) {
		t.Errorf("DecomposeIntoChildren([]): err = %v, want ErrDecomposeFailed", err)
	}
}

// TestDecomposeIntoChildren_SkipsNil.
func TestDecomposeIntoChildren_SkipsNil(t *testing.T) {
	t.Parallel()
	rejs := []*PlanParseRejection{
		nil,
		{Reason: ParseRejectReason{Code: CodeParseUnknownKind, Message: "only"}},
		nil,
	}
	p, err := DecomposeIntoChildren(rejs)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(p.Steps) != 1 {
		t.Errorf("Steps = %d, want 1 (nil-padded rejs)", len(p.Steps))
	}
}

// TestPlanErrorDecision_IsPlanError: duck-typing marker.
func TestPlanErrorDecision_IsPlanError(t *testing.T) {
	t.Parallel()
	var d *PlanErrorDecision
	if d.IsPlanError() {
		t.Errorf("nil PlanErrorDecision should report IsPlanError=false")
	}
	d = &PlanErrorDecision{}
	if !d.IsPlanError() {
		t.Errorf("non-nil PlanErrorDecision should report IsPlanError=true")
	}
}

// TestPlanErrorDecision_LastRejection.
func TestPlanErrorDecision_LastRejection(t *testing.T) {
	t.Parallel()
	first := &PlanParseRejection{Reason: ParseRejectReason{Code: CodeParseMalformedJSON}}
	last := &PlanParseRejection{Reason: ParseRejectReason{Code: CodeParseUnknownKind}}
	d := &PlanErrorDecision{Rejections: []*PlanParseRejection{first, last}}
	if d.LastRejection() != last {
		t.Errorf("LastRejection should return last appended")
	}
	var empty *PlanErrorDecision
	if empty.LastRejection() != nil {
		t.Errorf("nil decision LastRejection should be nil")
	}
	d2 := &PlanErrorDecision{Rejections: nil}
	if d2.LastRejection() != nil {
		t.Errorf("empty-rejection decision LastRejection should be nil")
	}
}

// TestNewPlanErrorDecision_NilOnSuccess: nil retry → nil decision.
func TestNewPlanErrorDecision_NilOnSuccess(t *testing.T) {
	t.Parallel()
	res := RetryResult{Plan: &Plan{}}
	d := NewPlanErrorDecision("sess_x", res)
	if d != nil {
		t.Errorf("NewPlanErrorDecision on success = %+v, want nil", d)
	}
}

// TestNewPlanErrorDecision_BuildsContext: rejected retry → populated decision.
func TestNewPlanErrorDecision_BuildsContext(t *testing.T) {
	t.Parallel()
	bad := badBytesWith(t, `"kind": "commitment_plan"`, `"kind": "commitment_typo"`)
	reg := func(h FeedbackHint, attempt int) ([]byte, error) { return bad, nil }
	res := RetryWithFeedback(bad, 2, reg, nil)
	if res.FinalRejection == nil {
		t.Fatalf("expected rejected retry")
	}
	d := NewPlanErrorDecision("sess_err", res)
	if d == nil {
		t.Fatalf("NewPlanErrorDecision = nil, want populated")
	}
	if d.SessionID != "sess_err" {
		t.Errorf("SessionID = %s, want sess_err", d.SessionID)
	}
	if len(d.Rejections) == 0 {
		t.Errorf("Rejections empty, want ≥1")
	}
	// After dedup-by-code, LastRejection().Reason.Code should still match
	// retry.FinalRejection.Reason.Code (the same code persists).
	lastCode := d.LastRejection().Reason.Code
	finalCode := res.FinalRejection.Reason.Code
	if lastCode != finalCode {
		t.Errorf("LastRejection().Code = %s, want %s (post-dedup)", lastCode, finalCode)
	}
}
