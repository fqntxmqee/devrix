package interfaces

// FallbackPolicy helpers — the enum itself is declared in task_spec.go
// (PR-A landed it there as part of the TaskSpec.ConvergenceBudget.Policy
// field, so PR-B does not redeclare it). The helpers here are pure
// functions that operate on the existing FallbackPolicy type, which keeps
// the Pure types invariant (IV-1) and avoids the package splitting that
// the design.md §2.1 originally sketched.

// FallbackPolicyRuleNames is the closed set of valid rule names for the
// FallbackRuleBased path. The four candidates come from PR-B design.md
// §3.1. Implementations must look up the chosen rule here; anything else
// is a configuration error and must fall back to FallbackPessimistic.
var FallbackPolicyRuleNames = []string{
	"most_tests_passed", // pick the candidate with the highest test pass count
	"compiled_clean",    // pick the candidate whose build produced zero warnings
	"min_cost",          // pick the candidate with the lowest Resource.TokensUsed
	"min_uncertainty",   // pick the candidate with the lowest UncertaintyCoord score (default)
}

// DefaultFallbackRule is the rule name used when FallbackRuleBased is
// selected and the env override D7_RULE_FALLBACK_STRATEGY is unset.
// "min_uncertainty" is the default because it most closely mirrors the
// Verifier's own scoring (LP-1 reputation uses the same uncertainty axis).
const DefaultFallbackRule = "min_uncertainty"

// Valid reports whether p is one of the 3 recognized FallbackPolicy values.
// The 0 value (FallbackAbort) is technically valid, but PR-B considers
// FallbackAbort to be the legacy path. Callers who want to assert
// "non-legacy" should use ValidNonLegacy instead.
func (p FallbackPolicy) Valid() bool {
	switch p {
	case FallbackAbort, FallbackPessimistic, FallbackRuleBased:
		return true
	default:
		return false
	}
}

// ValidNonLegacy reports whether p is one of the 2 PR-B-managed policies
// (Pessimistic or RuleBased). Used by the bootstrap to refuse constructing
// an EscapeEngine wired with FallbackAbort when the Feature Flag is on.
func (p FallbackPolicy) ValidNonLegacy() bool {
	switch p {
	case FallbackPessimistic, FallbackRuleBased:
		return true
	default:
		return false
	}
}

// ParseFallbackRuleName returns the rule name from a string, falling back
// to DefaultFallbackRule when the input is empty or unrecognized. The
// boolean return is false when the input was unparsable AND non-empty, so
// callers can distinguish "unset → default" from "typo → error path".
func ParseFallbackRuleName(s string) (name string, recognized bool) {
	s = trimSpaces(s)
	if s == "" {
		return DefaultFallbackRule, true
	}
	for _, n := range FallbackPolicyRuleNames {
		if n == s {
			return n, true
		}
	}
	return DefaultFallbackRule, false
}

// trimSpaces is a tiny inline replacement for strings.TrimSpace so this
// file does not need an extra import (the strings package is small but
// this keeps the helper boundary tight).
func trimSpaces(s string) string {
	start, end := 0, len(s)
	for start < end {
		c := s[start]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			break
		}
		start++
	}
	for end > start {
		c := s[end-1]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			break
		}
		end--
	}
	return s[start:end]
}
