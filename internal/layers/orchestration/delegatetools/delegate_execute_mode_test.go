package delegatetools

import (
	"context"
	"sync"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	"github.com/devrix/devrix/internal/shared/types"
)

// modeCapturingSubQueryRunner captures the Mode field of every DispatchRequest
// that flows through the D4-disabled fallback path. Mirrors the contract used
// by hubspoke_test.go::stubSubQueryRunner.
type modeCapturingSubQueryRunner struct {
	mu        sync.Mutex
	lastMode  contracts.SubAgentMode
	callCount int
	summary   string
}

func (m *modeCapturingSubQueryRunner) RunSubQuery(_ context.Context, _ *types.SessionContext, _, _, _ string, _ int, mode contracts.SubAgentMode) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastMode = mode
	m.callCount++
	return m.summary, nil
}

// T: D7-S2-A06-T14+T15+T17 (B.4.7) — delegate_explore end-to-end mode propagation.
//
// Wires the full path: delegateToolRunner.Execute → hubspoke.Dispatcher
// (D4 disabled) → SubQueryRunner.RunSubQuery. Asserts that the `mode` field
// from the tool input flows byte-exact through to SubQueryRunner.
//
// Mode coverage:
//   - "full"   → explicit legacy mode (AC8 backward compat path)
//   - "brief"  → default (Phase B new default; AC6)
//   - "fork"   → cache-friendly prefix (AC11a)
//   - ""       → empty defers to SubagentConfig.DefaultMode (AC6 fallback)
func TestDelegateTool_Execute_PropagatesModeEnd2End(t *testing.T) {
	cases := []struct {
		name        string
		inputMode   string
		wantForward contracts.SubAgentMode // empty string is allowed (deferred)
	}{
		{name: "full_legacy_backcompat", inputMode: "full", wantForward: contracts.SubAgentModeFull},
		{name: "brief_phase_b_default", inputMode: "brief", wantForward: contracts.SubAgentModeBrief},
		{name: "fork_cache_friendly", inputMode: "fork", wantForward: contracts.SubAgentModeFork},
		{name: "empty_defers_to_default", inputMode: "", wantForward: ""},
		// Unknown mode: schema validator would reject upstream; here we test
		// the parseSubAgentMode path which silently returns "" so the runner
		// can apply its own default. This guards against LLM sending garbage.
		{name: "unknown_mode_silently_dropped", inputMode: "explore", wantForward: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Save and restore globalDeps to keep test isolation.
			orig := globalDeps
			t.Cleanup(func() { globalDeps = orig })

			cap := &modeCapturingSubQueryRunner{summary: "ok"}
			hub := &recordingHubForMode{}
			disp := sessionorchestrator.NewDispatcher(
				config.DelegateConfig{Enabled: false}, // D2 subquery fallback path
				nil,                                   // executor (unused)
				cap,                                   // subQuery runner (captures mode)
				hub,                                   // hub for FlowEvents
				nil,                                   // leaderRes
			)
			globalDeps = Deps{Dispatcher: disp, Tasks: nil}

			runner := &delegateToolRunner{name: "delegate_explore", role: WorkerRoleExplore}
			input := `{"directive":"scan auth files"`
			if tc.inputMode != "" {
				input += `,"mode":"` + tc.inputMode + `"`
			}
			input += `}`

			ctx := tools.WithToolSessionContext(context.Background(), &types.SessionContext{
				SessionID: "sess-mode-test",
				IsWorker:  false,
			})

			res, err := runner.Execute(ctx, "tool-call-1", input)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if res == nil {
				t.Fatalf("Execute returned nil result")
			}
			if res.Error != "" {
				t.Fatalf("Execute returned error: %s", res.Error)
			}

			cap.mu.Lock()
			gotMode := cap.lastMode
			calls := cap.callCount
			cap.mu.Unlock()

			if calls != 1 {
				t.Fatalf("expected 1 SubQueryRunner call, got %d", calls)
			}
			if gotMode != tc.wantForward {
				t.Errorf("forwarded mode = %q, want %q", gotMode, tc.wantForward)
			}
		})
	}
}

// T: D7-S2-A06-T15 (B.4.7 boundary) — mode=full backward compat:
//
// The integration test above proves mode propagation. This unit-level
// companion verifies that the SubQueryRunner.RunSubQuery(..., "full")
// path eventually feeds SubTurnRunner with Mode="full" so that applyMode
// produces PreloadedMessages = parent[:-1] (D2-S15-A08-T07 invariant).
//
// We use the existing modeCapturingSubTurn stub (delegate_mode_propagation_test.go)
// to assert the chain: RunSubQuery → enforce.Run → SubTurnRunner.
func TestDelegateTool_Execute_ModeFull_ReachesSubTurnRunner(t *testing.T) {
	orig := globalDeps
	t.Cleanup(func() { globalDeps = orig })

	capturer := &modeCapturingSubTurn{}
	adapter := &SubQueryRunner{LoopDeps: enforce.SubQueryDeps{
		SubTurn: capturer,
	}}

	parent := &types.SessionContext{SessionID: "sess-mode-full", WorkDir: t.TempDir(), Model: "test"}
	out, err := adapter.RunSubQuery(context.Background(), parent, "explore", "scan", "t1", 1, contracts.SubAgentModeFull)
	if err != nil {
		t.Fatalf("RunSubQuery mode=full: %v", err)
	}
	if out != "ok" {
		t.Fatalf("expected stub summary 'ok', got %q", out)
	}
	gotPtr := capturer.captured.Load()
	if gotPtr == nil {
		t.Fatalf("SubTurn never invoked")
	}
	if *gotPtr != string(contracts.SubAgentModeFull) {
		t.Errorf("SubTurnRequest.Mode = %q, want %q", *gotPtr, contracts.SubAgentModeFull)
	}
}

// recordingHubForMode is a minimal FlowHub stub that swallows emits. We don't
// assert FlowEvents here — that's covered by hubspoke_test.go.
type recordingHubForMode struct{}

func (r *recordingHubForMode) Publish(_ context.Context, _ contracts.FlowEvent) {}
func (r *recordingHubForMode) Snapshot(_ string) contracts.WorkPlanSnapshot {
	return contracts.WorkPlanSnapshot{}
}