// Package d7spans emit tests: 5 P0/P1 Span ops × 2 (emit + nil-bridge
// fail-safe) = 10 T points. v6.0.0 6 S 精简 S4 验收要求。
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
//
// Bridge is wired via SetBridge for emit-happy-path tests and reset to
// nil for fail-safe tests. Tests use the observability package's built-
// in no-op tracer (default when IsEnabled() returns false), so emit
// never crashes even when the bridge is genuinely nil.
package d7spans

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

	end := EmitChannelRoute(context.Background(), "sess_x", "commit", "commit", "0.987", "false")
	if end == nil {
		t.Fatal("emit must return a non-nil end func")
	}
	end(nil) // must not panic
}

func TestD7S6A48T02_EmitChannelRoute_NilBridgeFailSafe(t *testing.T) {
	resetBridge()

	end := EmitChannelRoute(context.Background(), "sess_x", "commit", "commit", "0.987", "false")
	if end == nil {
		t.Fatal("emit must return a non-nil end func even when bridge is nil")
	}
	end(nil) // must not panic
}

// ─── D7-S6-A49 memory.persist ───────────────────────────────────────

func TestD7S6A49T03_EmitMemoryPersist_HappyPath(t *testing.T) {
	wireNoopBridge()
	defer resetBridge()

	end := EmitMemoryPersist(context.Background(), "sess_x", "skill", "LearningSOP", 60000, 256)
	if end == nil {
		t.Fatal("emit must return a non-nil end func")
	}
	end(nil)
}

func TestD7S6A49T04_EmitMemoryPersist_NilBridgeFailSafe(t *testing.T) {
	resetBridge()

	end := EmitMemoryPersist(context.Background(), "sess_x", "skill", "LearningSOP", 60000, 256)
	if end == nil {
		t.Fatal("emit must return a non-nil end func even when bridge is nil")
	}
	end(nil)
}

// ─── D7-S4-A47 system.anomaly_detect ─────────────────────────────────

func TestD7S4A47T05_EmitSystemAnomalyDetect_HappyPath(t *testing.T) {
	wireNoopBridge()
	defer resetBridge()

	end := EmitSystemAnomalyDetect(context.Background(), "sess_x", "rate_spike", "high", "3", "sess_x:5:5")
	if end == nil {
		t.Fatal("emit must return a non-nil end func")
	}
	end(nil)
}

func TestD7S4A47T06_EmitSystemAnomalyDetect_NilBridgeFailSafe(t *testing.T) {
	resetBridge()

	end := EmitSystemAnomalyDetect(context.Background(), "sess_x", "rate_spike", "high", "3", "sess_x:5:5")
	if end == nil {
		t.Fatal("emit must return a non-nil end func even when bridge is nil")
	}
	end(nil)
}

// ─── D7-S5-A33 taskgraph.synthesize ─────────────────────────────────

func TestD7S5A33T07_EmitTaskGraphSynthesize_HappyPath(t *testing.T) {
	wireNoopBridge()
	defer resetBridge()

	end := EmitTaskGraphSynthesize(context.Background(), "sess_x", 4, 3, 2, false)
	if end == nil {
		t.Fatal("emit must return a non-nil end func")
	}
	end(nil)
}

func TestD7S5A33T08_EmitTaskGraphSynthesize_NilBridgeFailSafe(t *testing.T) {
	resetBridge()

	end := EmitTaskGraphSynthesize(context.Background(), "sess_x", 4, 3, 2, false)
	if end == nil {
		t.Fatal("emit must return a non-nil end func even when bridge is nil")
	}
	end(nil)
}

// ─── D7-S5-A34 executor.select ─────────────────────────────────────

func TestD7S5A34T09_EmitExecutorSelect_HappyPath(t *testing.T) {
	wireNoopBridge()
	defer resetBridge()

	end := EmitExecutorSelect(context.Background(), "sess_x", 3, "subagent", "1.000", "kind_match")
	if end == nil {
		t.Fatal("emit must return a non-nil end func")
	}
	end(nil)
}

func TestD7S5A34T10_EmitExecutorSelect_NilBridgeFailSafe(t *testing.T) {
	resetBridge()

	end := EmitExecutorSelect(context.Background(), "sess_x", 3, "subagent", "1.000", "kind_match")
	if end == nil {
		t.Fatal("emit must return a non-nil end func even when bridge is nil")
	}
	end(nil)
}