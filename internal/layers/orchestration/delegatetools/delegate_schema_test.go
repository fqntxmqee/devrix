package delegatetools

import (
	"encoding/json"
	"testing"
)

// T: DM-20260620-001-B / B.2.4 (D4-S4-A07-T01) — delegate_* tool schema
// declares `mode` enum [brief|fork|full], default brief, with description
// referencing Phase B sub-agent isolation. Missing field = AC10 not wired.
func TestDelegateToolParameters_ModeEnum(t *testing.T) {
	raw := delegateToolParameters()
	var schema map[string]any
	if err := json.Unmarshal([]byte(raw), &schema); err != nil {
		t.Fatalf("delegateToolParameters: invalid JSON: %v", err)
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("delegate schema: properties missing: %+v", schema)
	}
	mode, ok := props["mode"].(map[string]any)
	if !ok {
		t.Fatalf("delegate schema: mode property missing: %+v", props)
	}
	enum, ok := mode["enum"].([]any)
	if !ok {
		t.Fatalf("delegate schema: mode.enum missing or wrong type: %+v", mode)
	}
	got := map[string]bool{}
	for _, e := range enum {
		if s, ok := e.(string); ok {
			got[s] = true
		}
	}
	for _, want := range []string{"brief", "fork", "full"} {
		if !got[want] {
			t.Errorf("delegate schema: mode.enum missing %q; got %v", want, enum)
		}
	}
	if def, _ := mode["default"].(string); def != "brief" {
		t.Errorf("delegate schema: mode.default = %q, want \"brief\"", def)
	}
}

// T: DM-20260620-001-B / B.2.4 — parseSubAgentMode normalizes the tool-input
// mode string into contracts.SubAgentMode. Unknown / empty → "" (so
// SubTurnRunner falls back to Cfg.DefaultMode).
func TestParseSubAgentMode(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"brief", "brief"},
		{"fork", "fork"},
		{"full", "full"},
		{"BRIEF", "brief"},
		{"  fork  ", "fork"},
		{"", ""},
		{"unknown", ""},
	}
	for _, c := range cases {
		got := string(parseSubAgentMode(c.in))
		if got != c.want {
			t.Errorf("parseSubAgentMode(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
