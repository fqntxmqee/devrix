// Package learn: 3-tier degradation tests (DM-20260707-001 PR-E, T64).
//
// Coverage matrix for ClassifyDegradation + DegradationLevel.String +
// EmitDegradationAudit:
//
//   1. ClassifyDegradation_NilError → None
//   2. ClassifyDegradation_L1ReputationStoreUnavailable
//   3. ClassifyDegradation_L2BayesianUpdateFailed
//   4. ClassifyDegradation_L3MemoryStoreFull (ShouldRetry=true)
//   5. ClassifyDegradation_L3MemoryStoreTransient (ShouldRetry=true)
//   6. ClassifyDegradation_UnknownError → conservative L1
//   7. DegradationLevel_String_AllCases
//   8. EmitDegradationAudit_NilAudit_PassThrough
//   9. EmitDegradationAudit_L3_LogsWarn
package learn

import (
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

// TestClassifyDegradation_NilError: healthy path returns DegradationNone.
func TestClassifyDegradation_NilError(t *testing.T) {
	t.Parallel()
	res := ClassifyDegradation("sess", nil)
	if res.Level != DegradationNone {
		t.Errorf("Level = %s, want None", res.Level)
	}
	if res.SkipLearn {
		t.Errorf("SkipLearn = true, want false")
	}
	if res.ShouldRetry {
		t.Errorf("ShouldRetry = true, want false")
	}
	if res.Audit != nil {
		t.Errorf("Audit = %v, want nil", res.Audit)
	}
}

// TestClassifyDegradation_L1ReputationStoreUnavailable.
func TestClassifyDegradation_L1ReputationStoreUnavailable(t *testing.T) {
	t.Parallel()
	res := ClassifyDegradation("sess_l1", ErrReputationStoreUnavailable)
	if res.Level != DegradationL1 {
		t.Errorf("Level = %s, want L1", res.Level)
	}
	if !res.SkipLearn {
		t.Errorf("L1 should SkipLearn")
	}
	if res.Audit == nil {
		t.Fatalf("Audit = nil, want populated")
	}
	if res.Audit.SessionID != "sess_l1" {
		t.Errorf("Audit.SessionID = %s, want sess_l1", res.Audit.SessionID)
	}
	if res.Audit.Retryable {
		t.Errorf("L1 should not be Retryable")
	}
}

// TestClassifyDegradation_L2BayesianUpdateFailed.
func TestClassifyDegradation_L2BayesianUpdateFailed(t *testing.T) {
	t.Parallel()
	res := ClassifyDegradation("sess_l2", ErrBayesianUpdateFailed)
	if res.Level != DegradationL2 {
		t.Errorf("Level = %s, want L2", res.Level)
	}
	if !res.SkipLearn {
		t.Errorf("L2 should SkipLearn")
	}
	if res.Audit == nil || res.Audit.SessionID != "sess_l2" {
		t.Errorf("Audit.SessionID mismatch")
	}
}

// TestClassifyDegradation_L3MemoryStoreFull: should retry.
func TestClassifyDegradation_L3MemoryStoreFull(t *testing.T) {
	t.Parallel()
	res := ClassifyDegradation("sess_l3_full", ErrMemoryStoreFull)
	if res.Level != DegradationL3 {
		t.Errorf("Level = %s, want L3", res.Level)
	}
	if res.SkipLearn {
		t.Errorf("L3 should NOT skip Learn (retry the store)")
	}
	if !res.ShouldRetry {
		t.Errorf("L3 should ShouldRetry")
	}
	if !res.Audit.Retryable {
		t.Errorf("L3 Audit.Retryable = false, want true")
	}
}

// TestClassifyDegradation_L3MemoryStoreTransient: also L3 retry.
func TestClassifyDegradation_L3MemoryStoreTransient(t *testing.T) {
	t.Parallel()
	res := ClassifyDegradation("sess_l3_trn", ErrMemoryStoreTransient)
	if res.Level != DegradationL3 {
		t.Errorf("Level = %s, want L3", res.Level)
	}
	if !res.ShouldRetry {
		t.Errorf("L3 transient should ShouldRetry")
	}
}

// TestClassifyDegradation_UnknownError_ConservativeL1: wrapped unknown error
// falls back to L1 (skip Learn).
func TestClassifyDegradation_UnknownError_ConservativeL1(t *testing.T) {
	t.Parallel()
	res := ClassifyDegradation("sess_unk", errors.New("something weird"))
	if res.Level != DegradationL1 {
		t.Errorf("Level = %s, want L1 (conservative fallback)", res.Level)
	}
	if !res.SkipLearn {
		t.Errorf("conservative L1 should SkipLearn")
	}
	if res.Audit == nil {
		t.Fatalf("Audit should be populated for unknown error")
	}
	if res.Audit.Reason != "unknown_error_conservative_skip" {
		t.Errorf("Reason = %s, want unknown_error_conservative_skip", res.Audit.Reason)
	}
}

// TestDegradationLevel_String_AllCases.
func TestDegradationLevel_String_AllCases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		level DegradationLevel
		want  string
	}{
		{DegradationNone, "none"},
		{DegradationL1, "L1_reputation_store"},
		{DegradationL2, "L2_bayesian_update"},
		{DegradationL3, "L3_memory_store"},
		{DegradationLevel(99), "unknown"},
	}
	for _, tc := range cases {
		if got := tc.level.String(); got != tc.want {
			t.Errorf("Level(%d).String() = %s, want %s", tc.level, got, tc.want)
		}
	}
}

// captureLogger is a minimal DegradationLogger for testing EmitDegradationAudit.
type captureLogger struct {
	mu       sync.Mutex
	infoMsgs []string
	warnMsgs []string
	errMsgs  []string
}

func (c *captureLogger) Info(msg string, args ...any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.infoMsgs = append(c.infoMsgs, msg)
}
func (c *captureLogger) Warn(msg string, args ...any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.warnMsgs = append(c.warnMsgs, msg)
}
func (c *captureLogger) Error(msg string, args ...any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.errMsgs = append(c.errMsgs, msg)
}

// TestEmitDegradationAudit_NilAudit_PassThrough.
func TestEmitDegradationAudit_NilAudit_PassThrough(t *testing.T) {
	t.Parallel()
	cap := &captureLogger{}
	res := DegradationResult{Level: DegradationNone}
	out := EmitDegradationAudit(cap, res)
	if out.Level != DegradationNone {
		t.Errorf("Level = %s, want None", out.Level)
	}
	if len(cap.infoMsgs)+len(cap.warnMsgs)+len(cap.errMsgs) != 0 {
		t.Errorf("expected no log calls for nil audit, got %d", len(cap.infoMsgs)+len(cap.warnMsgs)+len(cap.errMsgs))
	}
}

// TestEmitDegradationAudit_L3_LogsWarn: L3 logs at Warn (retryable).
func TestEmitDegradationAudit_L3_LogsWarn(t *testing.T) {
	t.Parallel()
	cap := &captureLogger{}
	res := ClassifyDegradation("sess_warn", ErrMemoryStoreFull)
	EmitDegradationAudit(cap, res)
	if len(cap.warnMsgs) != 1 {
		t.Errorf("expected 1 Warn call for L3, got %d (info=%d err=%d)", len(cap.warnMsgs), len(cap.infoMsgs), len(cap.errMsgs))
	}
	if !strings.Contains(cap.warnMsgs[0], "learn_degradation") {
		t.Errorf("Warn msg = %q, missing learn_degradation", cap.warnMsgs[0])
	}
}

// TestEmitDegradationAudit_L1_LogsError: L1 logs at Error.
func TestEmitDegradationAudit_L1_LogsError(t *testing.T) {
	t.Parallel()
	cap := &captureLogger{}
	res := ClassifyDegradation("sess_err", ErrReputationStoreUnavailable)
	EmitDegradationAudit(cap, res)
	if len(cap.errMsgs) != 1 {
		t.Errorf("expected 1 Error call for L1, got %d", len(cap.errMsgs))
	}
}

// TestEmitDegradationAudit_RealSlog: integration with slog.Logger (no panic).
func TestEmitDegradationAudit_RealSlog(t *testing.T) {
	t.Parallel()
	logger := slog.Default()
	res := ClassifyDegradation("sess_slog", ErrMemoryStoreFull)
	EmitDegradationAudit(logger, res) // must not panic
}