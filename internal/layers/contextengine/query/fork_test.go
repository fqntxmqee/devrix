package query_test

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/conversation"
	"github.com/devrix/devrix/internal/layers/contextengine/query"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D2-S10-A01-T41
func TestBuildForkedMessages_should_use_identical_placeholder_results(t *testing.T) {
	calls := `[{"id":"c1","type":"function","function":{"name":"bash","arguments":"{}"}},{"id":"c2","type":"function","function":{"name":"read_file","arguments":"{}"}}]`
	parent := []types.Message{
		{Role: types.MessageRoleUser, Content: "analyze repo"},
		{Role: types.MessageRoleAssistant, Content: "running tools", Metadata: map[string]string{conversation.MetaToolCalls: calls}},
	}

	a := query.BuildForkedMessages("scope auth module", parent)
	b := query.BuildForkedMessages("scope billing module", parent)
	if len(a) != 2 || len(b) != 2 {
		t.Fatalf("expected 2 fork messages, got %d and %d", len(a), len(b))
	}

	fpA := query.ForkPrefixFingerprint(a)
	fpB := query.ForkPrefixFingerprint(b)
	if fpA != fpB {
		t.Fatal("fork prefix fingerprint should match for cache sharing")
	}
	if !containsAll(a[1].Content, query.ForkPlaceholderResult, "scope auth module") {
		t.Fatal("fork A should contain placeholder and directive")
	}
	if !containsAll(b[1].Content, query.ForkPlaceholderResult, "scope billing module") {
		t.Fatal("fork B should contain placeholder and directive")
	}
	if a[1].Content == b[1].Content {
		t.Fatal("directives should differ per child")
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
