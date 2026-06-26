// Package hardening (emitter.go) provides v6.0.0 5 new P0/P1 Span emit helpers for D7 6 S 精简.
// Each function is a thin wrapper around observability.Bridge.Tracer().Start
// that no-ops when the bridge is nil or disabled.
//
// Why in hardening package: avoids each consumer (execute / learn / decisionplanning /
// wavescheduler / verify) re-implementing the same start/end dance. Centralised
// attribute naming keeps the Jaeger UI consistent.
//
// Why no constructor changes: a package-level setter is set once at bootstrap,
// then any caller in any orchestration sub-package can emit a Span without
// threading an obsBridge through every constructor.
package hardening

import (
	"context"
	"sync"

	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
)

// bridge is the package-level observability bridge. Set via SetBridge at
// bootstrap. Nil means tracing is disabled and all emitters no-op.
var (
	bridgeMu sync.RWMutex
	bridge   *observability.Bridge
)

// SetBridge wires the observability bridge for v6.0.0 5 new Span ops.
// Idempotent; safe to call multiple times. Called from
// bootstrap/wire_coordinator.go after WithObservability is applied.
func SetBridge(b *observability.Bridge) {
	bridgeMu.Lock()
	defer bridgeMu.Unlock()
	bridge = b
}

// start begins a Span. Returns (ctx, span) where span may be nil if bridge
// is nil or disabled. Caller MUST defer endSpan to ensure End() fires.
func start(ctx context.Context, operation string, attrs ...tracer.Attribute) (context.Context, tracer.Span) {
	bridgeMu.RLock()
	b := bridge
	bridgeMu.RUnlock()
	if b == nil || !b.IsEnabled() {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(operation, attrs...)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return b.Tracer().Start(ctx, operation, opts...)
}

// endSpan fires End() if span is non-nil. Mirrors sessionorchestrator.tracing.endSpan.
func endSpan(span tracer.Span) {
	if span != nil {
		span.End()
	}
}

// endSpanWithError fires RecordError + End if span is non-nil and err is non-nil.
func endSpanWithError(span tracer.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
	}
	span.End()
}

// --- S6 MUPS Pipeline ---

// EmitMUPSPipeline is the v6.0.0 root span for the 5-node MUPS pipeline
// (DM-20260626-009 hotfix: previously only OrchestratePath emitted this; the
// per-WorkItem ItemPipelineRunner path was missing it, so Jaeger showed
// the 5-node span tree on the legacy route only). Returns the enriched
// context so callers can chain the 5 sub-spans as children.
func EmitMUPSPipeline(ctx context.Context, sessionID, workItemID, pipelineIntent string) (context.Context, func(error)) {
	attrs := []tracer.Attribute{
		{Key: "session_id", Value: sessionID},
		{Key: "pipeline.work_item_id", Value: workItemID},
		{Key: "pipeline.intent", Value: pipelineIntent},
		{Key: "pipeline.nodes", Value: "observe,plan,wave,execute,verify,learn"},
	}
	c, span := start(ctx, telemetry.OpD7_S6_MUPS_Pipeline, attrs...)
	return c, func(err error) { endSpanWithError(span, err) }
}

// EmitChannelRoute wraps ChannelRouter.Route. v6.0.0 S6-A48 P0.
func EmitChannelRoute(ctx context.Context, sessionID, planKind, channelKind, score, fallback string) (context.Context, func(error)) {
	attrs := []tracer.Attribute{
		{Key: "session_id", Value: sessionID},
		{Key: "channel.kind", Value: channelKind},
		{Key: "plan.kind", Value: planKind},
		{Key: "score", Value: score},
		{Key: "fallback", Value: fallback},
	}
	c, span := start(ctx, telemetry.OpD7_S6_Channel_Route, attrs...)
	return c, func(err error) { endSpanWithError(span, err) }
}

// EmitMemoryPersist wraps Memory.Persist. v6.0.0 S6-A49 P0.
func EmitMemoryPersist(ctx context.Context, sessionID, channel, assetKind string, ttlMs int, payloadSize int) (context.Context, func(error)) {
	attrs := []tracer.Attribute{
		{Key: "session_id", Value: sessionID},
		{Key: "channel", Value: channel},
		{Key: "asset.kind", Value: assetKind},
		{Key: "ttl_ms", Value: intToString(ttlMs)},
		{Key: "payload_size", Value: intToString(payloadSize)},
	}
	c, span := start(ctx, telemetry.OpD7_S6_Memory_Persist, attrs...)
	return c, func(err error) { endSpanWithError(span, err) }
}

// --- S5 DecisionPlanning + Observe ---

// EmitTaskGraphSynthesize wraps DecisionPlanning.SynthesizeTaskGraph. v6.0.0 S5-A33 P1.
func EmitTaskGraphSynthesize(ctx context.Context, sessionID string, nodeCount, edgeCount, dagDepth int, cycleDetected bool) (context.Context, func(error)) {
	attrs := []tracer.Attribute{
		{Key: "session_id", Value: sessionID},
		{Key: "taskgraph.node_count", Value: intToString(nodeCount)},
		{Key: "taskgraph.edge_count", Value: intToString(edgeCount)},
		{Key: "taskgraph.dag_depth", Value: intToString(dagDepth)},
		{Key: "taskgraph.cycle_detected", Value: boolToString(cycleDetected)},
	}
	c, span := start(ctx, telemetry.OpD7_S5_TaskGraph_Synthesize, attrs...)
	return c, func(err error) { endSpanWithError(span, err) }
}

// --- S4 ExecutionFlow + Verify ---

// EmitSystemAnomalyDetect wraps verify.DetectSystemAnomaly. v6.0.0 S4-A47 P0.
func EmitSystemAnomalyDetect(ctx context.Context, sessionID, anomalyKind, severity, threshold, evidenceID string) (context.Context, func(error)) {
	attrs := []tracer.Attribute{
		{Key: "session_id", Value: sessionID},
		{Key: "anomaly.kind", Value: anomalyKind},
		{Key: "severity", Value: severity},
		{Key: "threshold", Value: threshold},
		{Key: "evidence_id", Value: evidenceID},
	}
	c, span := start(ctx, telemetry.OpD7_S4_System_Anomaly_Detect, attrs...)
	return c, func(err error) { endSpanWithError(span, err) }
}

// --- S3 WaveScheduler ---

// EmitExecutorSelect wraps WaveScheduler.ExecutorSelector.Select. v6.0.0 S5-A34 P1.
func EmitExecutorSelect(ctx context.Context, sessionID string, candidatesCount int, selectedKind, score, policy string) (context.Context, func(error)) {
	attrs := []tracer.Attribute{
		{Key: "session_id", Value: sessionID},
		{Key: "candidates_count", Value: intToString(candidatesCount)},
		{Key: "selected_kind", Value: selectedKind},
		{Key: "score", Value: score},
		{Key: "policy", Value: policy},
	}
	c, span := start(ctx, telemetry.OpD7_S3_Executor_Select, attrs...)
	return c, func(err error) { endSpanWithError(span, err) }
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
