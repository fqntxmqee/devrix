// Tests for the D6 LayerViolationProbe.
//
// Covers: L5-0-0-04  (D6 LayerViolationProbe registered and returns a score)
// Domain: D6
// Stage: s0_unit
package eval

import (
	"context"
	"testing"
)

// TestLayerViolationProbe_IsRegistered verifies that init() has placed the
// probe in the global registry, so the eval runner can find it by ID.
func TestLayerViolationProbe_IsRegistered(t *testing.T) {
	p := GetProbe("layer_violation")
	if p == nil {
		t.Fatal("expected layer_violation probe to be registered")
	}
	if p.ID() != "layer_violation" {
		t.Errorf("want id=layer_violation, got %s", p.ID())
	}
}

// TestLayerViolationProbe_Run_NoViolationsOnHealthyCodebase runs the probe
// against a healthy synthetic EvalItem and verifies the score is 1.0 and the
// details map reports zero reverse imports.
func TestLayerViolationProbe_Run_NoViolationsOnHealthyCodebase(t *testing.T) {
	p := GetProbe("layer_violation").(*LayerViolationProbe)

	item := EvalItem{
		ID:     "layer-violation-healthy",
		Bucket: "layering",
		Domain: "d6",
		Input: map[string]any{
			"root": "internal/layers",
		},
	}

	score, err := p.Run(context.Background(), item, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if score.Domain != "d6" {
		t.Errorf("want domain=d6, got %s", score.Domain)
	}
	if score.Details["reverse_import_count"] != 0 {
		t.Errorf("want reverse_import_count=0 on healthy codebase, got %v",
			score.Details["reverse_import_count"])
	}
	if score.Score != 1.0 {
		t.Errorf("want score=1.0 (no violations), got %v", score.Score)
	}
}

// TestLayerViolationProbe_ScoreFormula verifies the scoring contract:
//   - 0 violations → 1.0
//   - 1 violation  → 0.5
//   - 2+ violations → linearly decreased
func TestLayerViolationProbe_ScoreFormula(t *testing.T) {
	p := &LayerViolationProbe{}
	if got := p.scoreFromViolations(0); got != 1.0 {
		t.Errorf("scoreFromViolations(0) = %v, want 1.0", got)
	}
	if got := p.scoreFromViolations(1); got != 0.5 {
		t.Errorf("scoreFromViolations(1) = %v, want 0.5", got)
	}
	if got := p.scoreFromViolations(2); got != 0.0 {
		t.Errorf("scoreFromViolations(2) = %v, want 0.0", got)
	}
}
