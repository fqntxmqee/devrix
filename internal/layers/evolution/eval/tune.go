package eval

import (
	"fmt"
	"strings"
)

// TuneGenerator produces config tuning suggestions from delta regressions.
type TuneGenerator struct{}

// NewTuneGenerator creates a tune suggestion generator.
func NewTuneGenerator() *TuneGenerator {
	return &TuneGenerator{}
}

// Suggest maps regression entries to actionable config hints.
func (g *TuneGenerator) Suggest(delta *EvalDelta) []TuneSuggestion {
	if delta == nil || len(delta.Regressions) == 0 {
		return nil
	}
	var out []TuneSuggestion
	for _, reg := range delta.Regressions {
		if s := suggestForRegression(reg); s != nil {
			out = append(out, *s)
		}
	}
	return out
}

func suggestForRegression(reg DeltaEntry) *TuneSuggestion {
	dim := reg.Dimension
	drop := reg.Previous - reg.Current

	switch {
	case strings.HasSuffix(dim, ".compression_recall"):
		return &TuneSuggestion{
			Target:       "context_engine.compression.budget",
			Reason:       fmt.Sprintf("compression recall regressed by %.2f (%.2f → %.2f)", drop, reg.Previous, reg.Current),
			CurrentVal:   "unknown",
			SuggestedVal: "increase by 20%",
			Confidence:   confidenceForDelta(reg.Delta),
		}
	case strings.HasSuffix(dim, ".pev_tool_accuracy"):
		return &TuneSuggestion{
			Target:       "context_engine.harness.tool_pool.simple_mode",
			Reason:       fmt.Sprintf("PEV tool accuracy regressed by %.2f (%.2f → %.2f)", drop, reg.Previous, reg.Current),
			CurrentVal:   "false",
			SuggestedVal: "true",
			Confidence:   confidenceForDelta(reg.Delta),
		}
	case strings.HasSuffix(dim, ".provider_quality"):
		return &TuneSuggestion{
			Target:       "llm_gateway.default_provider",
			Reason:       fmt.Sprintf("provider quality regressed by %.2f (%.2f → %.2f)", drop, reg.Previous, reg.Current),
			CurrentVal:   "unknown",
			SuggestedVal: "review fallback chain weights",
			Confidence:   ConfidenceLow,
		}
	case strings.HasSuffix(dim, ".agent_forkjoin"):
		return &TuneSuggestion{
			Target:       "multi_agent.forkjoin.timeout",
			Reason:       fmt.Sprintf("fork/join quality regressed by %.2f (%.2f → %.2f)", drop, reg.Previous, reg.Current),
			CurrentVal:   "unknown",
			SuggestedVal: "increase join timeout by 30%",
			Confidence:   ConfidenceLow,
		}
	default:
		return &TuneSuggestion{
			Target:       dim,
			Reason:       fmt.Sprintf("dimension regressed by %.2f (%.2f → %.2f)", drop, reg.Previous, reg.Current),
			CurrentVal:   "unknown",
			SuggestedVal: "investigate recent changes",
			Confidence:   ConfidenceLow,
		}
	}
}

func confidenceForDelta(delta float64) string {
	if delta < RegressionRedThreshold {
		return ConfidenceHigh
	}
	if delta < RegressionYellowThreshold {
		return ConfidenceMedium
	}
	return ConfidenceLow
}
