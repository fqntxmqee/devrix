package filter

import (
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// PerRiskFilter restricts the visible tool set to those at or below the
// configured risk threshold. The risk scale is:
//
//	LOW (0) ⊂ MEDIUM (1) ⊂ HIGH (2)
//
// So Threshold=HIGH keeps all three, Threshold=MEDIUM keeps LOW+MEDIUM,
// and Threshold=LOW keeps only LOW.
//
// An empty (zero) RiskThreshold on the context is treated as "no
// restriction" (equivalent to HIGH) — this matches the per-agent
// pass-through convention used by PerAgentFilter for main/fix.
//
// DSAFT: TOOL-SURFACE-1-A03-F02
type PerRiskFilter struct{}

// NewPerRiskFilter returns a stateless PerRiskFilter.
func NewPerRiskFilter() *PerRiskFilter { return &PerRiskFilter{} }

// riskRank maps a RiskLevel to its numeric rank. Unknown values rank
// below MEDIUM (safer default — unknown tools are not auto-exposed at
// HIGH threshold).
func riskRank(r types.RiskLevel) int {
	switch r {
	case types.RiskLevelCritical:
		return 3
	case types.RiskLevelHigh:
		return 2
	case types.RiskLevelMedium:
		return 1
	case types.RiskLevelLow:
		return 0
	default:
		return -1
	}
}

// Apply implements contracts.ToolFilter.
func (f *PerRiskFilter) Apply(specs []contracts.ToolSpec, ctx contracts.FilterCtx) []contracts.ToolSpec {
	threshold := ctx.RiskThreshold
	if threshold == "" {
		return specs // no threshold → pass-through
	}
	limit := riskRank(threshold)
	if limit < 0 {
		return specs // unknown threshold → pass-through
	}
	out := make([]contracts.ToolSpec, 0, len(specs))
	for _, s := range specs {
		if riskRank(s.Risk) <= limit {
			out = append(out, s)
		}
	}
	return out
}
