package contextengine_test

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/layers/observability/runtime"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// D2-S11-A01-T02: 当 query_loop.enabled=true（默认）时，Process() 必须走
// query_loop 路径，runtime 计数 legacy_harness 增量 = 0。
func TestContextEngine_QueryLoopEnabled_NoLegacyIncrement(t *testing.T) {
	runtime.Reset()

	cfg := config.DefaultContextEngineConfig()
	cfg.QueryLoop.Enabled = true
	cfg.QueryLoop.MaxTurns = 3

	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM:        &mockctx.LLMGateway{Response: "ok"},
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
	if snap.QueryLoop < 1 {
		t.Errorf("query_loop = %d, want >= 1 (Process() must record the path it took)", snap.QueryLoop)
	}
}

// D2-S11-A01-T03: 100 次 Process() 循环后，legacy_harness 计数 = 0。
func TestContextEngine_100xQueryLoop_LegacyBaselineZero(t *testing.T) {
	runtime.Reset()

	cfg := config.DefaultContextEngineConfig()
	cfg.QueryLoop.Enabled = true
	cfg.QueryLoop.MaxTurns = 2

	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM:        &mockctx.LLMGateway{Response: "ok"},
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
	if snap.QueryLoop != int64(iterations) {
		t.Errorf("query_loop = %d, want %d (every Process() should record 1)", snap.QueryLoop, iterations)
	}
}

// D2-S11-A01-T02 副: 显式 query_loop.enabled=false → 路径计数走 legacy。
func TestContextEngine_QueryLoopDisabled_LegacyIncrement(t *testing.T) {
	runtime.Reset()

	cfg := config.DefaultContextEngineConfig()
	cfg.QueryLoop.Enabled = false
	cfg.Harness.Enabled = true

	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM:        &mockctx.LLMGateway{Response: "ok"},
		Tools:      &mockctx.ToolRunner{Output: "tool out"},
		ToolsReg:   mustBuiltinRegistry(t),
		Permission: mockctx.AllowAllPermission{},
		Config:     cfg,
	})

	session := types.NewSession("sess_legacy_2_9_02", "cli", t.TempDir())
	ch := engine.Process(context.Background(), session, "ping")
	for ev := range ch {
		_ = ev
	}

	snap := runtime.Snapshot()
	if snap.LegacyHarness < 1 {
		t.Errorf("legacy_harness = %d, want >= 1 (explicit query_loop.enabled=false must take the legacy path)", snap.LegacyHarness)
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
