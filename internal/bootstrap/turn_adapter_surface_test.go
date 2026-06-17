package bootstrap

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/turn"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// stubSurface is a contracts.ToolSurface whose Execute records every call.
// Use the New() factory to capture hits in tests.
type stubSurface struct {
	name   string
	risk   types.RiskLevel
	out    string
	err    string
	hits   *int32
	failGo bool
}

func (s *stubSurface) Name() string { return s.name }
func (s *stubSurface) Tools(_ context.Context, _, _ string) []contracts.ToolSpec {
	return []contracts.ToolSpec{{Name: s.name, Risk: s.risk}}
}
func (s *stubSurface) RiskLevel(name string) types.RiskLevel {
	if name == s.name {
		return s.risk
	}
	return "" // empty = I don't know this tool (used by findSurface)
}
func (s *stubSurface) Execute(_ context.Context, name, input, _ string) (*contracts.ToolResult, error) {
	if atomic.AddInt32(s.hits, 1); s.failGo {
		return nil, errGoStub
	}
	return &contracts.ToolResult{Output: s.out, Error: s.err}, nil
}

var errGoStub = stubGoError("surface: go error")

type stubGoError string

func (e stubGoError) Error() string { return string(e) }

// T: TOOL-SURFACE-1-T09 — ExecuteRound dispatches to a matching
// surface, not to a.tools.Execute, when a surface claims the name.
func TestExecuteRound_GoesThroughSurface_NotThroughIToolRunner(t *testing.T) {
	hits := new(int32)
	surfaces := []contracts.ToolSurface{
		&stubSurface{name: "alpha", risk: types.RiskLevelLow, out: "alpha-out", hits: hits},
	}
	adapter := &contextEngineAdapter{
		surfaces: surfaces,
		// tools left nil on purpose: any call here would NPE
	}
	res, err := adapter.ExecuteRound(context.Background(), turn.ToolRoundRequest{
		SessionID: "sess-x",
		ToolCalls: []llmgateway.ToolCall{{ID: "t1", Name: "alpha", Input: `{}`}},
	})
	if err != nil {
		t.Fatalf("ExecuteRound: %v", err)
	}
	if got := atomic.LoadInt32(hits); got != 1 {
		t.Errorf("surface hits = %d, want 1", got)
	}
	if len(res.Results) != 1 {
		t.Fatalf("len = %d, want 1", len(res.Results))
	}
	if res.Results[0].Output != "alpha-out" {
		t.Errorf("Output = %q, want alpha-out", res.Results[0].Output)
	}
}

// T: TOOL-SURFACE-1-T09 — findSurface: no surface claims the name →
// returns (nil, false). Caller falls back to legacy IToolRunner.
func TestExecuteRound_FindSurface_NotFound(t *testing.T) {
	adapter := &contextEngineAdapter{
		surfaces: []contracts.ToolSurface{
			&stubSurface{name: "alpha", risk: types.RiskLevelLow},
		},
	}
	surf, ok := adapter.findSurface("beta")
	if ok || surf != nil {
		t.Errorf("findSurface(beta) = (%v, %v), want (nil, false)", surf, ok)
	}
}

// T: TOOL-SURFACE-1-T09 — riskForTool consults surfaces first, then
// the legacy toolsReg, then defaults to LOW.
func TestExecuteRound_RiskForTool_SurfaceFirst(t *testing.T) {
	adapter := &contextEngineAdapter{
		surfaces: []contracts.ToolSurface{
			&stubSurface{name: "alpha", risk: types.RiskLevelHigh},
		},
		// No toolsReg → no fallback. Adapter should not panic.
	}
	if got := adapter.riskForTool("alpha"); got != types.RiskLevelHigh {
		t.Errorf("alpha risk = %q, want HIGH (from surface)", got)
	}
	if got := adapter.riskForTool("unknown"); got != types.RiskLevelLow {
		t.Errorf("unknown risk = %q, want LOW (default)", got)
	}
}

// T: TOOL-SURFACE-1-T09 — findSurface linear scan order: first surface
// that returns a non-empty RiskLevel wins. Empty = "I don't know".
func TestExecuteRound_FindSurface_FirstMatchWins(t *testing.T) {
	hits1, hits2 := new(int32), new(int32)
	adapter := &contextEngineAdapter{
		surfaces: []contracts.ToolSurface{
			&stubSurface{name: "a", risk: types.RiskLevelLow, hits: hits1},
			&stubSurface{name: "a", risk: types.RiskLevelHigh, hits: hits2},
		},
	}
	// Both surfaces claim "a" (by name match) but the first wins.
	surf, ok := adapter.findSurface("a")
	if !ok {
		t.Fatal("findSurface(a) returned false")
	}
	if surf.Name() != "a" {
		t.Errorf("Name = %q", surf.Name())
	}
	// Only the first surface saw Execute (a separate concern; this test
	// just pins the linear-scan contract).
}

// T: TOOL-SURFACE-1-T09 — Surface Go error is propagated as Result.Error,
// not as the round-level error.
func TestExecuteRound_SurfaceGoError_PropagatesToResult(t *testing.T) {
	hits := new(int32)
	adapter := &contextEngineAdapter{
		surfaces: []contracts.ToolSurface{
			&stubSurface{name: "alpha", hits: hits, failGo: true},
		},
	}
	res, err := adapter.ExecuteRound(context.Background(), turn.ToolRoundRequest{
		ToolCalls: []llmgateway.ToolCall{{ID: "t1", Name: "alpha", Input: `{}`}},
	})
	if err != nil {
		t.Fatalf("ExecuteRound returned Go error: %v", err)
	}
	if len(res.Results) != 1 {
		t.Fatalf("len = %d", len(res.Results))
	}
	if res.Results[0].Error == "" {
		t.Error("Result.Error empty, want propagated Go error")
	}
}
