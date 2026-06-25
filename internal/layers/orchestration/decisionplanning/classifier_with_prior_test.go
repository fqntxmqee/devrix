package decisionplanning

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
)

func TestRuleClassifier_ClassifyWithPrior_UsePriorBeta(t *testing.T) {
	c := NewRuleClassifier(orchtypes.DefaultConfig())
	// prior Beta(8,3) → Mean = 8/11 ≈ 0.727
	prior := learn.BuildAdaptivePrior(nil, learn.TrackModeOperator)
	prior.PriorBeta = learn.BetaPrior{Alpha: 8, Beta: 3}

	result, err := c.ClassifyWithPrior(context.Background(), "hello", prior)
	if err != nil {
		t.Fatalf("ClassifyWithPrior: %v", err)
	}
	if result.Kind != orchtypes.IntentFast {
		t.Errorf("Kind = %q, want %q", result.Kind, orchtypes.IntentFast)
	}
	// Baseline 95 × 0.727 = 69.06 → 69
	mean := 8.0 / 11.0
	wantConfidence := int(float64(95) * mean)
	if result.Confidence != wantConfidence {
		t.Errorf("Confidence = %d, want %d (baseline 95 × mean 0.727)", result.Confidence, wantConfidence)
	}
}

func TestRuleClassifier_ClassifyWithPrior_NilPrior_UseBaseline(t *testing.T) {
	c := NewRuleClassifier(orchtypes.DefaultConfig())
	baseline, _ := c.Classify(context.Background(), "hello")
	withNil, err := c.ClassifyWithPrior(context.Background(), "hello", nil)
	if err != nil {
		t.Fatalf("ClassifyWithPrior nil: %v", err)
	}
	if withNil.Confidence != baseline.Confidence {
		t.Errorf("With nil prior: Confidence = %d, want %d (baseline)", withNil.Confidence, baseline.Confidence)
	}
	if withNil.Kind != baseline.Kind {
		t.Errorf("With nil prior: Kind = %q, want %q", withNil.Kind, baseline.Kind)
	}
	if withNil.Reason != baseline.Reason {
		t.Errorf("With nil prior: Reason = %q, want %q (no prior adjustment)", withNil.Reason, baseline.Reason)
	}
}

func TestRuleClassifier_ClassifyWithPrior_ColdStart_UsesDefaultDeveloperPrior(t *testing.T) {
	c := NewRuleClassifier(orchtypes.DefaultConfig())
	// Cold start: BuildAdaptivePrior(nil, developer) = Beta(5,3) → Mean = 0.625
	prior := learn.BuildAdaptivePrior(nil, learn.TrackModeDeveloper)
	withCold, err := c.ClassifyWithPrior(context.Background(), "hello", prior)
	if err != nil {
		t.Fatalf("ClassifyWithPrior cold: %v", err)
	}
	// Baseline 95 × 0.625 = 59.375 → 59
	mean := 5.0 / 8.0
	wantConfidence := int(float64(95) * mean)
	if withCold.Confidence != wantConfidence {
		t.Errorf("With cold-start prior (DefaultDeveloperPrior Mean=0.625): Confidence = %d, want %d", withCold.Confidence, wantConfidence)
	}
}

func TestRuleClassifier_ClassifyWithPrior_ZeroMean_NoChange(t *testing.T) {
	c := NewRuleClassifier(orchtypes.DefaultConfig())
	// Manually constructed prior with Mean=0 (e.g. adversarial injection)
	prior := &learn.AdaptivePrior{
		PriorBeta: learn.BetaPrior{Alpha: 0, Beta: 0},
	}
	baseline, _ := c.Classify(context.Background(), "hello")
	withZero, err := c.ClassifyWithPrior(context.Background(), "hello", prior)
	if err != nil {
		t.Fatalf("ClassifyWithPrior zero: %v", err)
	}
	if withZero.Confidence != baseline.Confidence {
		t.Errorf("With zero-mean prior: Confidence = %d, want %d (baseline)", withZero.Confidence, baseline.Confidence)
	}
}

func TestRuleClassifier_ClassifyWithPrior_NoPriorMutation(t *testing.T) {
	c := NewRuleClassifier(orchtypes.DefaultConfig())
	prior := learn.BuildAdaptivePrior(nil, learn.TrackModeDeveloper)
	alphaBefore := prior.PriorBeta.Alpha
	betaBefore := prior.PriorBeta.Beta

	_, _ = c.ClassifyWithPrior(context.Background(), "hello", prior)

	if prior.PriorBeta.Alpha != alphaBefore {
		t.Errorf("Prior.Alpha mutated: %d → %d", alphaBefore, prior.PriorBeta.Alpha)
	}
	if prior.PriorBeta.Beta != betaBefore {
		t.Errorf("Prior.Beta mutated: %d → %d", betaBefore, prior.PriorBeta.Beta)
	}
}

func TestRuleClassifier_ClassifyWithPrior_CommandUntouched(t *testing.T) {
	c := NewRuleClassifier(orchtypes.DefaultConfig())
	prior := learn.BuildAdaptivePrior(nil, learn.TrackModeOperator)
	prior.PriorBeta = learn.BetaPrior{Alpha: 8, Beta: 3}

	// Commands have Confidence=100; with prior Mean=0.727 → 100 × 0.727 = 72 (clamp-safe)
	result, err := c.ClassifyWithPrior(context.Background(), "/help", prior)
	if err != nil {
		t.Fatalf("ClassifyWithPrior command: %v", err)
	}
	if result.Kind != orchtypes.IntentCommand {
		t.Errorf("Kind = %q, want %q", result.Kind, orchtypes.IntentCommand)
	}
	if result.Command != "/help" {
		t.Errorf("Command = %q, want %q", result.Command, "/help")
	}
	mean := 8.0 / 11.0
	wantConfidence := int(float64(100) * mean)
	if result.Confidence != wantConfidence {
		t.Errorf("Command Confidence = %d, want %d", result.Confidence, wantConfidence)
	}
}

func TestRuleClassifier_ClassifyWithPrior_HighMean_ClampTo100(t *testing.T) {
	c := NewRuleClassifier(orchtypes.DefaultConfig())
	// prior Beta(1000,1) → Mean ≈ 0.999
	prior := learn.BuildAdaptivePrior(nil, learn.TrackModeDeveloper)
	prior.PriorBeta = learn.BetaPrior{Alpha: 1000, Beta: 1}

	result, err := c.ClassifyWithPrior(context.Background(), "/help", prior)
	if err != nil {
		t.Fatalf("ClassifyWithPrior: %v", err)
	}
	if result.Confidence > 100 {
		t.Errorf("Confidence = %d, expected <= 100 (clamp high)", result.Confidence)
	}
}
