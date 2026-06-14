package coordinator

import (
	"context"
	"testing"
)

// T: D7-S5-T03 — ClassifyIntent rule engine covers fast/command/skip/orchestrate.
func TestRuleClassifier_Classify_Empty(t *testing.T) {
	c := NewRuleClassifier(DefaultConfig())
	got, err := c.Classify(context.Background(), "   \n  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Kind != IntentSkip {
		t.Fatalf("want IntentSkip, got %q", got.Kind)
	}
	if got.Confidence != 100 {
		t.Fatalf("want 100 confidence for skip, got %d", got.Confidence)
	}
}

// T: D7-S5-T03 (command-first) — recognized commands short-circuit LLM.
func TestRuleClassifier_Classify_CommandFirst(t *testing.T) {
	c := NewRuleClassifier(DefaultConfig())
	tests := []struct {
		name string
		in   string
		want IntentKind
		cmd  string
	}{
		{"plan command", "/plan add auth", IntentCommand, "/plan"},
		{"stop command", "/stop", IntentCommand, "/stop"},
		{"task command", "/task list", IntentCommand, "/task"},
		{"help command", "/help me", IntentCommand, "/help"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.Classify(context.Background(), tt.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Kind != tt.want {
				t.Fatalf("want %q, got %q (reason=%s)", tt.want, got.Kind, got.Reason)
			}
			if got.Command != tt.cmd {
				t.Fatalf("want command %q, got %q", tt.cmd, got.Command)
			}
			if got.Confidence != 100 {
				t.Fatalf("want 100 confidence for command, got %d", got.Confidence)
			}
		})
	}
}

// T: D7-S2-T02b — fast path rule matches within 1ms.
func TestRuleClassifier_Classify_FastPath(t *testing.T) {
	c := NewRuleClassifier(DefaultConfig())
	tests := []struct {
		name string
		in   string
	}{
		{"hello", "hello"},
		{"chinese greeting", "你好"},
		{"thanks", "thanks!"},
		{"status", "/status"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.Classify(context.Background(), tt.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Kind != IntentFast {
				t.Fatalf("want IntentFast, got %q (reason=%s)", got.Kind, got.Reason)
			}
			if got.Confidence < 70 {
				t.Fatalf("want confidence ≥ 70, got %d", got.Confidence)
			}
		})
	}
}

// T: D7-S5-T03 (negative) — complex message falls through to orchestrate.
func TestRuleClassifier_Classify_Orchestrate(t *testing.T) {
	c := NewRuleClassifier(DefaultConfig())
	got, err := c.Classify(context.Background(),
		"investigate the auth module latency and propose a refactor plan with milestones")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Kind != IntentOrchestrate {
		t.Fatalf("want IntentOrchestrate, got %q (reason=%s)", got.Kind, got.Reason)
	}
}

// T: D7-S5-T03 — short single-token messages default to fast with 70 confidence.
func TestRuleClassifier_Classify_ShortDefaultsFast(t *testing.T) {
	c := NewRuleClassifier(DefaultConfig())
	got, err := c.Classify(context.Background(), "what time is it")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Kind != IntentFast {
		t.Fatalf("want IntentFast, got %q", got.Kind)
	}
	if got.Confidence != 70 {
		t.Fatalf("want 70 confidence for short default, got %d", got.Confidence)
	}
}

// T: D7-S5-T06 (negative) — CommandFirst=false 时 /plan 不再匹配 IntentCommand。
// 断言不限定具体回退 Kind，避免规则微调时回归脆弱。
func TestRuleClassifier_Classify_CommandFirst_Disabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CommandFirst = false
	c := NewRuleClassifier(cfg)
	got, err := c.Classify(context.Background(), "/plan add auth")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Kind == IntentCommand {
		t.Fatalf("CommandFirst=false should not match IntentCommand, got kind=%q reason=%q",
			got.Kind, got.Reason)
	}
}
