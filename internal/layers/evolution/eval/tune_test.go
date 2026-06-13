package eval

import "testing"

func TestTuneGenerator_SuggestCompressionRegression(t *testing.T) {
	gen := NewTuneGenerator()
	delta := &EvalDelta{
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

	suggestions := gen.Suggest(delta)
	if len(suggestions) != 1 {
		t.Fatalf("len(suggestions) = %d, want 1", len(suggestions))
	}
	s := suggestions[0]
	if s.Target != "context_engine.compression.budget" {
		t.Errorf("Target = %q, want compression.budget", s.Target)
	}
	if s.Confidence != ConfidenceHigh {
		t.Errorf("Confidence = %q, want high", s.Confidence)
	}
}

func TestTuneGenerator_SuggestToolAccuracyRegression(t *testing.T) {
	gen := NewTuneGenerator()
	delta := &EvalDelta{
		Regressions: []DeltaEntry{
			{
				Dimension: "d2.tool_accuracy",
				Previous:  0.95,
				Current:   0.80,
				Delta:     -0.15,
				Severity:  SeverityRegression,
			},
		},
	}

	suggestions := gen.Suggest(delta)
	if len(suggestions) != 1 {
		t.Fatalf("len(suggestions) = %d, want 1", len(suggestions))
	}
	if suggestions[0].Target != "context_engine.harness.tool_pool.simple_mode" {
		t.Errorf("Target = %q", suggestions[0].Target)
	}
}

func TestTuneGenerator_NoRegressions(t *testing.T) {
	gen := NewTuneGenerator()
	if got := gen.Suggest(nil); got != nil {
		t.Errorf("Suggest(nil) = %v, want nil", got)
	}
	if got := gen.Suggest(&EvalDelta{}); got != nil {
		t.Errorf("Suggest(empty) = %v, want nil", got)
	}
}
