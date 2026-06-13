package eval

import "testing"

func TestCheckDeltaGate_Pass(t *testing.T) {
	result := CheckDeltaGate(&EvalDelta{Regressions: nil})
	if !result.Passed {
		t.Fatal("expected pass")
	}
}

func TestCheckDeltaGate_FailOnRegression(t *testing.T) {
	delta := &EvalDelta{
		BaselineID: "baseline",
		Regressions: []DeltaEntry{
			{
				Dimension: "d2.compression_recall",
				Previous:  0.90,
				Current:   0.75,
				Delta:     -0.15,
				Severity:  SeverityRegression,
			},
		},
	}
	result := CheckDeltaGate(delta)
	if result.Passed {
		t.Fatal("expected fail")
	}
	if !stringContains(result.Summary, "FAIL") {
		t.Errorf("summary = %q", result.Summary)
	}
}

func TestFormatDeltaSummary_IncludesImprovement(t *testing.T) {
	delta := &EvalDelta{
		BaselineID: "b1",
		ByDimension: map[string]DeltaEntry{
			"d2.tool_accuracy": {
				Dimension: "d2.tool_accuracy",
				Previous:  0.80,
				Current:   0.95,
				Delta:     0.15,
				Severity:  SeverityImprovement,
			},
		},
	}
	summary := FormatDeltaSummary(delta)
	if !stringContains(summary, "tool_accuracy") {
		t.Errorf("summary = %q", summary)
	}
}
