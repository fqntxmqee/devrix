package sessionorchestrator

import (
	"regexp"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
)

var artifactSummaryBaselinePattern = regexp.MustCompile(`(?i)baseline`)

// promoteSystemCategory elevates LLM ObsDeviation rows to CatSystem when
// inbound signals contain artifact_summary baseline/observed hints (P2 v1).
func promoteSystemCategory(obs []orchtypes.Observation, signalLines []string) []orchtypes.Observation {
	if len(obs) == 0 || !signalLinesHintSystemDeviation(signalLines) {
		return obs
	}
	out := make([]orchtypes.Observation, len(obs))
	for i, o := range obs {
		out[i] = o
		if o.Kind == orchtypes.ObsDeviation {
			cp := o
			cp.Category = orchtypes.CatSystem
			out[i] = cp
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
		lower := strings.ToLower(l)
		if artifactSummaryBaselinePattern.MatchString(lower) || strings.Contains(lower, "observed") {
			return true
		}
	}
	return false
}
