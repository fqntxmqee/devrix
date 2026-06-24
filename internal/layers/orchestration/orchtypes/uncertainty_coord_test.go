package orchtypes

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

// fromVerifierForTest wraps FromVerifierTyped with string-kind parsing so
// tests can keep table-driven string fixtures (mirrors the deleted string
// shim FromVerifier signature, but routes through the typed enum path).
func fromVerifierForTest(t *testing.T, verdict string, confidence float64, reason string, anomaly bool) UncertaintyCoord {
	t.Helper()
	kind, err := types.ParseVerdictKind(verdict)
	if err != nil {
		t.Fatalf("ParseVerdictKind(%q): %v", verdict, err)
	}
	c, err := FromVerifierTyped(kind, confidence, reason, anomaly)
	if err != nil {
		t.Fatalf("FromVerifierTyped(%q, %.3f, %q, %v): %v", verdict, confidence, reason, anomaly, err)
	}
	return c
}

func TestNewUncertaintyCoord_ClampsValue(t *testing.T) {
	c := NewUncertaintyCoord(1.5)
	if c.Value != 1.0 {
		t.Errorf("Value = %.3f, want 1.0", c.Value)
	}
	c = NewUncertaintyCoord(-0.3)
	if c.Value != 0.0 {
		t.Errorf("Value = %.3f, want 0.0", c.Value)
	}
	c = NewUncertaintyCoord(0.42)
	if c.Value != 0.42 {
		t.Errorf("Value = %.3f, want 0.42", c.Value)
	}
}

func TestFromVerifier_VerdictKinds(t *testing.T) {
	tests := []struct {
		verdict    string
		wantValue  float64
		confidence float64
		reason     string
		anomaly    bool
	}{
		{"pass", 0.0, 0.9, "all_criteria_met", false},
		{"partial", 0.4, 0.7, "some_steps_failed", false},
		{"indeterminate", 0.7, 0.3, "verifier_parse_failure", false},
		{"fail", 0.9, 0.95, "criteria_missed", false},
		{"pass", 0.95, 0.9, "but_catastrophic", true}, // system anomaly overrides to 0.95
	}
	for _, tt := range tests {
		t.Run(tt.verdict+"_anomaly="+boolStr(tt.anomaly), func(t *testing.T) {
			c := fromVerifierForTest(t, tt.verdict, tt.confidence, tt.reason, tt.anomaly)
			if c.Value != tt.wantValue {
				t.Errorf("Value = %.3f, want %.3f", c.Value, tt.wantValue)
			}
			if c.Confidence != tt.confidence {
				t.Errorf("Confidence = %.3f, want %.3f", c.Confidence, tt.confidence)
			}
			if c.Reason != tt.reason {
				t.Errorf("Reason = %s, want %s", c.Reason, tt.reason)
			}
			if !c.FromVerifier {
				t.Error("FromVerifier should be true")
			}
		})
	}
}

func TestUncertaintyCoord_WithMethods(t *testing.T) {
	c := NewUncertaintyCoord(0.5)
	c2 := c.WithValue(0.8)
	if c.Value != 0.5 {
		t.Errorf("original mutated: Value=%.2f, want 0.5", c.Value)
	}
	if c2.Value != 0.8 {
		t.Errorf("WithValue: Value=%.2f, want 0.8", c2.Value)
	}
	c3 := c2.WithReason("because")
	if c3.Reason != "because" {
		t.Errorf("WithReason: %s", c3.Reason)
	}
	c4 := c3.WithSideEffect(SideEffectInflight)
	if c4.SideEffectStatus != SideEffectInflight {
		t.Errorf("WithSideEffect: %s", c4.SideEffectStatus)
	}
}

func TestUncertaintyCoord_Validate(t *testing.T) {
	c := NewUncertaintyCoord(0.5)
	if err := c.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
	c2 := UncertaintyCoord{Value: 2.0}
	if err := c2.Validate(); err == nil {
		t.Error("expected error for Value > 1")
	}
	c3 := UncertaintyCoord{Value: 0.5, Confidence: -0.1}
	if err := c3.Validate(); err == nil {
		t.Error("expected error for Confidence < 0")
	}
}

func TestUncertaintyCoord_JSON_Phase1ShapeStillWorks(t *testing.T) {
	// Phase 1 baseline JSON: just {"value": 0.5, "updated_at": "..."}.
	// Our Marshal must keep that shape (omitempty for new fields).
	c := NewUncertaintyCoord(0.5)
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// Roundtrip with the legacy shape (no new fields).
	var legacy struct {
		Value     float64 `json:"value"`
		UpdatedAt string  `json:"updated_at"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		t.Fatalf("legacy Unmarshal: %v", err)
	}
	if legacy.Value != 0.5 {
		t.Errorf("legacy value = %.3f, want 0.5", legacy.Value)
	}
}

func TestUncertaintyCoord_JSON_RoundTrip_NewFields(t *testing.T) {
	c := fromVerifierForTest(t, "fail", 0.9, "criteria_missed", false)
	c = c.WithSideEffect(SideEffectRolledBack)
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got UncertaintyCoord
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !got.Equal(c) {
		t.Errorf("Roundtrip mismatch:\n got=%+v\n want=%+v", got, c)
	}
}

func TestUncertaintyCoord_JSON_OmitsEmptyNewFields(t *testing.T) {
	c := NewUncertaintyCoord(0.5)
	data, _ := json.Marshal(c)
	s := string(data)
	// New optional fields must NOT appear in JSON when empty.
	if contains(s, "from_verifier") {
		t.Error("from_verifier should be omitted when false")
	}
	if contains(s, "side_effect_status") {
		t.Error("side_effect_status should be omitted when empty")
	}
	if contains(s, "confidence") {
		t.Error("confidence should be omitted when 0")
	}
	if contains(s, "reason") {
		t.Error("reason should be omitted when empty")
	}
}

func TestUncertaintyCoord_IsColdStart(t *testing.T) {
	c1 := NewUncertaintyCoord(0.5)
	if !c1.IsColdStart() {
		t.Error("default 0.5 coord should be cold-start")
	}
	c2 := NewUncertaintyCoord(0.6)
	if c2.IsColdStart() {
		t.Error("0.6 coord should not be cold-start")
	}
	c3 := fromVerifierForTest(t, "pass", 0.9, "x", false)
	if c3.IsColdStart() {
		t.Error("verifier-derived coord should not be cold-start")
	}
	c4 := NewUncertaintyCoord(0.5).WithReason("explicit")
	if c4.IsColdStart() {
		t.Error("coord with explicit reason should not be cold-start")
	}
}

func TestUncertaintyCoord_NaN_ClampsToHalf(t *testing.T) {
	c := NewUncertaintyCoord(math.NaN())
	if c.Value != 0.5 {
		t.Errorf("NaN should clamp to 0.5, got %.3f", c.Value)
	}
}

// ⭐ RF.3.3 C3: FromVerifier must FAIL-FAST on unknown verdict kinds so
// the ORCH_COORD_VERDICT_7004 error code actually fires and noisy upstream
// typos surface immediately rather than being silently coerced to the
// 0.5 neutral default.
func TestUncertaintyCoord_FromVerifierTyped_UnknownKind(t *testing.T) {
	// ⭐ RF.3.3 C3: FromVerifierTyped must FAIL-FAST on unknown verdict
	// kinds so the ORCH_COORD_VERDICT_7004 error code actually fires and
	// noisy upstream typos surface immediately rather than being silently
	// coerced to the 0.5 neutral default.
	_, err := FromVerifierTyped(99, 0.9, "typo", false)
	if err == nil {
		t.Fatalf("expected error for unknown verdict kind 99")
	}
	if !errors.Is(err, ErrUncertaintyCoordInvalidVerdictKind) {
		t.Errorf("errors.Is should match sentinel, got %v", err)
	}
	// The error message should echo back the bad kind so logs are
	// traceable to the offending call site.
	if !strings.Contains(err.Error(), "VerdictKind(99)") {
		t.Errorf("error message should echo the bad verdict kind, got: %s", err.Error())
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
