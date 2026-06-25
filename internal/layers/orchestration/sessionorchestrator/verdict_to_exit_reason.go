package sessionorchestrator

import (
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

// VerdictToExitReason maps a Verifier Verdict (Phase 4 PR-D1 data contract)
// to an orchestrator-level ExitReason. This is the single source of truth
// for "why the turn stopped" when the stop was triggered by Verify, NOT
// by deterministic orchestrator conditions (max_turns / aborted_* /
// repeated_tool etc).
//
// Mapping:
//
//	VerdictPass            → ExitReasonNatural
//	VerdictPartial         → ExitReasonPartialVerified
//	VerdictIndeterminate   → ExitReasonVerifierAbstain
//	VerdictFail            → ExitReasonVerifierFail
//	SystemAnomaly=true     → ExitReasonSystemAnomaly (overrides above)
//
// sessionID is accepted (currently unused but reserved) so Phase 5
// ReputationEvidence can attribute the exit reason to a session without
// changing the function signature later.
//
// Phase 4 PR-D2 (DM-20260623-002) introduces this to replace the
// orchestrator.go inline string switch from doc 17/18. Prior to PR-D2
// verifier-driven exit reasons were ad-hoc; this unifies them into a
// 14-value enum (8 deterministic + 6 verify-driven).
func VerdictToExitReason(v workmodel.Verdict, sessionID string) ExitReason {
	_ = sessionID // reserved for Phase 5 ReputationEvidence attribution
	if v.SystemAnomaly {
		return ExitReasonSystemAnomaly
	}
	switch v.Kind {
	case orchtypes.VerdictPass:
		return ExitReasonNatural
	case orchtypes.VerdictPartial:
		return ExitReasonPartialVerified
	case orchtypes.VerdictIndeterminate:
		return ExitReasonVerifierAbstain
	case orchtypes.VerdictFail:
		return ExitReasonVerifierFail
	default:
		// Unknown verdict kind → safe default to VerifierAbstain so
		// downstream consumers know to surface for human review rather
		// than treating as healthy LLM finish.
		return ExitReasonVerifierAbstain
	}
}