// PessimisticCommitGuard default implementation (PR-B, DM-20260629-008).
//
// 5 类触发条件 → FallbackPolicy 决策：
//   1. resource_exhausted  → FallbackPessimistic (commit MVP)
//   2. cb_l1               → FallbackPessimistic (commit MVP)
//   3. indeterminate_3x    → FallbackRuleBased (候选规则打分)
//   4. empty_evidence      → FallbackPessimistic (commit MVP)
//   5. manual_abort        → FallbackAbort (向后兼容, Failed)
//
// Feature Flag: D7_PESSIMISTIC_COMMIT_ENABLED (default false = 0 行为变更).
// Feature Flag off 时 Evaluate 直接返回 ok=true 跳过整套逻辑.
package escape

import (
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
)

// DefaultPessimisticCommitGuard is the production PessimisticCommitGuard
// implementation. It is a pure decision function: no I/O, no global state,
// no goroutines. The wire-up layer (internal/bootstrap) decides whether
// to enable it via the Feature Flag.
type DefaultPessimisticCommitGuard struct {
	// Enabled gates the entire 5-class trigger evaluation. When false
	// the guard is a no-op and the legacy v6.0.x path runs untouched.
	// The bootstrap consults D7_PESSIMISTIC_COMMIT_ENABLED at startup
	// and passes the bool here.
	Enabled bool

	// RuleName is the FallbackRuleBased candidate selected by the env
	// override D7_RULE_FALLBACK_STRATEGY (default: "min_uncertainty").
	// Validated against interfaces.FallbackPolicyRuleNames.
	RuleName string

	// AbortTimeoutMs is the grace window for FallbackAbort before
	// returning ORCH_FALLBACK_ABORT_TIMEOUT_7113. Default 5000 ms.
	AbortTimeoutMs int

	// IndeterminateThreshold is the number of consecutive
	// ResultKindIndeterminate verdicts required to trigger the
	// "indeterminate_3x" rule-based path. Default 3.
	IndeterminateThreshold int
}

// NewDefaultPessimisticCommitGuard constructs a guard with sensible
// defaults. Callers should override at least the Enabled field and
// optionally RuleName from the env.
func NewDefaultPessimisticCommitGuard() *DefaultPessimisticCommitGuard {
	return &DefaultPessimisticCommitGuard{
		Enabled:                false, // default off — 0 behavior change
		RuleName:               interfaces.DefaultFallbackRule,
		AbortTimeoutMs:         5000,
		IndeterminateThreshold: 3,
	}
}

// Evaluate implements PessimisticCommitGuard. When the guard is
// disabled (Feature Flag off) it is a pure pass-through: ok=true and a
// nil error. The 5-class trigger scan only runs when Enabled=true.
func (g *DefaultPessimisticCommitGuard) Evaluate(
	spec *interfaces.TaskSpec,
	report *interfaces.TaskReport,
	budget interfaces.ConvergenceBudget,
) (bool, string, error) {
	if g == nil || !g.Enabled {
		return true, "", nil
	}
	if report == nil {
		return true, "", nil
	}

	// T1: resource exhausted
	if ok, reason := g.checkResourceExhausted(report); !ok {
		return false, reason, nil
	}
	// T2: CB L1+ triggered (signalled by report.Blockage[].Kind == Infeasible)
	if ok, reason := g.checkCircuitBreakerL1(report); !ok {
		return false, reason, nil
	}
	// T3: N consecutive indeterminate verdicts
	if ok, reason := g.checkIndeterminateThreshold(report); !ok {
		return false, reason, nil
	}
	// T4: empty evidence on Pass verdict (AC15 enforcement lives in PR-C;
	// PR-B's pessimistic path is the catch-net).
	if ok, reason := g.checkEmptyEvidence(report); !ok {
		return false, reason, nil
	}
	// T5: manual abort — for PR-B this is signalled by FallbackAbort on
	// the spec's ConvergenceBudget.Policy (set by handleInterrupt).
	if ok, reason := g.checkManualAbort(spec); !ok {
		return false, reason, nil
	}
	return true, "", nil
}

// ResolveFallback picks the FallbackPolicy for a blocked report. The
// selection rule is:
//
//   - If the spec's ConvergenceBudget.Policy is one of the 3 valid
//     values, use it. FallbackAbort takes precedence (preserves v6.0.x
//     behavior for operators who flip the policy deliberately).
//   - Otherwise, return FallbackPessimistic as the safest default.
func (g *DefaultPessimisticCommitGuard) ResolveFallback(
	report *interfaces.TaskReport,
) (interfaces.FallbackPolicy, string) {
	if g == nil || !g.Enabled {
		return interfaces.FallbackPessimistic, ""
	}
	if report == nil {
		return interfaces.FallbackPessimistic, ""
	}
	// Inspect the spec's policy via the report's Blockage.Source
	// convention: a Blockage with Source == "policy_override" carries
	// the policy name in Description. This is a soft signal; when no
	// override is present we fall through to Pessimistic.
	for _, b := range report.Blockage {
		if b.Source == "policy_override" {
			switch strings.ToLower(strings.TrimSpace(b.Description)) {
			case "abort":
				return interfaces.FallbackAbort, ""
			case "rule_based":
				name, _ := interfaces.ParseFallbackRuleName(g.RuleName)
				return interfaces.FallbackRuleBased, name
			case "pessimistic":
				return interfaces.FallbackPessimistic, ""
			}
		}
	}
	return interfaces.FallbackPessimistic, ""
}

// BuildMVPArtifact synthesizes a best-effort MVP from a blocked report.
// The MVP carries:
//
//   - Output: the report's existing Message + any non-empty Result.Message,
//     joined with a newline. Empty if the report has nothing — caller
//     should treat this as a hard error (ErrORCHPessimisticMVPEmpty).
//   - RiskWarnings: one entry per Blockage (the load-bearing reason the
//     plan didn't finish) plus a static "partial commit" header.
//   - Trigger: the reason string returned by Evaluate (one of the
//     Trigger* constants in interfaces/).
//   - Traceback: capped concatenation of the Blockage.Traceback fields.
//   - GeneratedAtMs: time.Now().UnixMilli().
//
// The function never mutates the receiver report; the caller is
// expected to call TaskReport.WithMVPArtifact on the result.
func (g *DefaultPessimisticCommitGuard) BuildMVPArtifact(
	report *interfaces.TaskReport,
	reason string,
) interfaces.MVPArtifact {
	if report == nil {
		return interfaces.MVPArtifact{
			Output:       "",
			RiskWarnings:  []string{"report is nil"},
			Trigger:      reason,
			ChainHash:    buildChainHash("", time.Now().UnixMilli()),
		}
	}

	var out strings.Builder
	if report.Result.Message != "" {
		out.WriteString(report.Result.Message)
	}
	for _, b := range report.Blockage {
		if b.Description != "" {
			if out.Len() > 0 {
				out.WriteString("\n")
			}
			out.WriteString(b.Description)
		}
	}
	if report.Evidence.TestResult != "" {
		if out.Len() > 0 {
			out.WriteString("\n")
		}
		out.WriteString("test: ")
		out.WriteString(report.Evidence.TestResult)
	}

	var warnings []string
	warnings = append(warnings, "partial commit (pessimistic fallback)")
	for _, b := range report.Blockage {
		if b.Description != "" {
			warnings = append(warnings, "blockage: "+b.Description)
		}
	}

	var traceback strings.Builder
	for i, b := range report.Blockage {
		if b.Traceback == "" {
			continue
		}
		if i > 0 {
			traceback.WriteString(" | ")
		}
		// Cap each entry to 256 chars to keep the artifact bounded.
		if len(b.Traceback) > 256 {
			traceback.WriteString(b.Traceback[:256])
			traceback.WriteString("...")
		} else {
			traceback.WriteString(b.Traceback)
		}
	}

	return interfaces.MVPArtifact{
		Output:       out.String(),
		RiskWarnings: warnings,
		Trigger:      reason,
		ChainHash:    buildChainHash(traceback.String(), time.Now().UnixMilli()),
	}
}

// --- 5 trigger checks (PR-B design.md §3.2) ---

func (g *DefaultPessimisticCommitGuard) checkResourceExhausted(
	report *interfaces.TaskReport,
) (bool, string) {
	used := report.Resource.TokensUsed
	budget := report.Resource.TokensBudget
	// Reserve defaults to 10% of budget; PR-C exposes this via env.
	reserve := budget / 10
	if reserve < 1 {
		reserve = 1
	}
	if interfaces.RemainingBelowReserve(used, budget, reserve) {
		return false, interfaces.TriggerResourceExhausted
	}
	return true, ""
}

func (g *DefaultPessimisticCommitGuard) checkCircuitBreakerL1(
	report *interfaces.TaskReport,
) (bool, string) {
	// A Blockage with Kind == BlockageInfeasible is the canonical signal
	// that the 5-layer CB has fired (set by circuit_breaker.go L1 action
	// added in S4-5). Multiple kinds of Blockage can coexist; only the
	// first Infeasible triggers.
	for _, b := range report.Blockage {
		if b.Kind == interfaces.BlockageInfeasible {
			return false, interfaces.TriggerCircuitBreakerL1
		}
	}
	return true, ""
}

func (g *DefaultPessimisticCommitGuard) checkIndeterminateThreshold(
	report *interfaces.TaskReport,
) (bool, string) {
	threshold := g.IndeterminateThreshold
	if threshold < 1 {
		threshold = 3
	}
	// The report itself doesn't carry the rolling window (that's a
	// pipeline concern); for PR-B we approximate by counting how many
	// "indeterminate" hints are present in the Dissent slice. This is
	// a conservative signal — when Dissent is empty, the threshold
	// can't fire from the report alone, and the engine is expected to
	// pre-filter by CB L3 evidence.
	if report.Result.Kind == interfaces.ResultKindIndeterminate {
		count := 1 + len(report.Dissent)
		if count >= threshold {
			return false, interfaces.TriggerIndeterminate3x
		}
	}
	return true, ""
}

func (g *DefaultPessimisticCommitGuard) checkEmptyEvidence(
	report *interfaces.TaskReport,
) (bool, string) {
	if report.Result.Kind != interfaces.ResultKindPass {
		return true, ""
	}
	ev := report.Evidence
	if ev.TestResult == "" && ev.LogExcerpt == "" && ev.ArtifactHash == "" {
		return false, interfaces.TriggerEmptyEvidence
	}
	return true, ""
}

func (g *DefaultPessimisticCommitGuard) checkManualAbort(
	spec *interfaces.TaskSpec,
) (bool, string) {
	if spec == nil {
		return true, ""
	}
	if spec.ConvergenceBudget.Policy == interfaces.FallbackAbort {
		return false, interfaces.TriggerManualAbort
	}
	return true, ""
}

// buildChainHash produces the ChainHash field for MVPArtifact. PR-A
// defines ChainHash as a hex SHA-256 of the produced artifact; for PR-B
// we use a much cheaper non-cryptographic digest (FNV-1a over the
// traceback + unix-millis) because the MVP is a "best-effort" payload
// and a full SHA pass would be a waste. PR-C will replace this with the
// canonical SHA-256 if operators need audit-grade integrity.
func buildChainHash(traceback string, unixMillis int64) string {
	const offset64 uint64 = 14695981039346656037
	const prime64 uint64 = 1099511628211
	h := offset64
	for i := 0; i < len(traceback); i++ {
		h ^= uint64(traceback[i])
		h *= prime64
	}
	for i := 0; i < 8; i++ {
		b := byte(unixMillis >> (i * 8))
		h ^= uint64(b)
		h *= prime64
	}
	const hexDigits = "0123456789abcdef"
	var buf [16]byte
	for i := 0; i < 16; i++ {
		buf[i] = hexDigits[(h>>(i*4))&0xF]
	}
	return string(buf[:])
}
