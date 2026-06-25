package sessionorchestrator

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/shared/types"
)

// Covers: D7-S2-A06-T09
// Scenario: RunTurn completes via D7 orchestrator without D2 loop involvement.
func TestOrchestrator_RunTurn_MainPathOnly(t *testing.T) {
	llm := &stubLLM{chunks: []llmgateway.Chunk{
		textChunk("hello"),
		doneChunk(),
	}}
	ctxPrep := &stubContext{prepared: PreparedContext{SystemPrompt: "be brief"}}

	orch := NewOrchestrator(OrchestratorDeps{
		LLM:      llm,
		Context:  ctxPrep,
		Tools:    &stubTools{},
		Persist:  &stubPersist{},
		MaxTurns: 1,
	})

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID:   "sess-loopfirst",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "hi", SessionID: "sess-loopfirst"},
		MaxTurns:    1,
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	for range ch {
	}
}
