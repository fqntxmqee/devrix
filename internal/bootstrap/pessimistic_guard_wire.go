// Pessimistic Commit Guard wiring helpers (PR-B, DM-20260629-008).
//
// Status: env helper + factory landed in PR-B. The actual production
// wire (EscapeEngine.SetPessimisticGuard + ChannelRouter.SetPessimisticGuard)
// is deliberately deferred to a follow-up PR once the observability
// surface (span ops, metrics) for the guard is finalized. PR-B
// guarantees 0 behavior change — the guard is feature-flag gated, and
// when the flag is unset the helpers return the disabled-by-default
// guard.
//
// Feature Flag: D7_PESSIMISTIC_COMMIT_ENABLED
//   - "true" | "1" | "yes" → guard enabled
//   - anything else (incl. unset) → guard disabled (default, 0 行为变更)
//
// Rule override: D7_RULE_FALLBACK_STRATEGY
//   - one of "most_tests_passed" | "compiled_clean" | "min_cost" | "min_uncertainty"
//   - default: "min_uncertainty" (PR-B's safe default; aligns with the
//     Verifier's own uncertainty scoring).
package bootstrap

import (
	"os"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/escape"
	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
)

// PessimisticCommitEnabled reports the value of the D7_PESSIMISTIC_COMMIT_ENABLED
// feature flag. Returns false for any unrecognized value or when the env
// is unset — the default is "off" so PR-B ships with 0 behavior change.
//
// The check is intentionally permissive: "1", "true", "TRUE", "yes" all
// count as enabled. Anything else is treated as disabled.
func PessimisticCommitEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("D7_PESSIMISTIC_COMMIT_ENABLED")))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// PessimisticRuleStrategy reports the value of the
// D7_RULE_FALLBACK_STRATEGY env override. Returns the validated rule
// name; unrecognized values fall back to the default ("min_uncertainty")
// and the boolean return is false. Empty env returns the default with
// recognized=true.
func PessimisticRuleStrategy() (name string, recognized bool) {
	return interfaces.ParseFallbackRuleName(os.Getenv("D7_RULE_FALLBACK_STRATEGY"))
}

// NewPessimisticCommitGuardFromEnv builds a DefaultPessimisticCommitGuard
// with the Enabled and RuleName fields wired from the env. The guard
// is always returned (never nil) so the caller's wire code can be
// unconditional; when the flag is off the guard's methods are
// pass-through no-ops.
//
// The returned guard is the v6.0.x→v7.0 bridge type; future PRs will
// replace it with a wire-from-config version once devrix.yaml schema
// gains the d7.pessimistic_commit block (PR-C scope).
func NewPessimisticCommitGuardFromEnv() *escape.DefaultPessimisticCommitGuard {
	g := escape.NewDefaultPessimisticCommitGuard()
	g.Enabled = PessimisticCommitEnabled()
	name, _ := PessimisticRuleStrategy()
	g.RuleName = name
	return g
}
