package coordinator

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/turn"
	"github.com/devrix/devrix/internal/layers/orchestration/wave"
	"github.com/devrix/devrix/internal/shared/contracts"
)

type stubToolBase struct {
	calls atomic.Int32
}

func (s *stubToolBase) ExecuteRound(_ context.Context, req turn.ToolRoundRequest) (turn.ToolRoundResult, error) {
	s.calls.Add(1)
	results := make([]turn.ToolResult, len(req.ToolCalls))
	for i, tc := range req.ToolCalls {
		results[i] = turn.ToolResult{ToolCallID: tc.ID, Output: "ok"}
	}
	return turn.ToolRoundResult{Results: results}, nil
}

type fakeWaveForTools struct {
	starts atomic.Int32
}

func (f *fakeWaveForTools) Start(_ context.Context, _ string, _ *wave.TaskGraph) error {
	f.starts.Add(1)
	return nil
}

func (f *fakeWaveForTools) WaitForCompletion(_ context.Context, _ string) ([]wave.Artifact, error) {
	return []wave.Artifact{{TaskID: "t1", Summary: "done", ExitCode: 0}}, nil
}

// T: D7-S2-L5-02 — delegate_wave tool triggers OrchestratePath without ingress routing.
func TestTurnToolExecutor_DelegateWave(t *testing.T) {
	base := &stubToolBase{}
	fake := &fakeWaveForTools{}
	op := NewOrchestratePath(NewTaskDecomposer(), fake, nil)
	exec := NewTurnToolExecutor(base, op, nil, true)

	var streamed int
	ctx := turn.WithToolEventStream(context.Background(), func(ev *contracts.EngineEvent) {
		if ev != nil {
			streamed++
		}
	})

	_, err := exec.ExecuteRound(ctx, turn.ToolRoundRequest{
		SessionID: "sess-1",
		ToolCalls: []llmgateway.ToolCall{{
			ID:    "tc1",
			Name:  toolDelegateWave,
			Input: `{"goal":"design auth refactor && add tests"}`,
		}},
	})
	if err != nil {
		t.Fatalf("ExecuteRound: %v", err)
	}
	if fake.starts.Load() != 1 {
		t.Fatalf("Wave Start calls = %d, want 1", fake.starts.Load())
	}
	if streamed == 0 {
		t.Fatal("expected streamed engine events from delegate_wave")
	}
	if exec.DelegateWaveCount.Load() != 1 {
		t.Fatalf("delegate_wave count = %d", exec.DelegateWaveCount.Load())
	}
}
