package sessionorchestrator

import "github.com/devrix/devrix/internal/layers/orchestration/orchtypes"

import "testing"

func TestRouteLabel_should_map_intent_kinds(t *testing.T) {
	t.Helper()
	cases := []struct {
		intent orchtypes.IntentClassification
		want   string
	}{
		{orchtypes.IntentClassification{Kind: orchtypes.IntentFast}, "fast"},
		{orchtypes.IntentClassification{Kind: orchtypes.IntentFast, Reason: "loop_first_default"}, "turn"},
		{orchtypes.IntentClassification{Kind: orchtypes.IntentCommand, Command: "/plan"}, "command"},
		{orchtypes.IntentClassification{Kind: orchtypes.IntentOrchestrate}, "orchestrate"},
		{orchtypes.IntentClassification{Kind: orchtypes.IntentSkip}, "skip"},
	}
	for _, tc := range cases {
		if got := routeLabel(tc.intent); got != tc.want {
			t.Fatalf("routeLabel(%q) = %q, want %q", tc.intent.Kind, got, tc.want)
		}
	}
}

func TestIntentClassifyAttrs_should_include_command_when_present(t *testing.T) {
	t.Helper()
	attrs := intentClassifyAttrs(orchtypes.IntentClassification{
		Kind:       orchtypes.IntentCommand,
		Confidence: 100,
		Reason:     "whitelist",
		Command:    "/help",
	}, "rule")
	if len(attrs) < 4 {
		t.Fatalf("expected at least 4 attrs, got %d", len(attrs))
	}
	foundCommand := false
	for _, attr := range attrs {
		if attr.Key == "orchestration.command" && attr.Value == "/help" {
			foundCommand = true
		}
	}
	if !foundCommand {
		t.Fatal("expected orchestration.command=/help in attrs")
	}
}
