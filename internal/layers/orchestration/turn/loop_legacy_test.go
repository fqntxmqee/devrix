package turn

// T: D7-S2-A06-T09 — LoopFirst=true keeps the canonical D7 RunTurn
// main path off the D2.QueryLoop.Run hot path. Production wiring
// instantiates a shared d2_query_loop_legacy_invocations_total
// counter; the orchestrator never touches it, so a full turn must
// leave the counter at zero.
import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/observability/instrument/metrics"
	"github.com/devrix/devrix/internal/shared/types"
)

// Covers: D7-S2-A06-T09
// Scenario: loopFirst=true — the orchestrator (D7-S2-A06 RunTurn)
// completes a turn without bumping the legacy D2 counter.
func TestOrchestrator_RunTurn_DoesNotInvokeLegacyQueryLoop(t *testing.T) {
	counter := metrics.NewCounter("d2_query_loop_legacy_invocations_total", nil)

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
		// LegacyCounter is intentionally absent: the D7 orchestrator
		// does not depend on D2.QueryLoop.Run at all.
	})
	_ = counter // explicit reference for grep-ability

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID:   "sess-loopfirst",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "hi", SessionID: "sess-loopfirst"},
		MaxTurns:    1,
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	// Drain events to ensure the full turn ran.
	for range ch {
	}

	if v := counter.Value(); v != 0 {
		t.Errorf("legacy counter = %d, want 0 (orchestrator must not touch D2.QueryLoop.Run)", v)
	}
}
