package contextengine_test

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/layers/observability/configure/runtime"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// D2-S11-A01-T02: Process() delegates to D7 PreparedTurnRunner; legacy_harness
// counter must stay at zero.
func TestContextEngine_QueryLoopEnabled_NoLegacyIncrement(t *testing.T) {
	runtime.Reset()

	cfg := config.DefaultContextEngineConfig()
	cfg.TurnRuntime.MaxTurns = 3

	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		PreparedTurnRunner: &mockctx.StaticPreparedTurnRunner{Response: "ok"},
		Summarizer:     &mockctx.StaticSummarizer{},
		Tools:      &mockctx.ToolRunner{Output: "tool out"},
		ToolsReg:   mustBuiltinRegistry(t),
		Permission: mockctx.AllowAllPermission{},
		Config:     cfg,
	})

	session := types.NewSession("sess_l5_2_9_02", "cli", t.TempDir())
	ch := engine.Process(context.Background(), session, "ping")
	for ev := range ch {
		_ = ev
	}

	snap := runtime.Snapshot()
	if snap.LegacyHarness != 0 {
		t.Errorf("legacy_harness = %d, want 0 (QueryLoop must not touch legacy path)", snap.LegacyHarness)
	}
	if snap.D7Turn < 1 {
		t.Errorf("d7_turn = %d, want >= 1 (Process() must record the path it took)", snap.D7Turn)
	}
}

// D2-S11-A01-T03: 100 次 Process() 循环后，legacy_harness 计数 = 0。
func TestContextEngine_100xQueryLoop_LegacyBaselineZero(t *testing.T) {
	runtime.Reset()

	cfg := config.DefaultContextEngineConfig()
	cfg.TurnRuntime.MaxTurns = 2

	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		PreparedTurnRunner: &mockctx.StaticPreparedTurnRunner{Response: "ok"},
		Summarizer:     &mockctx.StaticSummarizer{},
		Tools:      &mockctx.ToolRunner{Output: "tool out"},
		ToolsReg:   mustBuiltinRegistry(t),
		Permission: mockctx.AllowAllPermission{},
		Config:     cfg,
	})

	const iterations = 100
	for i := 0; i < iterations; i++ {
		session := types.NewSession("sess_baseline_"+itoa(i), "cli", t.TempDir())
		ch := engine.Process(context.Background(), session, "iter")
		for ev := range ch {
			_ = ev
		}
	}

	snap := runtime.Snapshot()
	if snap.LegacyHarness != 0 {
		t.Errorf("legacy_harness = %d, want 0 after %d iterations", snap.LegacyHarness, iterations)
	}
	if snap.D7Turn != int64(iterations) {
		t.Errorf("d7_turn = %d, want %d (every Process() should record 1)", snap.D7Turn, iterations)
	}
}

// itoa is a tiny no-allocation integer formatter used to label sessions
// in the 100-iteration baseline test. Avoids pulling in fmt just for one
// call site.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
