package sessionorchestrator

import (
	"regexp"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
)

// Metric-pair hints on artifact_summary that justify elevating ObsDeviation
// to CatSystem (P2). Prefer structured tokens over bare "observed" substring
// to avoid false promotes on prose like "user observed login works".
var (
	artifactSummaryBaselinePattern = regexp.MustCompile(`(?i)\bbaseline\b`)
	artifactSummaryObservedPattern = regexp.MustCompile(`(?i)\bobserved\s*[=:]`)
)

// promoteSystemCategory elevates LLM ObsDeviation rows to CatSystem when
// inbound signals contain artifact_summary baseline / observed= hints (P2 v1).
func promoteSystemCategory(obs []orchtypes.Observation, signalLines []string) []orchtypes.Observation {
	if len(obs) == 0 || !signalLinesHintSystemDeviation(signalLines) {
		return obs
	}
	out := make([]orchtypes.Observation, len(obs))
	for i, o := range obs {
		out[i] = o
		if o.Kind == orchtypes.ObsDeviation {
			out[i] = o.WithCategory(orchtypes.CatSystem)
		}
	}
	return out
}

func signalLinesHintSystemDeviation(lines []string) bool {
	for _, line := range lines {
		l := strings.TrimSpace(line)
		if !strings.HasPrefix(l, SignalPrefixArtifactSummary) {
			continue
		}
		if artifactSummaryBaselinePattern.MatchString(l) || artifactSummaryObservedPattern.MatchString(l) {
			return true
		}
	}
	return false
}
