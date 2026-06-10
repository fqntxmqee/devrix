package eval

import (
	"fmt"
	"strings"
)

// GateResult summarizes delta gate evaluation.
type GateResult struct {
	Passed      bool
	Regressions []DeltaEntry
	Summary     string
}

// CheckDeltaGate fails when delta contains regression entries.
func CheckDeltaGate(delta *EvalDelta) GateResult {
	if delta == nil || len(delta.Regressions) == 0 {
		return GateResult{
			Passed:  true,
			Summary: "eval delta gate: passed (no regressions)",
		}
	}
	return GateResult{
		Passed:      false,
		Regressions: delta.Regressions,
		Summary:     FormatDeltaSummary(delta),
	}
}

// FormatDeltaSummary renders a human-readable delta report for CI logs.
func FormatDeltaSummary(delta *EvalDelta) string {
	if delta == nil {
		return "eval delta: no baseline comparison"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "eval delta vs baseline %s\n", delta.BaselineID)
	if len(delta.Regressions) == 0 {
		b.WriteString("status: PASS (no regressions)\n")
	} else {
		fmt.Fprintf(&b, "status: FAIL (%d regressions)\n", len(delta.Regressions))
		for _, reg := range delta.Regressions {
			fmt.Fprintf(&b, "  - %s: %.3f → %.3f (Δ %.3f, %s)\n",
				reg.Dimension, reg.Previous, reg.Current, reg.Delta, reg.Severity)
		}
	}
	for dim, entry := range delta.ByDimension {
		if entry.Severity == SeverityImprovement {
			fmt.Fprintf(&b, "  + %s: %.3f → %.3f (Δ %.3f)\n", dim, entry.Previous, entry.Current, entry.Delta)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// GateError wraps a failed gate result as an error.
type GateError struct {
	Result GateResult
}

func (e *GateError) Error() string {
	return e.Result.Summary
}
