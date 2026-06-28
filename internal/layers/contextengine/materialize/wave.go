package materialize

import "strings"

const (
	WavePolicyFresh    = "fresh"
	WavePolicyResume   = "resume"
	WavePolicyUpstream = "upstream"
)

// PolicyFromWaveContext maps Wave scheduler context policies to Materialize modes (D7-S16 T34).
func PolicyFromWaveContext(policy string) Policy {
	switch strings.TrimSpace(policy) {
	case WavePolicyResume:
		return Policy{Mode: ModeResume}
	case WavePolicyUpstream:
		return Policy{Mode: ModeUpstream}
	default:
		return Policy{Mode: ModeFresh}
	}
}
