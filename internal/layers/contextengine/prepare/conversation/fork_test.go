package conversation_test

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D2-S10-A01-T41
func TestBuildForkedMessages_should_use_identical_placeholder_results(t *testing.T) {
	calls := `[{"id":"c1","type":"function","function":{"name":"bash","arguments":"{}"}},{"id":"c2","type":"function","function":{"name":"read_file","arguments":"{}"}}]`
	parent := []types.Message{
		{Role: types.MessageRoleUser, Content: "analyze repo"},
		{Role: types.MessageRoleAssistant, Content: "running tools", Metadata: map[string]string{conversation.MetaToolCalls: calls}},
	}

	a := conversation.BuildForkedMessages("scope auth module", parent)
	b := conversation.BuildForkedMessages("scope billing module", parent)
	if len(a) != 2 || len(b) != 2 {
		t.Fatalf("expected 2 fork messages, got %d and %d", len(a), len(b))
	}

	fpA := conversation.ForkPrefixFingerprint(a)
	fpB := conversation.ForkPrefixFingerprint(b)
	if fpA != fpB {
		t.Fatal("fork prefix fingerprint should match for cache sharing")
	}
	if !containsAll(a[1].Content, conversation.ForkPlaceholderResult, "scope auth module") {
		t.Fatal("fork A should contain placeholder and directive")
	}
	if !containsAll(b[1].Content, conversation.ForkPlaceholderResult, "scope billing module") {
		t.Fatal("fork B should contain placeholder and directive")
	}
	if a[1].Content == b[1].Content {
		t.Fatal("directives should differ per child")
	}
}

// T: D2-S15-A08-T07
func TestBuildForkedMessages_NoToolCallsFallback(t *testing.T) {
	parent := []types.Message{
		{Role: types.MessageRoleUser, Content: "just a directive"},
		{Role: types.MessageRoleAssistant, Content: "no tool calls here"},
	}
	out := conversation.BuildForkedMessages("analyze billing", parent)
	if len(out) != 1 {
		t.Fatalf("no tool_calls fallback: expected 1 directive message, got %d", len(out))
	}
	if out[0].Role != types.MessageRoleUser {
		t.Fatalf("expected user-role directive, got %s", out[0].Role)
	}
	if !strings.Contains(out[0].Content, "analyze billing") {
		t.Fatalf("directive user should embed the directive, got %q", out[0].Content)
	}
	// No tool_calls → no placeholder anchor (nothing to anchor to).
	// This is the documented limitation: fork mode without prior tool_calls
	// is not a meaningful fork candidate; SubTurnRunner typically rejects
	// such requests before reaching the helper.
	if strings.Contains(out[0].Content, conversation.ForkPlaceholderResult) {
		t.Fatalf("no-tool-calls fallback should not include placeholder, got %q", out[0].Content)
	}
}

// T: D2-S15-A08-T08
func TestBuildForkedMessages_MultipleToolCallPlaceholders(t *testing.T) {
	calls := `[{"id":"c1","type":"function","function":{"name":"bash","arguments":"{}"}},{"id":"c2","type":"function","function":{"name":"read_file","arguments":"{}"}},{"id":"c3","type":"function","function":{"name":"grep","arguments":"{}"}}]`
	parent := []types.Message{
		{Role: types.MessageRoleUser, Content: "explore"},
		{Role: types.MessageRoleAssistant, Content: "running 3 tools", Metadata: map[string]string{conversation.MetaToolCalls: calls}},
	}
	out := conversation.BuildForkedMessages("scope all", parent)
	if len(out) != 2 {
		t.Fatalf("expected 2 fork messages, got %d", len(out))
	}
	if out[0].Role != types.MessageRoleAssistant {
		t.Fatalf("expected assistant first, got %s", out[0].Role)
	}
	// Count placeholder occurrences — one per tool_call ID.
	for _, id := range []string{"c1", "c2", "c3"} {
		want := "[tool_result id=" + id + "]\n" + conversation.ForkPlaceholderResult
		if !strings.Contains(out[1].Content, want) {
			t.Fatalf("missing placeholder block for %s in directive user:\n%s", id, out[1].Content)
		}
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if p == "" {
			continue
		}
		if len(s) < len(p) || !containsSubstring(s, p) {
			return false
		}
	}
	return true
}

func containsSubstring(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
