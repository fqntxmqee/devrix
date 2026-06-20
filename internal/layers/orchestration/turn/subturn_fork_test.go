package turn

import (
	"bytes"
	"context"
	"sync"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// TestSubTurnRunner_ForkSiblingPrefixStable (D2-S15-A08-T06) —
// AC11a invariant: multiple fork sub-agents spawned with identical parent
// messages produce byte-level identical prefix (prompt cache friendly).
// Future Anthropic cache_control integration benefits directly.
func TestSubTurnRunner_ForkSiblingPrefixStable(t *testing.T) {
	const siblings = 10

	// Fixed parent history — every fork child sees this same input.
	parent := []types.Message{
		{Role: types.MessageRoleUser, Content: "explore the auth subsystem"},
		{Role: types.MessageRoleAssistant, Content: "I'll search for auth-related code.",
			Metadata: map[string]string{"tool_calls": `[{"id":"t1","type":"function","function":{"name":"grep","arguments":"{\"pattern\":\"auth\"}"}},{"id":"t2","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"/auth/main.go\"}"}}]`}},
		{Role: types.MessageRoleUser, Content: "[tool_result t1] match: 12 files",
			Metadata: map[string]string{"tool_results": `[{"call_id":"t1","content":"match: 12 files"}]`}},
		{Role: types.MessageRoleUser, Content: "[tool_result t2] file contents",
			Metadata: map[string]string{"tool_results": `[{"call_id":"t2","content":"file contents"}]`}},
	}

	// Vary the directive per sibling (the LLM may pass different
	// sub-tasks to each fork child). Even with different directives,
	// the prefix portion (assistant + placeholder tool_results) must
	// stay byte-level identical.
	fingerprints := make([][]byte, siblings)
	directives := make([]string, siblings)
	for i := 0; i < siblings; i++ {
		directives[i] = "fork directive #" + string(rune('A'+i))
	}

	// Run siblings in parallel to also catch any non-determinism
	// (e.g. timestamp in IDs would break byte-level equality).
	var wg sync.WaitGroup
	wg.Add(siblings)
	for i := 0; i < siblings; i++ {
		go func(idx int) {
			defer wg.Done()
			llm := &captureStubLLM{chunks: []llmgateway.Chunk{textChunk("ok"), doneChunk()}}
			orch := NewOrchestrator(OrchestratorDeps{
				LLM: llm, Context: &stubContext{}, Tools: &stubTools{}, Persist: &stubPersist{}, MaxTurns: 4,
			})
			runner := NewSubTurnRunner(orch, SubTurnConfig{DefaultMode: "brief", MaxDepth: 3})

			sibling := append([]types.Message(nil), parent...)
			sibling = append(sibling, types.Message{
				Role:    types.MessageRoleUser,
				Content: directives[idx],
			})

			_, err := runner.RunSubTurn(context.Background(), contracts.SubTurnRequest{
				SessionID: "s",
				Messages:  sibling,
				Mode:      contracts.SubAgentModeFork,
			})
			if err != nil {
				t.Errorf("sibling %d: RunSubTurn: %v", idx, err)
				return
			}
			msgs := llm.lastMessages()
			// Prefix = all messages except the last (the original
			// directive message). The directive message itself is
			// allowed to vary per sibling.
			//
			// For the second-to-last message (the directive_user
			// synthesized by BuildForkedMessages), we use the D2
			// canonical ForkPrefixFingerprint helper which truncates
			// at the directive line — so only the placeholder +
			// boilerplate contribute to the fingerprint.
			if len(msgs) < 2 {
				t.Errorf("sibling %d: expected ≥2 messages, got %d", idx, len(msgs))
				return
			}
			prefixMsgs := msgs[:len(msgs)-1]
			fp := conversation.ForkPrefixFingerprint(prefixMsgs)
			fingerprints[idx] = []byte(fp)
		}(i)
	}
	wg.Wait()

	// Assert all fingerprints are byte-equal.
	for i := 1; i < siblings; i++ {
		if !bytes.Equal(fingerprints[0], fingerprints[i]) {
			t.Fatalf("sibling %d prefix differs from sibling 0:\n  0=%x\n  %d=%x", i, fingerprints[0], i, fingerprints[i])
		}
	}
}

// TestSubTurnRunner_ForkPrefix_ContainsPlaceholder (D2-S15-A08-T08) —
// AC11a invariant: every fork-mode prefix must contain the byte-level
// fixed placeholder literal "Fork started — processing in background".
// This guards the prompt cache contract: any change to the placeholder
// text invalidates cache for all prior fork children.
func TestSubTurnRunner_ForkPrefix_ContainsPlaceholder(t *testing.T) {
	llm, runner := buildSubTurnFixture(t)
	parent := []types.Message{
		{Role: types.MessageRoleAssistant, Content: "asst",
			Metadata: map[string]string{"tool_calls": `[{"id":"t1","type":"function","function":{"name":"x","arguments":"{}"}}]`}},
		{Role: types.MessageRoleUser, Content: "[tool_result t1] data",
			Metadata: map[string]string{"tool_results": `[{"call_id":"t1","content":"data"}]`}},
		{Role: types.MessageRoleUser, Content: "directive"},
	}
	_, err := runner.RunSubTurn(context.Background(), contracts.SubTurnRequest{
		SessionID: "s",
		Messages:  parent,
		Mode:      contracts.SubAgentModeFork,
	})
	if err != nil {
		t.Fatalf("RunSubTurn: %v", err)
	}
	got := llm.lastMessages()
	// Direct literal check — the placeholder must appear byte-exact
	// in at least one user message (the synthesized directive user
	// message from BuildForkedMessages), not a normalized/trimmed
	// variant. This is the cache contract: any future refactor that
	// alters the placeholder text invalidates the entire cache.
	for _, m := range got {
		if m.Role == types.MessageRoleUser && bytes.Contains([]byte(m.Content), []byte(conversation.ForkPlaceholderResult)) {
			return
		}
	}
	t.Fatalf("placeholder %q not present byte-exact in any user message; got: %+v",
		conversation.ForkPlaceholderResult, got)
}
