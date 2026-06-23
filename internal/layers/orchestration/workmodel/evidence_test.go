package workmodel

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestEvidence_NewEvidence_ValidInputs(t *testing.T) {
	e, err := NewEvidence("criteria_met", 0.9, "plan_001")
	if err != nil {
		t.Fatalf("NewEvidence returned error: %v", err)
	}
	if e.Reason != "criteria_met" {
		t.Errorf("Reason = %q, want %q", e.Reason, "criteria_met")
	}
	if e.Confidence != 0.9 {
		t.Errorf("Confidence = %f, want 0.9", e.Confidence)
	}
	if e.SourceRef != "plan_001" {
		t.Errorf("SourceRef = %q, want %q", e.SourceRef, "plan_001")
	}
	if e.ExtractedAt.IsZero() {
		t.Error("ExtractedAt should be set automatically")
	}
}

func TestEvidence_NewEvidence_EmptyReason_FailsValidation(t *testing.T) {
	_, err := NewEvidence("", 0.5, "plan_001")
	if err == nil {
		t.Fatal("NewEvidence(\"\") should return error")
	}
	if !strings.Contains(err.Error(), "Reason is required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestEvidence_NewEvidence_EmptySourceRef_FailsValidation(t *testing.T) {
	_, err := NewEvidence("criteria_met", 0.5, "")
	if err == nil {
		t.Fatal("NewEvidence(empty SourceRef) should return error")
	}
	if !strings.Contains(err.Error(), "SourceRef is required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestEvidence_NewEvidence_ConfidenceOutOfRange_ClampedToFallback(t *testing.T) {
	// > 1.0 → fallback 0.5
	e1, err := NewEvidence("ok", 1.5, "plan_1")
	if err != nil {
		t.Fatalf("NewEvidence(1.5) returned error: %v", err)
	}
	if e1.Confidence != 0.5 {
		t.Errorf("Confidence 1.5 not clamped to fallback 0.5: got %f", e1.Confidence)
	}
	// < 0 → fallback 0.5
	e2, err := NewEvidence("ok", -0.3, "plan_1")
	if err != nil {
		t.Fatalf("NewEvidence(-0.3) returned error: %v", err)
	}
	if e2.Confidence != 0.5 {
		t.Errorf("Confidence -0.3 not clamped to fallback 0.5: got %f", e2.Confidence)
	}
}

func TestEvidence_NewEvidence_ConfidenceValidPreserved(t *testing.T) {
	e, err := NewEvidence("ok", 0.7, "plan_1")
	if err != nil {
		t.Fatalf("NewEvidence(0.7) returned error: %v", err)
	}
	if e.Confidence != 0.7 {
		t.Errorf("Confidence 0.7: got %f, want 0.7", e.Confidence)
	}
}

func TestEvidence_Validate_AllFieldsValid(t *testing.T) {
	e := Evidence{
		Reason:      "ok",
		Confidence:  0.8,
		SourceRef:   "plan_1",
		ExtractedAt: time.Now(),
	}
	if err := e.Validate(); err != nil {
		t.Errorf("Validate returned error for valid Evidence: %v", err)
	}
}

func TestEvidence_Validate_AllFieldsRequired(t *testing.T) {
	cases := []struct {
		name string
		ev   Evidence
	}{
		{"empty reason", Evidence{Confidence: 0.5, SourceRef: "p1"}},
		{"empty source ref", Evidence{Reason: "r", Confidence: 0.5}},
		{"confidence out of range (>1)", Evidence{Reason: "r", Confidence: 1.5, SourceRef: "p1"}},
		{"confidence out of range (<0)", Evidence{Reason: "r", Confidence: -0.1, SourceRef: "p1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.ev.Validate(); err == nil {
				t.Errorf("Validate should fail for case %q", tc.name)
			}
		})
	}
}

func TestEvidence_WithCounterexample(t *testing.T) {
	e, _ := NewEvidence("ok", 0.8, "plan_1")
	e2 := e.WithCounterexample("counter_001")
	if e.Counterexample != "" {
		t.Errorf("Original Evidence.Counterexample mutated: got %q", e.Counterexample)
	}
	if e2.Counterexample != "counter_001" {
		t.Errorf("New Evidence.Counterexample: got %q, want %q", e2.Counterexample, "counter_001")
	}
}

func TestEvidence_WithConfidence_Clamped(t *testing.T) {
	e, _ := NewEvidence("ok", 0.5, "plan_1")
	e1 := e.WithConfidence(2.0)
	if e1.Confidence != 0.5 {
		t.Errorf("WithConfidence(2.0) not clamped to 0.5: got %f", e1.Confidence)
	}
	e2 := e.WithConfidence(0.3)
	if e2.Confidence != 0.3 {
		t.Errorf("WithConfidence(0.3) = %f, want 0.3", e2.Confidence)
	}
}

func TestEvidence_MarshalJSON_WireFormat(t *testing.T) {
	e, _ := NewEvidence("ok", 0.8, "plan_1")
	e = e.WithCounterexample("counter_001")
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	// Round-trip back and verify.
	var decoded Evidence
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if decoded.Reason != e.Reason {
		t.Errorf("Reason drift: got %q, want %q", decoded.Reason, e.Reason)
	}
	if decoded.Confidence != e.Confidence {
		t.Errorf("Confidence drift: got %f, want %f", decoded.Confidence, e.Confidence)
	}
	if decoded.Counterexample != e.Counterexample {
		t.Errorf("Counterexample drift: got %q, want %q", decoded.Counterexample, e.Counterexample)
	}
	if decoded.SourceRef != e.SourceRef {
		t.Errorf("SourceRef drift: got %q, want %q", decoded.SourceRef, e.SourceRef)
	}
}

func TestEvidence_UnmarshalJSON_ZeroExtractedAt_DefaultsToNow(t *testing.T) {
	raw := `{"reason":"ok","confidence":0.8,"source_ref":"plan_1"}`
	var e Evidence
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if e.ExtractedAt.IsZero() {
		t.Error("ExtractedAt should default to time.Now() when zero in payload")
	}
}