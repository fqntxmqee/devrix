package interfaces

import (
	"errors"

	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// PessimisticCommitGuard is the L3 defensive-runtime contract that decides
// what to do when a Plan / Channel / Verifier pipeline terminates without
// producing a clean Pass verdict. Implementations live in escape/fallback.go;
// this file is a Pure types contract (see doc.go §1) so consumers can wire
// the guard into mups/execute without depending on the escape package.
//
// Three outcomes are defined (see PR-B design.md §3.1):
//
//   - Pessimistic Commit (default): synthesize an MVPArtifact so the user
//     gets a best-effort answer + risk warnings instead of nothing.
//   - Rule-based Fallback: pick a candidate from an env-configurable rule
//     set (most_tests_passed | compiled_clean | min_cost | min_uncertainty).
//   - Abort: v6.0.x backward-compat behavior. Returns ResultKind=Failed.
//
// The guard is consulted at every Channel.Execute exit point. The
// ResolveFallback helper is exposed so the engine can run a single
// Evaluate pass and then look up the chosen policy without re-walking the
// trigger set.
type PessimisticCommitGuard interface {
	// Evaluate returns (ok=true) when the report is acceptable as-is.
	// ok=false with a non-empty blockedReason means the guard has decided
	// to fall back; callers should then call ResolveFallback to pick a
	// policy and BuildMVPArtifact to synthesize the MVP payload.
	//
	// Triggers (5 classes — PR-B design.md §3.2):
	//   1. Resource exhausted (tokens_remaining <= min_reserve)
	//   2. EscapeForceExit / CB L1 fired
	//   3. >= 3 consecutive INDETERMINATE verdicts
	//   4. Verifier "empty evidence PASS"
	//   5. Manual abort (IM channel closed)
	Evaluate(spec *TaskSpec, report *TaskReport, budget ConvergenceBudget) (ok bool, blockedReason string, err error)

	// ResolveFallback returns the chosen FallbackPolicy for a blocked
	// report. When the policy is FallbackRuleBased, ruleName is the name
	// of the rule that was selected (e.g. "min_uncertainty"); for the
	// other two policies ruleName is the empty string.
	ResolveFallback(report *TaskReport) (policy FallbackPolicy, ruleName string)

	// BuildMVPArtifact synthesizes a best-effort MVP from a blocked report
	// plus the reason string returned by Evaluate. The MVP carries the
	// partial output, the risk warnings, the trigger name, a short
	// traceback, and a timestamp.
	BuildMVPArtifact(report *TaskReport, reason string) MVPArtifact
}

// Trigger names returned in blockedReason and stored in MVPArtifact.Trigger.
// Stable for span attributes and downstream filtering.
const (
	TriggerResourceExhausted = "resource_exhausted"
	TriggerCircuitBreakerL1  = "cb_l1"
	TriggerIndeterminate3x   = "indeterminate_3x"
	TriggerEmptyEvidence     = "empty_evidence"
	TriggerManualAbort       = "manual_abort"
)

// Sentinel errors for the contracts layer. Code range 7110-7119 is reserved
// for PR-B (escape subdomain also uses 7100-7199 but only 7101-7104 are
// taken as of v6.0.x → 7110-7113 has no overlap). The wrap helpers live
// here so callers don't need to import escape just to translate a sentinel
// into a Code+Message triple.
var (
	// ErrORCHPessimisticTriggered is returned by PessimisticCommitGuard
	// implementations when a guard decides to fall back. It is the
	// canonical signal that an MVPArtifact will be produced; the
	// downstream consumer should read FallbackUsed + MVPArtifact off the
	// returned TaskReport.
	ErrORCHPessimisticTriggered = errors.New("interfaces: pessimistic commit triggered")

	// ErrORCHPessimisticMVPEmpty is returned by BuildMVPArtifact when the
	// blocked report has no partial output to commit. This should be
	// rare (the report must have produced *something* to be a candidate
	// for Pessimistic Commit) but is checked defensively so we never
	// emit an MVPArtifact with Output == "".
	ErrORCHPessimisticMVPEmpty = errors.New("interfaces: mvp artifact output is empty")

	// ErrORCHFallbackRuleInvalid is returned by ResolveFallback when
	// FallbackRuleBased is selected but the rule name (from env or
	// override) does not match any of the 4 candidates. Implementation
	// must fall back to FallbackPessimistic in this case.
	ErrORCHFallbackRuleInvalid = errors.New("interfaces: fallback rule not recognized")

	// ErrORCHFallbackAbortTimeout is returned by the Abort path when
	// the abort itself exceeds its grace window. Distinct from a normal
	// Abort so the audit log can surface it as a system anomaly.
	ErrORCHFallbackAbortTimeout = errors.New("interfaces: fallback abort timeout")
)

// NewORCHPessimisticTriggeredError returns a *sharederrors.SentinelError
// for ErrORCHPessimisticTriggered with the canonical 7110 code.
func NewORCHPessimisticTriggeredError() *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_PESSIMISTIC_TRIGGERED_7110",
		"pessimistic commit triggered",
		ErrORCHPessimisticTriggered,
	)
}

// NewORCHPessimisticMVPEmptyError returns a *sharederrors.SentinelError
// for ErrORCHPessimisticMVPEmpty with the canonical 7111 code.
func NewORCHPessimisticMVPEmptyError() *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_PESSIMISTIC_MVP_EMPTY_7111",
		"mvp artifact output is empty",
		ErrORCHPessimisticMVPEmpty,
	)
}

// NewORCHFallbackRuleInvalidError returns a *sharederrors.SentinelError
// for ErrORCHFallbackRuleInvalid with the canonical 7112 code.
func NewORCHFallbackRuleInvalidError() *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_FALLBACK_RULE_INVALID_7112",
		"fallback rule not recognized",
		ErrORCHFallbackRuleInvalid,
	)
}

// NewORCHFallbackAbortTimeoutError returns a *sharederrors.SentinelError
// for ErrORCHFallbackAbortTimeout with the canonical 7113 code.
func NewORCHFallbackAbortTimeoutError() *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_FALLBACK_ABORT_TIMEOUT_7113",
		"fallback abort timeout",
		ErrORCHFallbackAbortTimeout,
	)
}
