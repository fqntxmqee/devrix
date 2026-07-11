package sessionorchestrator

import (
	"strings"
)

// Registered Observe signal line prefixes (v1 SoT). Append-only evolution.
const (
	SignalPrefixArtifactSummary    = "artifact_summary:"
	SignalPrefixChildDownlinkScope = "child_downlink_scope_in:"
	SignalPrefixExpectedReturn     = "expected_return:"
)

func formatObserveSignalLine(prefix, value string) string {
	return strings.TrimSpace(prefix) + " " + strings.TrimSpace(value)
}

func isRegisteredObserveSignalLine(line string) bool {
	line = strings.TrimSpace(line)
	for _, p := range []string{
		SignalPrefixArtifactSummary,
		SignalPrefixChildDownlinkScope,
		SignalPrefixExpectedReturn,
	} {
		if strings.HasPrefix(line, p) {
			return true
		}
	}
	return false
}
