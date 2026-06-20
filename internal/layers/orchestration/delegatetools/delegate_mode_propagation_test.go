package delegatetools

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/layers/orchestration/hubspoke"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: DM-20260620-001-B / B.2.5 (D4-S4-A07-T01 boundary) — end-to-end
// mode propagation: SubQueryRunner.RunSubQuery(..., mode) → enforce.Run
// → SubTurnRunner → SubTurnRequest.Mode. This guards the wiring chain so
// a future refactor that drops Mode (e.g. signature shrink) breaks here.
func TestSubQueryRunner_PropagatesModeToSubTurn(t *testing.T) {
	capturer := &modeCapturingSubTurn{}
	adapter := &SubQueryRunner{LoopDeps: enforce.SubQueryDeps{
		SubTurn: capturer,
	}}
	parent := &types.SessionContext{SessionID: "sess_mode", WorkDir: t.TempDir(), Model: "test"}

	for _, mode := range []contracts.SubAgentMode{
		contracts.SubAgentModeBrief,
		contracts.SubAgentModeFork,
		contracts.SubAgentModeFull,
	} {
		capturer.captured.Store((*string)(nil))
		_, err := adapter.RunSubQuery(context.Background(), parent, "explore", "scan", "t1", 1, mode)
		if err != nil {
			t.Fatalf("RunSubQuery mode=%q: %v", mode, err)
		}
		gotPtr := capturer.captured.Load()
		if gotPtr == nil {
			t.Fatalf("SubTurn never invoked for mode=%q", mode)
		}
		if string(mode) != *gotPtr {
			t.Errorf("mode=%q: SubTurnRequest.Mode = %q, want %q", mode, *gotPtr, mode)
		}
	}

	// Empty mode: should also propagate empty (SubTurnRunner will resolve
	// via Cfg.DefaultMode).
	capturer.captured.Store((*string)(nil))
	_, err := adapter.RunSubQuery(context.Background(), parent, "explore", "scan", "t1", 1, "")
	if err != nil {
		t.Fatalf("RunSubQuery mode=empty: %v", err)
	}
	gotPtr := capturer.captured.Load()
	if gotPtr == nil {
		t.Fatalf("SubTurn never invoked for mode=empty")
	}
	if *gotPtr != "" {
		t.Errorf("mode=empty: SubTurnRequest.Mode = %q, want \"\"", *gotPtr)
	}
}

// modeCapturingSubTurn is a stub SubTurnExecutor that records the Mode
// field of the SubTurnRequest it received. We use atomic.Pointer to avoid
// the race detector when the runner is called from multiple goroutines.
type modeCapturingSubTurn struct {
	captured atomic.Pointer[string]
}

func (m *modeCapturingSubTurn) RunSubTurn(_ context.Context, req contracts.SubTurnRequest) (*contracts.SubTurnResult, error) {
	mode := string(req.Mode)
	m.captured.Store(&mode)
	// Emit a complete event so collectSubTurnResult returns cleanly.
	// The orchestrator path is bypassed (we return the result directly
	// via a fake event channel). Since we're a stub, return an empty
	// result synchronously without going through orchestrator.
	return &contracts.SubTurnResult{
		AssistantText: "ok",
		Messages:      []types.Message{{Role: types.MessageRoleAssistant, Content: "ok"}},
	}, nil
}

// Stub reporter to keep the SubQueryRunner happy when FlowReporter is nil.
type noopFlowReporter struct{}

func (n noopFlowReporter) OnStarted(_ context.Context, _ contracts.SubQueryFlowParams, _ string) {}
func (n noopFlowReporter) OnCompleted(_ context.Context, _ contracts.SubQueryFlowParams, _ string) {}
func (n noopFlowReporter) OnFailed(_ context.Context, _ contracts.SubQueryFlowParams, _ string)    {}
func (n noopFlowReporter) WrapEmit(_ context.Context, _ contracts.SubQueryFlowParams, base contracts.EngineEmitFunc) contracts.EngineEmitFunc {
	return base
}

// silence unused warning
var _ = hubspoke.NewFlowReporter
