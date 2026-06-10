package eval

import "testing"

func TestDeltaAnalyzer_NoBaseline(t *testing.T) {
	da := NewDeltaAnalyzer(nil)
	report := &EvalReport{ID: "test", Scores: []DomainScore{{Domain: "d2", Dimension: "test", Score: 0.8}}}

	delta := da.Compare(report)
	if delta != nil {
		t.Error("expected nil delta when no baseline")
	}
}

func TestDeltaAnalyzer_NilCurrent(t *testing.T) {
	baseline := &EvalReport{ID: "baseline", Scores: []DomainScore{{Domain: "d2", Dimension: "test", Score: 0.8}}}
	da := NewDeltaAnalyzer(baseline)

	delta := da.Compare(nil)
	if delta != nil {
		t.Error("expected nil delta when current is nil")
	}
}

func TestDeltaAnalyzer_SameConfig(t *testing.T) {
	baseline := &EvalReport{
		ID: "baseline",
		Scores: []DomainScore{
			{Domain: "d2", Dimension: "compression_recall", Score: 0.85},
		},
	}
	current := &EvalReport{
		ID: "current",
		Scores: []DomainScore{
			{Domain: "d2", Dimension: "compression_recall", Score: 0.85},
		},
	}

	da := NewDeltaAnalyzer(baseline)
	delta := da.Compare(current)

	if delta == nil {
		t.Fatal("delta should not be nil")
	}
	entry, ok := delta.ByDimension["d2.compression_recall"]
	if !ok {
		t.Fatal("missing dimension in delta")
	}
	if entry.Severity != SeverityStable {
		t.Errorf("severity = %s, want stable", entry.Severity)
	}
	if entry.Delta != 0 {
		t.Errorf("delta = %v, want 0", entry.Delta)
	}
}

func TestDeltaAnalyzer_Regression(t *testing.T) {
	baseline := &EvalReport{
		ID: "baseline",
		Scores: []DomainScore{
			{Domain: "d2", Dimension: "compression_recall", Score: 0.85},
		},
	}
	current := &EvalReport{
		ID: "current",
		Scores: []DomainScore{
			{Domain: "d2", Dimension: "compression_recall", Score: 0.50},
		},
	}

	da := NewDeltaAnalyzer(baseline)
	delta := da.Compare(current)

	if delta == nil {
		t.Fatal("delta should not be nil")
	}
	if len(delta.Regressions) == 0 {
		t.Fatal("expected regression, got none")
	}
	entry := delta.Regressions[0]
	if entry.Severity != SeverityRegression {
		t.Errorf("severity = %s, want regression", entry.Severity)
	}
}

func TestDeltaAnalyzer_Improvement(t *testing.T) {
	baseline := &EvalReport{
		ID: "baseline",
		Scores: []DomainScore{
			{Domain: "d2", Dimension: "compression_recall", Score: 0.50},
		},
	}
	current := &EvalReport{
		ID: "current",
		Scores: []DomainScore{
			{Domain: "d2", Dimension: "compression_recall", Score: 0.85},
		},
	}

	da := NewDeltaAnalyzer(baseline)
	delta := da.Compare(current)

	entry := delta.ByDimension["d2.compression_recall"]
	if entry.Severity != SeverityImprovement {
		t.Errorf("severity = %s, want improvement", entry.Severity)
	}
}

func TestDeltaAnalyzer_MultipleDimensions(t *testing.T) {
	baseline := &EvalReport{
		ID: "baseline",
		Scores: []DomainScore{
			{Domain: "d2", Dimension: "compression_recall", Score: 0.85},
			{Domain: "d2", Dimension: "pev_tool_accuracy", Score: 0.90},
		},
	}
	current := &EvalReport{
		ID: "current",
		Scores: []DomainScore{
			{Domain: "d2", Dimension: "compression_recall", Score: 0.80},
			{Domain: "d2", Dimension: "pev_tool_accuracy", Score: 0.95},
		},
	}

	da := NewDeltaAnalyzer(baseline)
	delta := da.Compare(current)

	if len(delta.ByDimension) != 2 {
		t.Errorf("ByDimension count = %d, want 2", len(delta.ByDimension))
	}
}

func TestDeltaAnalyzer_BucketCompare(t *testing.T) {
	baseline := &EvalReport{
		ID: "baseline",
		Scores: []DomainScore{
			{
				Domain: "d2", Dimension: "compression_recall", Score: 0.85,
				Buckets: map[string]float64{"production": 0.90, "adversarial": 0.70},
			},
		},
	}
	current := &EvalReport{
		ID: "current",
		Scores: []DomainScore{
			{
				Domain: "d2", Dimension: "compression_recall", Score: 0.80,
				Buckets: map[string]float64{"production": 0.88, "adversarial": 0.60},
			},
		},
	}

	da := NewDeltaAnalyzer(baseline)
	delta := da.Compare(current)

	if len(delta.ByBucket) == 0 {
		t.Fatal("expected bucket comparison")
	}
}
