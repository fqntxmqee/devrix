package sessionorchestrator

import (
	"context"
	"github.com/devrix/devrix/internal/layers/orchestration/decisionplanning"
	"sync/atomic"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/turn"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
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

func (f *fakeWaveForTools) Start(_ context.Context, _ string, _ *wavescheduler.TaskGraph) error {
	f.starts.Add(1)
	return nil
}

func (f *fakeWaveForTools) WaitForCompletion(_ context.Context, _ string) ([]wavescheduler.Artifact, error) {
	return []wavescheduler.Artifact{{TaskID: "t1", Summary: "done", ExitCode: 0}}, nil
}

// T: D7-S2-L5-02 — delegate_wave tool triggers OrchestratePath without ingress routing.
func TestTurnToolExecutor_DelegateWave(t *testing.T) {
	base := &stubToolBase{}
	fake := &fakeWaveForTools{}
	op := NewOrchestratePath(decisionplanning.NewTaskDecomposer(), fake, nil)
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

// T: D7-S2-A06-T03 (DM-20260617-004 devrix-d7-tool-ctx-inject)
// orchestrationToolSchemas (LoopFirst injected tool list) must include the
// free_fork tool so users saying "用 free_fork 启 N 个 worker" reach a
// real registered tool. Without this, the LLM hallucinates old tool names
// (delegate_wave / task_output / task_list_background) under loop_first.
func TestOrchestrationToolSchemas_ExposesFreeFork(t *testing.T) {
	schemas := orchestrationToolSchemas()
	names := make(map[string]bool, len(schemas))
	for _, s := range schemas {
		names[s.Name] = true
	}
	for _, want := range []string{"delegate_wave", "enter_plan_mode", "free_fork"} {
		if !names[want] {
			t.Errorf("orchestrationToolSchemas missing %q; got names=%v", want, names)
		}
	}
}

// T: D7-S2-A06-T03 — free_fork schema declares parent_session + requests with
// required name/prompt, matching the D2 freeforkRunner input contract.
func TestOrchestrationToolSchemas_FreeFork_Parameters(t *testing.T) {
	schemas := orchestrationToolSchemas()
	var ff *turn.ToolSchema
	for i := range schemas {
		if schemas[i].Name == "free_fork" {
			ff = &schemas[i]
			break
		}
	}
	if ff == nil {
		t.Fatal("free_fork schema not found in orchestrationToolSchemas")
	}
	if ff.Description == "" {
		t.Error("free_fork schema: description is empty")
	}
	props, ok := ff.Parameters["properties"].(map[string]any)
	if !ok {
		t.Fatalf("free_fork schema: properties missing or wrong type: %+v", ff.Parameters)
	}
	for _, want := range []string{"parent_session", "requests"} {
		if _, ok := props[want]; !ok {
			t.Errorf("free_fork schema: missing property %q; got=%v", want, props)
		}
	}
	required, ok := ff.Parameters["required"].([]any)
	if !ok {
		t.Fatalf("free_fork schema: required missing or wrong type")
	}
	wantReq := map[string]bool{"parent_session": false, "requests": false}
	for _, r := range required {
		if s, ok := r.(string); ok {
			if _, present := wantReq[s]; present {
				wantReq[s] = true
			}
		}
	}
	for k, seen := range wantReq {
		if !seen {
			t.Errorf("free_fork schema: required[%q] missing", k)
		}
	}
}
