// Package learn: 3-tier degradation (DM-20260707-001 PR-E, T64).
//
// The Learn pipeline has 3 critical paths, any of which can fail independently:
//
//	L1: ReputationStore (Bayesian α/β state)             → β[α+β] counter
//	L2: BayesianUpdate (verdict → posterior evidence)     → math
//	L3: Memory.Store (SkillMemory / FeedbackMemory write) → persistence
//
// The 3-tier degradation ensures a failure in any tier does NOT cascade into
// a complete Learn crash. Instead, each tier falls through to a defined
// degradation path:
//
//	L1 fail (store unavailable)  → log + emit AuditEntry + skip Learn (no α/β update)
//	L2 fail (Bayesian error)     → log + emit AuditEntry + skip memory write
//	L3 fail (memory write error) → log + emit AuditEntry + retry queue → FeedbackMemory on MaxRetries
//
// Why 3 tiers: each tier has different recovery semantics:
//   - L1 is recoverable on next round (BayesianUpdate is monotonic).
//   - L2 is recoverable on next round (the prior is still consistent).
//   - L3 may need human intervention; FeedbackMemory escalation signals
//     operators that memory is failing.
//   - Crash-and-retry on L3 (the previous behavior) caused complete Learn
//     loss when memory was temporarily full. Tier-specific degradation
//     preserves as much state as possible per failure.
package learn

import (
	"context"
	"errors"
	"log/slog"
)

// Sentinel errors for the 3-tier degradation classifier. These wrap the
// underlying store / math / memory errors so ClassifyDegradation can route
// via errors.Is. Tests + production both raise these.
var (
	// ErrBayesianUpdateFailed — BayesianUpdate returned a non-nil error
	// (typically prior==nil cold start, or math overflow). L2 fall-through.
	ErrBayesianUpdateFailed = errors.New("learn: bayesian update failed")

	// ErrMemoryStoreFull — memory store rejected a write due to capacity
	// (disk full / quota exceeded). L3 retry → FeedbackMemory escalation
	// on MaxRetries exhaustion.
	ErrMemoryStoreFull = errors.New("learn: memory store full")

	// ErrMemoryStoreTransient — memory store returned a transient IO
	// error (network blip / connection reset). L3 retry with backoff.
	ErrMemoryStoreTransient = errors.New("learn: memory store transient")
)

// DegradationLevel identifies which tier of the Learn pipeline failed.
//
// The zero value (DegradationNone) represents a healthy Learn path.
type DegradationLevel int

const (
	// DegradationNone — Learn completed all 3 tiers successfully.
	DegradationNone DegradationLevel = iota

	// DegradationL1 — ReputationStore unavailable. The α/β counter cannot
	// be read or written. The caller logs + skips the entire Learn (no
	// BayesianUpdate, no memory write) and emits an AuditEntry so the
	// dashboard can flag the session.
	DegradationL1

	// DegradationL2 — BayesianUpdate failed (e.g. prior==nil, math
	// overflow). The ReputationStore may have a row but the math to
	// update it failed. The caller emits an AuditEntry + skips the memory
	// write. The ReputationStore row is left untouched (consistent with
	// "monotonic BayesianUpdate" — a failed update is not an update).
	DegradationL2

	// DegradationL3 — Memory.Store failed (e.g. full disk, transient IO).
	// The ReputationStore update may have already succeeded; the caller
	// emits an AuditEntry + retries via the scheduled memory queue
	// (MaxRetries=3) and finally escalates to FeedbackMemory if all
	// retries fail. This is the only tier that retries.
	DegradationL3
)

// String renders the degradation level for logging.
func (d DegradationLevel) String() string {
	switch d {
	case DegradationNone:
		return "none"
	case DegradationL1:
		return "L1_reputation_store"
	case DegradationL2:
		return "L2_bayesian_update"
	case DegradationL3:
		return "L3_memory_store"
	default:
		return "unknown"
	}
}

// AuditEntry is the structured log row emitted when a degradation fires.
// Downstream the D5 dashboard reads this and surfaces degraded sessions.
type AuditEntry struct {
	SessionID string
	Tier      DegradationLevel
	Reason    string
	Err       error
	Retryable bool
}

// DegradationResult is the output of ClassifyDegradation: which tier (if any)
// fired, whether the Learn should be skipped, and the AuditEntry to emit.
type DegradationResult struct {
	// Level is the degradation tier (DegradationNone = healthy).
	Level DegradationLevel

	// SkipLearn is true when the caller should NOT call Memory.Store (L1 +
	// L2 fall through to a no-op; L3 still attempts Memory.Store with
	// retry).
	SkipLearn bool

	// Audit is the entry to emit. Always non-nil when Level != DegradationNone.
	Audit *AuditEntry

	// ShouldRetry is true when L3 hit a transient error and the scheduled
	// memory queue should retry (MaxRetries=3 → FeedbackMemory on exhaustion).
	ShouldRetry bool
}

// ClassifyDegradation inspects the error from a Learn sub-step and returns
// the matching DegradationResult. The classifier is a pure function — the
// caller decides whether to skip / retry / emit audit.
//
// errorClassifiers is the lookup table. Each entry matches the error by
// errors.Is against the sentinel + decides the tier:
//
//   - ErrReputationStoreUnavailable → L1 (no α/β available)
//   - ErrBayesianUpdateFailed       → L2 (math failed)
//   - ErrMemoryStoreFull / IO       → L3 (transient, retry)
//   - everything else                → L1 conservative fallback (skip Learn)
//
// Pure function (no I/O) so the 9-classification matrix is unit-testable.
func ClassifyDegradation(sessionID string, err error) DegradationResult {
	if err == nil {
		return DegradationResult{Level: DegradationNone}
	}

	switch {
	case errors.Is(err, ErrReputationStoreUnavailable):
		return DegradationResult{
			Level:     DegradationL1,
			SkipLearn: true,
			Audit: &AuditEntry{
				SessionID: sessionID,
				Tier:      DegradationL1,
				Reason:    "reputation_store_unavailable",
				Err:       err,
				Retryable: false,
			},
		}

	case errors.Is(err, ErrBayesianUpdateFailed):
		return DegradationResult{
			Level:     DegradationL2,
			SkipLearn: true,
			Audit: &AuditEntry{
				SessionID: sessionID,
				Tier:      DegradationL2,
				Reason:    "bayesian_update_failed",
				Err:       err,
				Retryable: false,
			},
		}

	case errors.Is(err, ErrMemoryStoreFull), errors.Is(err, ErrMemoryStoreTransient):
		return DegradationResult{
			Level:       DegradationL3,
			SkipLearn:   false,
			ShouldRetry: true,
			Audit: &AuditEntry{
				SessionID: sessionID,
				Tier:      DegradationL3,
				Reason:    "memory_store_failure",
				Err:       err,
				Retryable: true,
			},
		}

	default:
		// Unknown error → conservative L1 (skip Learn). The dashboard will
		// surface the raw error so operators can triage.
		return DegradationResult{
			Level:     DegradationL1,
			SkipLearn: true,
			Audit: &AuditEntry{
				SessionID: sessionID,
				Tier:      DegradationL1,
				Reason:    "unknown_error_conservative_skip",
				Err:       err,
				Retryable: false,
			},
		}
	}
}

// DegradationLogger is the interface for emitting AuditEntry to a structured
// log + dashboard. Implemented by slog.Logger in production; tests use a
// captured recorder.
type DegradationLogger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// EmitDegradationAudit logs the AuditEntry at the appropriate severity
// (Info for known + handled, Error for unknown). Returns the DegradationResult
// for the caller to act on.
//
// Why not auto-skip Learn inside ClassifyDegradation: keep the classifier
// pure. The caller decides whether to honor SkipLearn — e.g. an integration
// test might want to force the L3 retry path regardless.
func EmitDegradationAudit(logger DegradationLogger, result DegradationResult) DegradationResult {
	if result.Audit == nil {
		return result
	}
	attrs := []any{
		slog.String("session_id", result.Audit.SessionID),
		slog.String("tier", result.Level.String()),
		slog.String("reason", result.Audit.Reason),
		slog.String("err", result.Audit.Err.Error()),
		slog.Bool("retryable", result.Audit.Retryable),
	}
	switch result.Level {
	case DegradationL3:
		logger.Warn("learn_degradation", attrs...)
	default:
		logger.Error("learn_degradation", attrs...)
	}
	return result
}

// Helper to make Learn-from-context integration clean.
func degradationFromContext(ctx context.Context, sessionID string, err error) DegradationResult {
	return ClassifyDegradation(sessionID, err)
}