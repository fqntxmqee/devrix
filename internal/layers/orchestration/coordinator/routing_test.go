package coordinator

import (
	"context"
	"testing"
)

func TestConfig_IsLoopFirst_Default(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	if !cfg.IsLoopFirst() {
		t.Fatal("default routing mode should be loop_first")
	}
}

func TestRuleClassifier_LoopFirst_DefaultTurn(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RoutingMode = RoutingModeLoopFirst
	c := NewRuleClassifier(cfg)
	got, err := c.Classify(context.Background(), "investigate auth latency and propose refactor")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if got.Kind != IntentFast {
		t.Fatalf("kind = %q, want fast (turn)", got.Kind)
	}
	if got.Confidence != 100 {
		t.Fatalf("confidence = %d, want 100", got.Confidence)
	}
	if got.Reason != "loop_first_default" {
		t.Fatalf("reason = %q", got.Reason)
	}
}

func TestRuleClassifier_LoopFirst_Greeting(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RoutingMode = RoutingModeLoopFirst
	c := NewRuleClassifier(cfg)
	got, err := c.Classify(context.Background(), "你好")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if got.Kind != IntentFast || got.Confidence < 90 {
		t.Fatalf("greeting should stay on turn path: %+v", got)
	}
}

func TestRuleClassifier_Legacy_OrchestrateFallback(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RoutingMode = RoutingModeRuleOrchestrate
	c := NewRuleClassifier(cfg)
	got, err := c.Classify(context.Background(), "investigate auth module latency and propose a detailed refactor plan with tests")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if got.Kind != IntentOrchestrate {
		t.Fatalf("kind = %q, want orchestrate in legacy mode", got.Kind)
	}
}
