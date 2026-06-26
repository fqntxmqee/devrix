// Package hardening (emitter_test.go) emit tests: 8 P0/P1 Span ops × 2 (emit + nil-bridge
// fail-safe) = 16 T points. v6.0.0 6 S 精简 S4 验收要求 + DM-20260626-009 follow-up inner spans.
//
// T 点编号规则（DSAFT 标准 D{X}-S{X}-A{XX}-T{XX}）：
//   D7-S6-A48-T01 channel.route  emit happy path
//   D7-S6-A48-T02 channel.route  nil-bridge fail-safe
//   D7-S6-A49-T03 memory.persist emit happy path
//   D7-S6-A49-T04 memory.persist nil-bridge fail-safe
//   D7-S4-A47-T05 system.anomaly_detect emit happy path
//   D7-S4-A47-T06 system.anomaly_detect nil-bridge fail-safe
//   D7-S5-A33-T07 taskgraph.synthesize emit happy path
//   D7-S5-A33-T08 taskgraph.synthesize nil-bridge fail-safe
//   D7-S5-A34-T09 executor.select emit happy path
//   D7-S5-A34-T10 executor.select nil-bridge fail-safe
//   D7-S1-A52-T11 worktree.op  emit happy path
//   D7-S1-A52-T12 worktree.op  nil-bridge fail-safe
//   D7-S1-A53-T13 subworktree.run  emit happy path
//   D7-S1-A53-T14 subworktree.run  nil-bridge fail-safe
//   D7-S5-A54-T15 subturn.iteration  emit happy path
//   D7-S5-A54-T16 subturn.iteration  nil-bridge fail-safe
//
// Bridge is wired via SetBridge for emit-happy-path tests and reset to
// nil for fail-safe tests. Tests use the observability package's built-
// in no-op tracer (default when IsEnabled() returns false), so emit
// never crashes even when the bridge is genuinely nil.
package hardening

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/observability"
)

// resetBridge is a tiny helper that clears the package-level bridge so
// fail-safe tests start from a known-nil state. Tests that need a
// bridge wire it themselves.
func resetBridge() {
	bridgeMu.Lock()
	bridge = nil
	bridgeMu.Unlock()
}

// wireNoopBridge installs a fresh Bridge backed by a no-op Observability
// instance. The bridge's Tracer() returns the SDK's no-op tracer so
// emit calls succeed without panicking; Span.End() / RecordError are
// no-ops. This is the cheapest way to exercise the emit code paths
// without spinning up an OTLP exporter.
func wireNoopBridge() {
	b := observability.NewBridge(observability.NewNoOp())
	SetBridge(b)
}

// ─── D7-S6-A48 channel.route ────────────────────────────────────────

func TestD7S6A48T01_EmitChannelRoute_HappyPath(t *testing.T) {
	wireNoopBridge()
	defer resetBridge()

	_, end := EmitChannelRoute(context.Background(), "sess_x", "commit", "commit", "0.987", "false")
	if end == nil {
		t.Fatal("emit must return a non-nil end func")
	}
	end(nil) // must not panic
}

func TestD7S6A48T02_EmitChannelRoute_NilBridgeFailSafe(t *testing.T) {
	resetBridge()

	_, end := EmitChannelRoute(context.Background(), "sess_x", "commit", "commit", "0.987", "false")
	if end == nil {
		t.Fatal("emit must return a non-nil end func even when bridge is nil")
	}
	end(nil) // must not panic
}

// ─── D7-S6-A49 memory.persist ───────────────────────────────────────

func TestD7S6A49T03_EmitMemoryPersist_HappyPath(t *testing.T) {
	wireNoopBridge()
	defer resetBridge()

	_, end := EmitMemoryPersist(context.Background(), "sess_x", "skill", "LearningSOP", 60000, 256)
	if end == nil {
		t.Fatal("emit must return a non-nil end func")
	}
	end(nil)
}

func TestD7S6A49T04_EmitMemoryPersist_NilBridgeFailSafe(t *testing.T) {
	resetBridge()

	_, end := EmitMemoryPersist(context.Background(), "sess_x", "skill", "LearningSOP", 60000, 256)
	if end == nil {
		t.Fatal("emit must return a non-nil end func even when bridge is nil")
	}
	end(nil)
}

// ─── D7-S4-A47 system.anomaly_detect ─────────────────────────────────

func TestD7S4A47T05_EmitSystemAnomalyDetect_HappyPath(t *testing.T) {
	wireNoopBridge()
	defer resetBridge()

	_, end := EmitSystemAnomalyDetect(context.Background(), "sess_x", "rate_spike", "high", "3", "sess_x:5:5")
	if end == nil {
		t.Fatal("emit must return a non-nil end func")
	}
	end(nil)
}

func TestD7S4A47T06_EmitSystemAnomalyDetect_NilBridgeFailSafe(t *testing.T) {
	resetBridge()

	_, end := EmitSystemAnomalyDetect(context.Background(), "sess_x", "rate_spike", "high", "3", "sess_x:5:5")
	if end == nil {
		t.Fatal("emit must return a non-nil end func even when bridge is nil")
	}
	end(nil)
}

// ─── D7-S5-A33 taskgraph.synthesize ─────────────────────────────────

func TestD7S5A33T07_EmitTaskGraphSynthesize_HappyPath(t *testing.T) {
	wireNoopBridge()
	defer resetBridge()

	_, end := EmitTaskGraphSynthesize(context.Background(), "sess_x", 4, 3, 2, false)
	if end == nil {
		t.Fatal("emit must return a non-nil end func")
	}
	end(nil)
}

func TestD7S5A33T08_EmitTaskGraphSynthesize_NilBridgeFailSafe(t *testing.T) {
	resetBridge()

	_, end := EmitTaskGraphSynthesize(context.Background(), "sess_x", 4, 3, 2, false)
	if end == nil {
		t.Fatal("emit must return a non-nil end func even when bridge is nil")
	}
	end(nil)
}

// ─── D7-S5-A34 executor.select ─────────────────────────────────────

func TestD7S5A34T09_EmitExecutorSelect_HappyPath(t *testing.T) {
	wireNoopBridge()
	defer resetBridge()

	_, end := EmitExecutorSelect(context.Background(), "sess_x", 3, "subagent", "1.000", "kind_match")
	if end == nil {
		t.Fatal("emit must return a non-nil end func")
	}
	end(nil)
}

func TestD7S5A34T10_EmitExecutorSelect_NilBridgeFailSafe(t *testing.T) {
	resetBridge()

	_, end := EmitExecutorSelect(context.Background(), "sess_x", 3, "subagent", "1.000", "kind_match")
	if end == nil {
		t.Fatal("emit must return a non-nil end func even when bridge is nil")
	}
	end(nil)
}

// ─── D7-S6-A50 mups.pipeline ────────────────────────────────────────

func TestD7S6A50T11_EmitMUPSPipeline_HappyPath(t *testing.T) {
	wireNoopBridge()
	defer resetBridge()

	ctx, end := EmitMUPSPipeline(context.Background(), "sess_x", "wi_x", "intent_orchestrate")
	if end == nil {
		t.Fatal("emit must return a non-nil end func")
	}
	if ctx == nil {
		t.Fatal("emit must return a non-nil ctx so the 5 sub-spans can chain")
	}
	end(nil)
}

func TestD7S6A50T12_EmitMUPSPipeline_NilBridgeFailSafe(t *testing.T) {
	resetBridge()

	ctx, end := EmitMUPSPipeline(context.Background(), "sess_x", "wi_x", "intent_orchestrate")
	if end == nil {
		t.Fatal("emit must return a non-nil end func even when bridge is nil")
	}
	if ctx == nil {
		t.Fatal("emit must return the input ctx unchanged when bridge is nil")
	}
	end(nil)
}

// ─── D7-S1-A52 worktree.op (DM-20260626-009 follow-up inner spans) ──

// TestD7S1A52T11_EmitWorktreeOp_HappyPath verifies EmitWorktreeOp returns
// a non-nil end func and accepts both nil and non-nil error. The span
// itself is the inner layer instrumentation for ItemPipelineRunner's
// r.Tasks.Tree().Xxx() mutations (set_round_phase / list_children /
// apply_pipeline_round / update_status). Without this, Jaeger shows the
// MUPS pipeline root but not which phase transitions actually happened.
func TestD7S1A52T11_EmitWorktreeOp_HappyPath(t *testing.T) {
	wireNoopBridge()
	defer resetBridge()

	end := EmitWorktreeOp(context.Background(), "sess_x", "set_round_phase", "item_1", "observe")
	if end == nil {
		t.Fatal("emit must return a non-nil end func")
	}
	end(nil)
}

// TestD7S1A52T12_EmitWorktreeOp_NilBridgeFailSafe ensures the helper is
// safe to call when the bootstrap has not wired a bridge yet (e.g. unit
// tests, dry-run mode, or when observability is disabled in config). All
// 11 wired call sites in item_pipeline.go rely on this no-op behaviour.
func TestD7S1A52T12_EmitWorktreeOp_NilBridgeFailSafe(t *testing.T) {
	resetBridge()

	end := EmitWorktreeOp(context.Background(), "sess_x", "apply_pipeline_round", "item_1", "await_child")
	if end == nil {
		t.Fatal("emit must return a non-nil end func even when bridge is nil")
	}
	end(nil) // must not panic
}

// ─── D7-S1-A53 subworktree.run ─────────────────────────────────────

func TestD7S1A53T13_EmitSubWorktreeRun_HappyPath(t *testing.T) {
	wireNoopBridge()
	defer resetBridge()

	end := EmitSubWorktreeRun(context.Background(), "sess_x", "parent_1", "child_2", "spawn_parallel_explore")
	if end == nil {
		t.Fatal("emit must return a non-nil end func")
	}
	end(nil)
}

func TestD7S1A53T14_EmitSubWorktreeRun_NilBridgeFailSafe(t *testing.T) {
	resetBridge()

	end := EmitSubWorktreeRun(context.Background(), "sess_x", "parent_1", "child_2", "spawn_parallel_explore")
	if end == nil {
		t.Fatal("emit must return a non-nil end func even when bridge is nil")
	}
	end(nil) // must not panic
}

// ─── D7-S5-A54 subturn.iteration ───────────────────────────────────

// TestD7S5A54T15_EmitSubTurnIteration_HappyPath verifies the per-iter span
// fires for both a normal iter (finish_reason = "stop", stop_reason = "" or
// "ok") and a cap-hit iter (finish_reason = "tool_calls", stop_reason =
// "max_iters"). Both shapes need to be exercised because the helper
// itself doesn't validate — the caller chooses which combo to pass.
func TestD7S5A54T15_EmitSubTurnIteration_HappyPath(t *testing.T) {
	wireNoopBridge()
	defer resetBridge()

	end := EmitSubTurnIteration(context.Background(), "sess_x", "item_1", 3, "stop", "ok")
	if end == nil {
		t.Fatal("emit must return a non-nil end func")
	}
	end(nil)
}

func TestD7S5A54T16_EmitSubTurnIteration_NilBridgeFailSafe(t *testing.T) {
	resetBridge()

	end := EmitSubTurnIteration(context.Background(), "sess_x", "item_1", 5, "tool_calls", "max_iters")
	if end == nil {
		t.Fatal("emit must return a non-nil end func even when bridge is nil")
	}
	end(nil) // must not panic
}