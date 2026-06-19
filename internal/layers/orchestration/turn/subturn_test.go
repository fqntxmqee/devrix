package turn

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestSubTurnRunner_RunSubTurn(t *testing.T) {
	llm := &stubLLM{chunks: []llmgateway.Chunk{textChunk("sub answer"), doneChunk()}}
	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: &stubContext{}, Tools: &stubTools{}, Persist: &stubPersist{}, MaxTurns: 4,
	})
	runner := NewSubTurnRunner(orch)

	res, err := runner.RunSubTurn(context.Background(), contracts.SubTurnRequest{
		SessionID:    "sess-sub",
		SystemPrompt: "explore",
		Messages:     []types.Message{{Role: types.MessageRoleUser, Content: "scan"}},
		MaxTurns:     2,
		Scope:        contracts.SubTurnScopeSubQuery,
		ChildContext: &types.SessionContext{SessionID: "sess-sub", AgentID: "explore_1"},
	})
	if err != nil {
		t.Fatalf("RunSubTurn: %v", err)
	}
	if res == nil || res.AssistantText == "" {
		t.Fatalf("expected assistant text, got %+v", res)
	}
}
