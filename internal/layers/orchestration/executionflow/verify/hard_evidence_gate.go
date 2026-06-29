// Hard Evidence Gate (PR-C, DM-20260629-009).
//
// Provides verify-time rejection of a VerdictPass that lacks the kind-specific
// minimum evidence, closing AC15 (Verifier "空证 PASS" silent corruption).
//
// Gated behind the D7_HARD_EVIDENCE_ENABLED feature flag. When the flag is
// unset (default), the gate is a no-op and the verifier behavior is
// unchanged. This preserves PR-C's "0 行为变更" promise.
//
// Wiring: production call sites should invoke GateVerdictPass(report) once
// per Pass verdict. The gate returns (true, nil) when the flag is disabled,
// so pre-flag-flip code paths are safe.
package verify

import (
	"os"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
)

// hardEvidenceEnabled reads D7_HARD_EVIDENCE_ENABLED directly. Tests can
// override the gate's behavior via SetHardEvidenceEnabledForTest.
//
// Inlined here (not via bootstrap.HardEvidenceEnabled) to avoid an
// import cycle: bootstrap imports sessionorchestrator which imports
// executionflow/verify. Bootstrap's Helper is also exported for callers
// outside the orchestration package hierarchy.
func hardEvidenceEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("D7_HARD_EVIDENCE_ENABLED")))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// SetHardEvidenceEnabledForTest overrides the feature flag read inside the
// gate. Used by tests in other packages that can't easily flip env vars.
// Returns a restore function.
func SetHardEvidenceEnabledForTest(enabled bool) func() {
	old := hardEvidenceEnabledFn
	hardEvidenceEnabledFn = func() bool { return enabled }
	return func() { hardEvidenceEnabledFn = old }
}

// hardEvidenceEnabledFn is the function-var indirection used by the gate.
// Initialised to the env read; tests can swap it via SetHardEvidenceEnabledForTest.
var hardEvidenceEnabledFn = hardEvidenceEnabled

// GateVerdictPass returns (true, nil) if the verdict may pass; (false, error)
// if Hard Evidence rejects it. When the feature flag is disabled (default),
// always returns (true, nil) — i.e., 0 behavior change.
//
// `kind` defaults to "code" when empty (conservative). `ev` may be nil and
// is treated as "no evidence".
func GateVerdictPass(ev *interfaces.Evidence, kind string) (bool, error) {
	if !hardEvidenceEnabledFn() {
		return true, nil
	}
	hard := interfaces.ExtractHardEvidenceFromEvidence(ev)
	if kind != "" {
		hard = hard.WithKind(kind)
	}
	if hard.Verified() {
		return true, nil
	}
	return false, interfaces.NewHardEvidenceMissingError()
}
