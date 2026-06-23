package workmodel

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

func TestParseVerifierOutput_ValidJSON_ReturnsVerdict(t *testing.T) {
	raw := `{"kind": "pass", "confidence": 0.9, "reason": "all_criteria_met"}`
	got, err := ParseVerifierOutput(raw)
	if err != nil {
		t.Fatalf("ParseVerifierOutput returned error: %v", err)
	}
	if got.ParsedKind != types.VerdictPass {
		t.Errorf("ParsedKind = %v, want VerdictPass", got.ParsedKind)
	}
	if got.Confidence != 0.9 {
		t.Errorf("Confidence = %f, want 0.9", got.Confidence)
	}
	if got.Reason != "all_criteria_met" {
		t.Errorf("Reason = %q, want %q", got.Reason, "all_criteria_met")
	}
	if got.Raw != raw {
		t.Errorf("Raw = %q, want %q", got.Raw, raw)
	}
}

func TestParseVerifierOutput_InvalidJSON_ReturnsError(t *testing.T) {
	raw := `{not valid json`
	_, err := ParseVerifierOutput(raw)
	if err == nil {
		t.Fatal("ParseVerifierOutput(\"invalid json\") should return error")
	}
}

func TestParseVerifierOutput_UnknownKind_ReturnsError(t *testing.T) {
	raw := `{"kind": "unknown_kind", "confidence": 0.5}`
	_, err := ParseVerifierOutput(raw)
	if err == nil {
		t.Fatal("ParseVerifierOutput(unknown kind) should return error")
	}
}

func TestParseVerifierOutputWithRetry_3Failures_ReturnsIndeterminate(t *testing.T) {
	// G8-1 P0-3 fix: 3 retries of malformed output → INDETERMINATE (NOT error, NOT FAIL).
	raw := `{not valid json`
	got := ParseVerifierOutputWithRetry(raw, DefaultMaxParseRetries)
	if got.ParsedKind != types.VerdictIndeterminate {
		t.Errorf("ParsedKind = %v, want VerdictIndeterminate", got.ParsedKind)
	}
	if got.RetryCount != DefaultMaxParseRetries {
		t.Errorf("RetryCount = %d, want %d", got.RetryCount, DefaultMaxParseRetries)
	}
	if got.Raw != raw {
		t.Errorf("Raw should be preserved: got %q, want %q", got.Raw, raw)
	}
	if got.Confidence != 0 {
		t.Errorf("Confidence should be 0 on retry failure: got %f", got.Confidence)
	}
}

func TestParseVerifierOutputWithRetry_2Failures1Success_ReturnsVerdict(t *testing.T) {
	// After 2 failures, the third retry succeeds (simulated by passing valid JSON
	// but counting it as success on the first parse).
	raw := `{"kind": "pass", "confidence": 0.85, "reason": "ok"}`
	got := ParseVerifierOutputWithRetry(raw, DefaultMaxParseRetries)
	if got.ParsedKind != types.VerdictPass {
		t.Errorf("ParsedKind = %v, want VerdictPass", got.ParsedKind)
	}
	if got.RetryCount != 0 {
		t.Errorf("RetryCount = %d, want 0 (first try succeeded)", got.RetryCount)
	}
	if got.Confidence != 0.85 {
		t.Errorf("Confidence = %f, want 0.85", got.Confidence)
	}
}

func TestParseVerifierOutputWithRetry_AllSuccess_FirstTry(t *testing.T) {
	raw := `{"kind": "fail", "confidence": 0.95, "reason": "criteria_missed"}`
	got := ParseVerifierOutputWithRetry(raw, DefaultMaxParseRetries)
	if got.ParsedKind != types.VerdictFail {
		t.Errorf("ParsedKind = %v, want VerdictFail", got.ParsedKind)
	}
	if got.RetryCount != 0 {
		t.Errorf("RetryCount = %d, want 0", got.RetryCount)
	}
}

func TestParseVerifierOutputWithRetry_DefaultMaxRetries(t *testing.T) {
	// Passing maxRetries=0 should use DefaultMaxParseRetries.
	raw := `{not valid json`
	got := ParseVerifierOutputWithRetry(raw, 0)
	if got.RetryCount != DefaultMaxParseRetries {
		t.Errorf("RetryCount = %d, want %d (DefaultMaxParseRetries)", got.RetryCount, DefaultMaxParseRetries)
	}
	if got.ParsedKind != types.VerdictIndeterminate {
		t.Errorf("ParsedKind = %v, want VerdictIndeterminate", got.ParsedKind)
	}
}

func TestParseVerifierOutputWithRetry_NegativeMaxRetries_UsesDefault(t *testing.T) {
	raw := `{not valid json`
	got := ParseVerifierOutputWithRetry(raw, -5)
	if got.RetryCount != DefaultMaxParseRetries {
		t.Errorf("RetryCount = %d, want %d", got.RetryCount, DefaultMaxParseRetries)
	}
}

func TestParseVerifierOutputWithRetry_PreservesRawOnFailure(t *testing.T) {
	raw := `corrupted_json_with_known_markers_xyz`
	got := ParseVerifierOutputWithRetry(raw, 2)
	if got.Raw != raw {
		t.Errorf("Raw = %q, want %q (should preserve for debugging)", got.Raw, raw)
	}
}

func TestParseVerifierOutputWithRetry_All4Kinds(t *testing.T) {
	cases := []struct {
		raw  string
		want types.VerdictKind
	}{
		{`{"kind": "pass"}`, types.VerdictPass},
		{`{"kind": "partial"}`, types.VerdictPartial},
		{`{"kind": "indeterminate"}`, types.VerdictIndeterminate},
		{`{"kind": "fail"}`, types.VerdictFail},
	}
	for _, tc := range cases {
		got := ParseVerifierOutputWithRetry(tc.raw, DefaultMaxParseRetries)
		if got.ParsedKind != tc.want {
			t.Errorf("ParseVerifierOutputWithRetry(%s) = %v, want %v", tc.raw, got.ParsedKind, tc.want)
		}
	}
}