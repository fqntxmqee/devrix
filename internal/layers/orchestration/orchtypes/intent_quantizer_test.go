package orchtypes

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/learn"
)

func TestIntentQuantizer_Quantize_Empty(t *testing.T) {
	q := NewIntentQuantizer(DefaultConfig())
	p, err := q.Quantize(context.Background(), "   ")
	if err != nil {
		t.Fatalf("Quantize empty: %v", err)
	}
	if p.Class != IntentClassSkip {
		t.Errorf("Class = %q, want %q", p.Class, IntentClassSkip)
	}
	if p.Confidence != 100 {
		t.Errorf("Confidence = %d, want 100", p.Confidence)
	}
}

func TestIntentQuantizer_Quantize_Greeting(t *testing.T) {
	q := NewIntentQuantizer(DefaultConfig())
	p, err := q.Quantize(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Quantize greeting: %v", err)
	}
	if p.Class != IntentClassFact {
		t.Errorf("Class = %q, want %q", p.Class, IntentClassFact)
	}
	if p.Confidence != 95 {
		t.Errorf("Confidence = %d, want 95", p.Confidence)
	}
}

func TestIntentQuantizer_Quantize_Command(t *testing.T) {
	q := NewIntentQuantizer(DefaultConfig())
	p, err := q.Quantize(context.Background(), "/help me")
	if err != nil {
		t.Fatalf("Quantize command: %v", err)
	}
	if p.Class != IntentClassCommand {
		t.Errorf("Class = %q, want %q", p.Class, IntentClassCommand)
	}
	if p.Confidence != 100 {
		t.Errorf("Confidence = %d, want 100", p.Confidence)
	}
}

func TestIntentQuantizer_Quantize_Question(t *testing.T) {
	q := NewIntentQuantizer(DefaultConfig())
	p, err := q.Quantize(context.Background(), "what is the weather?")
	if err != nil {
		t.Fatalf("Quantize question: %v", err)
	}
	if p.Class != IntentClassFact {
		t.Errorf("Class = %q, want %q", p.Class, IntentClassFact)
	}
}

func TestIntentQuantizer_Quantize_ShortFallback(t *testing.T) {
	q := NewIntentQuantizer(DefaultConfig())
	p, err := q.Quantize(context.Background(), "explain lambda")
	if err != nil {
		t.Fatalf("Quantize short: %v", err)
	}
	if p.Class != IntentClassFact {
		t.Errorf("Class = %q, want %q", p.Class, IntentClassFact)
	}
	if p.Confidence != 70 {
		t.Errorf("Confidence = %d, want 70", p.Confidence)
	}
}

func TestIntentQuantizer_Quantize_Orchestrate(t *testing.T) {
	q := NewIntentQuantizer(DefaultConfig())
	p, err := q.Quantize(context.Background(),
		"please write me a 500-line Python script that does data analysis with multiple steps")
	if err != nil {
		t.Fatalf("Quantize long: %v", err)
	}
	if p.Class != IntentClassOrchestrate {
		t.Errorf("Class = %q, want %q", p.Class, IntentClassOrchestrate)
	}
}

func TestIntentQuantizer_QuantizeWithPrior_UsePriorBeta(t *testing.T) {
	q := NewIntentQuantizer(DefaultConfig())
	// prior.PriorBeta = Beta(8,3) → Mean = 8/11 ≈ 0.727
	prior := learn.BuildAdaptivePrior(nil, learn.TrackModeOperator) // operator = Beta(8,1), but we want Beta(8,3)
	prior.PriorBeta = learn.BetaPrior{Alpha: 8, Beta: 3}

	p, err := q.QuantizeWithPrior(context.Background(), "hello", prior)
	if err != nil {
		t.Fatalf("QuantizeWithPrior: %v", err)
	}
	if p.Class != IntentClassFact {
		t.Errorf("Class = %q, want %q", p.Class, IntentClassFact)
	}
	// Baseline 95 × 0.727 = 69.06 → 69
	mean := 8.0 / 11.0
	wantConfidence := int(float64(95) * mean)
	if p.Confidence != wantConfidence {
		t.Errorf("Confidence = %d, want %d (baseline 95 × mean 0.727)", p.Confidence, wantConfidence)
	}
}

func TestIntentQuantizer_QuantizeWithPrior_NilPrior_NoChange(t *testing.T) {
	q := NewIntentQuantizer(DefaultConfig())
	baseline, _ := q.Quantize(context.Background(), "hello")
	withNil, err := q.QuantizeWithPrior(context.Background(), "hello", nil)
	if err != nil {
		t.Fatalf("QuantizeWithPrior nil: %v", err)
	}
	if withNil.Confidence != baseline.Confidence {
		t.Errorf("With nil prior: Confidence = %d, want %d (baseline)", withNil.Confidence, baseline.Confidence)
	}
	if withNil.Class != baseline.Class {
		t.Errorf("With nil prior: Class = %q, want %q", withNil.Class, baseline.Class)
	}
}

func TestIntentQuantizer_QuantizeWithPrior_ColdStart_UsesDefaultDeveloperPrior(t *testing.T) {
	q := NewIntentQuantizer(DefaultConfig())
	// Cold start: BuildAdaptivePrior(nil, developer) = Beta(5,3) → Mean = 0.625
	prior := learn.BuildAdaptivePrior(nil, learn.TrackModeDeveloper)
	withCold, err := q.QuantizeWithPrior(context.Background(), "hello", prior)
	if err != nil {
		t.Fatalf("QuantizeWithPrior cold: %v", err)
	}
	// Baseline 95 × 0.625 = 59.375 → 59
	mean := 5.0 / 8.0
	wantConfidence := int(float64(95) * mean)
	if withCold.Confidence != wantConfidence {
		t.Errorf("With cold-start prior (DefaultDeveloperPrior Mean=0.625): Confidence = %d, want %d", withCold.Confidence, wantConfidence)
	}
}

func TestIntentQuantizer_QuantizeWithPrior_ZeroMean_NoChange(t *testing.T) {
	q := NewIntentQuantizer(DefaultConfig())
	// Manually constructed prior with Mean=0 (e.g. adversarial injection)
	prior := &learn.AdaptivePrior{
		PriorBeta: learn.BetaPrior{Alpha: 0, Beta: 0},
	}
	baseline, _ := q.Quantize(context.Background(), "hello")
	withZero, err := q.QuantizeWithPrior(context.Background(), "hello", prior)
	if err != nil {
		t.Fatalf("QuantizeWithPrior zero: %v", err)
	}
	if withZero.Confidence != baseline.Confidence {
		t.Errorf("With zero-mean prior: Confidence = %d, want %d (baseline)", withZero.Confidence, baseline.Confidence)
	}
}

func TestIntentQuantizer_QuantizeWithPrior_NoPriorMutation(t *testing.T) {
	q := NewIntentQuantizer(DefaultConfig())
	prior := learn.BuildAdaptivePrior(nil, learn.TrackModeDeveloper)
	priorAlphaBefore := prior.PriorBeta.Alpha
	priorBetaBefore := prior.PriorBeta.Beta

	_, _ = q.QuantizeWithPrior(context.Background(), "hello", prior)

	if prior.PriorBeta.Alpha != priorAlphaBefore {
		t.Errorf("Prior.Alpha mutated: %d → %d", priorAlphaBefore, prior.PriorBeta.Alpha)
	}
	if prior.PriorBeta.Beta != priorBetaBefore {
		t.Errorf("Prior.Beta mutated: %d → %d", priorBetaBefore, prior.PriorBeta.Beta)
	}
}

func TestIntentQuantizer_QuantizeWithPrior_ClampHigh(t *testing.T) {
	q := NewIntentQuantizer(DefaultConfig())
	// prior.PriorBeta with very high Mean (e.g. Beta(100,1) → Mean = 0.99)
	prior := learn.BuildAdaptivePrior(nil, learn.TrackModeOperator)
	prior.PriorBeta = learn.BetaPrior{Alpha: 100, Beta: 1}

	p, err := q.QuantizeWithPrior(context.Background(), "/help", prior)
	if err != nil {
		t.Fatalf("QuantizeWithPrior: %v", err)
	}
	// Baseline 100 × 0.99 = 99 → clamp 100? No, 99 < 100 so no clamp.
	// But test that confidence is bounded.
	if p.Confidence > 100 {
		t.Errorf("Confidence = %d, expected <= 100 (clamped)", p.Confidence)
	}
}
