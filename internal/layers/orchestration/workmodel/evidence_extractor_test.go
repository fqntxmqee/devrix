package workmodel

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

func TestLLMEvidenceExtractor_ValidListOutput_Extracts3Fields(t *testing.T) {
	raw := `{
		"evidences": [
			{"reason": "step_1_passed", "confidence": 0.95, "counterexample": ""},
			{"reason": "step_2_criteria_met", "confidence": 0.88, "counterexample": "edge_case_x"}
		]
	}`
	ext := NewLLMEvidenceExtractor()
	v := VerifierOutput{Raw: raw, ParsedKind: types.VerdictPass, SourceID: "verifier_1"}
	evs, err := ext.Extract(context.Background(), v)
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if len(evs) != 2 {
		t.Fatalf("got %d Evidence records, want 2", len(evs))
	}
	if evs[0].Reason != "step_1_passed" {
		t.Errorf("evs[0].Reason = %q, want %q", evs[0].Reason, "step_1_passed")
	}
	if evs[0].Confidence != 0.95 {
		t.Errorf("evs[0].Confidence = %f, want 0.95", evs[0].Confidence)
	}
	if evs[0].Counterexample != "" {
		t.Errorf("evs[0].Counterexample should be empty: got %q", evs[0].Counterexample)
	}
	if evs[1].Counterexample != "edge_case_x" {
		t.Errorf("evs[1].Counterexample = %q, want %q", evs[1].Counterexample, "edge_case_x")
	}
	if evs[0].SourceRef != "verifier_1" {
		t.Errorf("evs[0].SourceRef = %q, want %q", evs[0].SourceRef, "verifier_1")
	}
}

func TestLLMEvidenceExtractor_ValidSingleOutput_FallbackShape(t *testing.T) {
	raw := `{"reason": "single_evidence", "confidence": 0.7}`
	ext := NewLLMEvidenceExtractor()
	v := VerifierOutput{Raw: raw, ParsedKind: types.VerdictPass, SourceID: "verifier_2"}
	evs, err := ext.Extract(context.Background(), v)
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if len(evs) != 1 {
		t.Fatalf("got %d Evidence, want 1", len(evs))
	}
	if evs[0].Reason != "single_evidence" {
		t.Errorf("Reason = %q, want %q", evs[0].Reason, "single_evidence")
	}
	if evs[0].Confidence != 0.7 {
		t.Errorf("Confidence = %f, want 0.7", evs[0].Confidence)
	}
	if evs[0].SourceRef != "verifier_2" {
		t.Errorf("SourceRef = %q, want %q", evs[0].SourceRef, "verifier_2")
	}
}

func TestLLMEvidenceExtractor_MalformedOutput_ReturnsError(t *testing.T) {
	raw := `not a json`
	ext := NewLLMEvidenceExtractor()
	v := VerifierOutput{Raw: raw, ParsedKind: types.VerdictFail, SourceID: "verifier_3"}
	_, err := ext.Extract(context.Background(), v)
	if err == nil {
		t.Fatal("Extract should fail on malformed JSON without list or single reason field")
	}
	if !strings.Contains(err.Error(), "malformed LLM output") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLLMEvidenceExtractor_EmptyOutput_ReturnsEmptySlice(t *testing.T) {
	ext := NewLLMEvidenceExtractor()
	v := VerifierOutput{Raw: "", ParsedKind: types.VerdictIndeterminate, SourceID: "verifier_4"}
	evs, err := ext.Extract(context.Background(), v)
	if err != nil {
		t.Fatalf("Extract should not error on empty raw: %v", err)
	}
	if len(evs) != 0 {
		t.Errorf("got %d Evidence, want 0 (empty)", len(evs))
	}
}

func TestLLMEvidenceExtractor_NoSourceID_UsesFallback(t *testing.T) {
	raw := `{"reason": "ok", "confidence": 0.5}`
	ext := NewLLMEvidenceExtractor()
	v := VerifierOutput{Raw: raw, ParsedKind: types.VerdictPass} // SourceID empty
	evs, err := ext.Extract(context.Background(), v)
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if len(evs) != 1 {
		t.Fatalf("got %d Evidence, want 1", len(evs))
	}
	if !strings.HasPrefix(evs[0].SourceRef, "verifier_output") {
		t.Errorf("SourceRef fallback should start with \"verifier_output\", got %q", evs[0].SourceRef)
	}
}

func TestLLMEvidenceExtractor_EmptyReasonInList_FailsExtraction(t *testing.T) {
	raw := `{"evidences": [{"reason": "", "confidence": 0.5}]}`
	ext := NewLLMEvidenceExtractor()
	v := VerifierOutput{Raw: raw, SourceID: "v1"}
	_, err := ext.Extract(context.Background(), v)
	if err == nil {
		t.Fatal("Extract should fail when evidence list contains empty Reason")
	}
	if !strings.Contains(err.Error(), "Reason is required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLLMEvidenceExtractor_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	ext := NewLLMEvidenceExtractor()
	v := VerifierOutput{Raw: `{"reason":"x","confidence":0.5}`, SourceID: "v1"}
	_, err := ext.Extract(ctx, v)
	if err == nil {
		t.Fatal("Extract should fail on cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error should wrap context.Canceled: %v", err)
	}
}

func TestStubEvidenceExtractor_ReturnsFixedEvidence(t *testing.T) {
	fixed, _ := NewEvidence("stub_reason", 0.6, "stub_source")
	stub := NewStubEvidenceExtractor([]Evidence{fixed}, nil)
	evs, err := stub.Extract(context.Background(), VerifierOutput{})
	if err != nil {
		t.Fatalf("Stub.Extract returned error: %v", err)
	}
	if len(evs) != 1 {
		t.Fatalf("got %d Evidence, want 1", len(evs))
	}
	if evs[0].Reason != "stub_reason" {
		t.Errorf("Reason = %q, want %q", evs[0].Reason, "stub_reason")
	}
}

func TestStubEvidenceExtractor_ReturnsConfiguredError(t *testing.T) {
	configErr := errors.New("stub_error")
	stub := NewStubEvidenceExtractor(nil, configErr)
	_, err := stub.Extract(context.Background(), VerifierOutput{})
	if err == nil {
		t.Fatal("Stub should return configured error")
	}
	if err.Error() != "stub_error" {
		t.Errorf("error = %q, want %q", err.Error(), "stub_error")
	}
}

func TestEvidenceExtractor_Validate_EmptyListFails(t *testing.T) {
	ext := NewLLMEvidenceExtractor()
	if err := ext.Validate(nil); err == nil {
		t.Error("Validate(nil) should fail")
	}
	if err := ext.Validate([]Evidence{}); err == nil {
		t.Error("Validate([]) should fail")
	}
}

func TestEvidenceExtractor_Validate_EmptyReasonInListFails(t *testing.T) {
	ext := NewLLMEvidenceExtractor()
	ev1, _ := NewEvidence("good", 0.5, "s1")
	ev2, _ := NewEvidence("bad", 0.5, "s2")
	ev2.Reason = "" // mutate to empty
	evs := []Evidence{ev1, ev2}
	err := ext.Validate(evs)
	if err == nil {
		t.Error("Validate should fail when any Evidence has empty Reason")
	}
	if !strings.Contains(err.Error(), "evidence[1]") {
		t.Errorf("error should reference evidence[1]: %v", err)
	}
}

func TestEvidenceExtractor_Validate_AllValid(t *testing.T) {
	ext := NewLLMEvidenceExtractor()
	ev1, _ := NewEvidence("good1", 0.5, "s1")
	ev2, _ := NewEvidence("good2", 0.6, "s2")
	if err := ext.Validate([]Evidence{ev1, ev2}); err != nil {
		t.Errorf("Validate returned error for valid list: %v", err)
	}
}