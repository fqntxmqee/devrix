package interfaces

import (
	"errors"
	"testing"

	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// TestPessimisticCommitGuard_InterfaceCompiles — static check that the
// PessimisticCommitGuard interface is satisfiable by a zero-dependency
// fake. PR-B design.md §4.1 requires the interface signature to be stable
// across all consumers; this test guards the shape.
func TestPessimisticCommitGuard_InterfaceCompiles(t *testing.T) {
	var _ PessimisticCommitGuard = (*guardFake)(nil)
}

type guardFake struct{}

func (guardFake) Evaluate(_ *TaskSpec, _ *TaskReport, _ ConvergenceBudget) (bool, string, error) {
	return true, "", nil
}
func (guardFake) ResolveFallback(_ *TaskReport) (FallbackPolicy, string) {
	return FallbackPessimistic, ""
}
func (guardFake) BuildMVPArtifact(_ *TaskReport, reason string) MVPArtifact {
	return MVPArtifact{Output: "x", RiskWarnings: []string{reason}, Trigger: reason}
}

// TestNewORCHPessimisticTriggeredError — D7-S18-A11-T01 sub-case 1.
func TestNewORCHPessimisticTriggeredError(t *testing.T) {
	e := NewORCHPessimisticTriggeredError()
	if e == nil {
		t.Fatal("returned nil")
	}
	if e.Code != "ORCH_PESSIMISTIC_TRIGGERED_7110" {
		t.Errorf("Code = %q, want ORCH_PESSIMISTIC_TRIGGERED_7110", e.Code)
	}
	if !errors.Is(e, ErrORCHPessimisticTriggered) {
		t.Errorf("error chain missing ErrORCHPessimisticTriggered: %v", e)
	}
	var sErr *sharederrors.SentinelError
	if !errors.As(e, &sErr) {
		t.Errorf("error is not a *sharederrors.SentinelError: %T", e)
	} else if sErr.Message == "" {
		t.Error("Message should be non-empty")
	}
}

// TestNewORCHPessimisticMVPEmptyError — D7-S18-A11-T01 sub-case 2.
func TestNewORCHPessimisticMVPEmptyError(t *testing.T) {
	e := NewORCHPessimisticMVPEmptyError()
	if e.Code != "ORCH_PESSIMISTIC_MVP_EMPTY_7111" {
		t.Errorf("Code = %q, want ORCH_PESSIMISTIC_MVP_EMPTY_7111", e.Code)
	}
	if !errors.Is(e, ErrORCHPessimisticMVPEmpty) {
		t.Errorf("error chain missing ErrORCHPessimisticMVPEmpty: %v", e)
	}
}

// TestNewORCHFallbackRuleInvalidError — D7-S18-A11-T01 sub-case 3.
func TestNewORCHFallbackRuleInvalidError(t *testing.T) {
	e := NewORCHFallbackRuleInvalidError()
	if e.Code != "ORCH_FALLBACK_RULE_INVALID_7112" {
		t.Errorf("Code = %q, want ORCH_FALLBACK_RULE_INVALID_7112", e.Code)
	}
	if !errors.Is(e, ErrORCHFallbackRuleInvalid) {
		t.Errorf("error chain missing ErrORCHFallbackRuleInvalid: %v", e)
	}
}

// TestNewORCHFallbackAbortTimeoutError — D7-S18-A11-T01 sub-case 4.
func TestNewORCHFallbackAbortTimeoutError(t *testing.T) {
	e := NewORCHFallbackAbortTimeoutError()
	if e.Code != "ORCH_FALLBACK_ABORT_TIMEOUT_7113" {
		t.Errorf("Code = %q, want ORCH_FALLBACK_ABORT_TIMEOUT_7113", e.Code)
	}
	if !errors.Is(e, ErrORCHFallbackAbortTimeout) {
		t.Errorf("error chain missing ErrORCHFallbackAbortTimeout: %v", e)
	}
}

// TestTriggerConstants_Stable — the 5 trigger names are stable for span
// attributes and downstream filtering; this test guards accidental renames.
func TestTriggerConstants_Stable(t *testing.T) {
	want := []string{
		TriggerResourceExhausted,
		TriggerCircuitBreakerL1,
		TriggerIndeterminate3x,
		TriggerEmptyEvidence,
		TriggerManualAbort,
	}
	expected := []string{"resource_exhausted", "cb_l1", "indeterminate_3x", "empty_evidence", "manual_abort"}
	if len(want) != len(expected) {
		t.Fatalf("trigger count mismatch: %d vs %d", len(want), len(expected))
	}
	for i, w := range want {
		if w != expected[i] {
			t.Errorf("trigger[%d] = %q, want %q", i, w, expected[i])
		}
	}
}

// TestNewORCHPessimisticTriggeredError_UniqueCodes — guard against code
// collisions with the existing escape/ 7101-7104 range. PR-B reserves
// 7110-7113; this test fails fast if a future change drifts into a taken
// range.
func TestNewORCHPessimisticTriggeredError_UniqueCodes(t *testing.T) {
	helpers := []struct {
		helper func() *sharederrors.SentinelError
		code   string
	}{
		{NewORCHPessimisticTriggeredError, "ORCH_PESSIMISTIC_TRIGGERED_7110"},
		{NewORCHPessimisticMVPEmptyError, "ORCH_PESSIMISTIC_MVP_EMPTY_7111"},
		{NewORCHFallbackRuleInvalidError, "ORCH_FALLBACK_RULE_INVALID_7112"},
		{NewORCHFallbackAbortTimeoutError, "ORCH_FALLBACK_ABORT_TIMEOUT_7113"},
	}
	seen := map[string]bool{}
	for _, h := range helpers {
		if seen[h.code] {
			t.Errorf("duplicate code %q", h.code)
		}
		seen[h.code] = true
		got := h.helper()
		if got.Code != h.code {
			t.Errorf("helper code = %q, want %q", got.Code, h.code)
		}
	}
}
