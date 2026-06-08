package collaboration

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/multiagent"
)

func TestBuildPromptForMode_should_enhance_chain_of_thought(t *testing.T) {
	out := BuildPromptForMode(multiagent.ModeChainOfThought, "base")
	if out == "base" {
		t.Fatal("expected prompt enhancement")
	}
	if !contains(out, "Chain-of-Thought") {
		t.Fatalf("prompt = %q", out)
	}
}

func TestValidateMode_should_reject_unknown(t *testing.T) {
	if err := ValidateMode("unknown-mode"); err == nil {
		t.Fatal("expected validation error")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexSubstring(s, sub))
}

func indexSubstring(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
