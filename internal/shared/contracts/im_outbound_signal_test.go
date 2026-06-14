package contracts

import (
	"testing"
	"time"
)

func TestMapEngineEventToSignal_kinds(t *testing.T) {
	start := time.Now()
	cases := []struct {
		typ  string
		kind SignalKind
		term bool
	}{
		{"thinking", SignalThinking, false},
		{"tool_call", SignalTask, false},
		{"text", SignalConclusion, false},
		{"complete", SignalConclusion, true},
		{"error", SignalConclusion, true},
	}
	for _, tc := range cases {
		sig, ok := MapEngineEventToSignal(&EngineEvent{Type: tc.typ, SessionID: "s1"}, 1, "turn-1", start)
		if !ok {
			t.Fatalf("%s: not mapped", tc.typ)
		}
		if sig.Kind != tc.kind {
			t.Fatalf("%s: kind=%q want %q", tc.typ, sig.Kind, tc.kind)
		}
		if sig.IsTerminal != tc.term {
			t.Fatalf("%s: terminal=%v want %v", tc.typ, sig.IsTerminal, tc.term)
		}
	}
}

func TestParseConclusionFeedback(t *testing.T) {
	ok, reason := ParseConclusionFeedback("/feedback wrong answer")
	if !ok || reason != "wrong answer" {
		t.Fatalf("feedback parse: ok=%v reason=%q", ok, reason)
	}
	ok, _ = ParseConclusionFeedback("hello")
	if ok {
		t.Fatal("expected non-feedback")
	}
}
