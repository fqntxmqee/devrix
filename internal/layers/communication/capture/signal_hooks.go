package capture

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/communication/capture/signal"
	"github.com/devrix/devrix/internal/layers/communication/kernel"
	"github.com/devrix/devrix/internal/layers/observability/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/tracer"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

func (g *CommunicationGateway) beginInboundTurn(sessionID, inboundTurnID string) {
	if g.turnTracker != nil {
		g.turnTracker.BeginTurn(sessionID, inboundTurnID, g.clock.Now())
	}
}

func (g *CommunicationGateway) endInboundTurn(sessionID string) {
	if g.turnTracker != nil {
		g.turnTracker.EndTurn(sessionID)
	}
}

func (g *CommunicationGateway) recordConclusionFeedback(ctx context.Context, session *types.Session, reason string) {
	if g.obsBridge == nil || g.obsBridge.Tracer() == nil {
		return
	}
	_, span := g.obsBridge.Tracer().Start(ctx, telemetry.OpUserFeedbackConclusionRejected,
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpUserFeedbackConclusionRejected,
			tracer.Attribute{Key: "session.id", Value: session.SessionID},
			tracer.Attribute{Key: "feedback.reason", Value: reason},
		)...),
	)
	if span != nil {
		span.End()
	}
}

func (g *CommunicationGateway) emitOutboundSignalSpans(
	ctx context.Context,
	session *types.Session,
	sig contracts.IMOutboundSignal,
	chain signal.ChainReport,
	event *EngineEvent,
) {
	if g.obsBridge == nil || g.obsBridge.Tracer() == nil {
		return
	}
	op := signalSpanOp(sig.Kind)
	attrs := telemetry.SpanAttrs(op,
		tracer.Attribute{Key: "session.id", Value: session.SessionID},
		tracer.Attribute{Key: "signal.kind", Value: string(sig.Kind)},
		tracer.Attribute{Key: "signal.sequence", Value: fmt.Sprintf("%d", sig.Sequence)},
		tracer.Attribute{Key: "source.event_id", Value: sig.SourceEventID},
		tracer.Attribute{Key: "elapsed_ms", Value: fmt.Sprintf("%d", sig.ElapsedMs)},
		tracer.Attribute{Key: "inbound.turn_id", Value: sig.InboundTurnID},
		tracer.Attribute{Key: "signal.is_terminal", Value: fmt.Sprintf("%t", sig.IsTerminal)},
	)
	_, span := g.obsBridge.Tracer().Start(ctx, op,
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(attrs...),
	)
	if span != nil {
		span.End()
	}

	intact := "true"
	breakAt := ""
	if !chain.Intact {
		intact = "false"
		breakAt = string(chain.BreakAt)
	}
	_, chainSpan := g.obsBridge.Tracer().Start(ctx, telemetry.OpD1SignalChainIntegrity,
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpD1SignalChainIntegrity,
			tracer.Attribute{Key: "session.id", Value: session.SessionID},
			tracer.Attribute{Key: "chain.intact", Value: intact},
			tracer.Attribute{Key: "chain.break_at_kind", Value: breakAt},
			tracer.Attribute{Key: "chain.saw_thinking", Value: fmt.Sprintf("%t", chain.SawThinking)},
			tracer.Attribute{Key: "chain.saw_task", Value: fmt.Sprintf("%t", chain.SawTask)},
			tracer.Attribute{Key: "chain.saw_conclusion", Value: fmt.Sprintf("%t", chain.SawConclusion)},
		)...),
	)
	if chainSpan != nil {
		chainSpan.End()
	}

	if sig.Kind == contracts.SignalTask && event != nil &&
		(event.Type == "tool_call" || event.Type == "tool_result" || event.Type == "worker_progress") {
		toolName := event.ToolName
		if toolName == "" && event.Metadata != nil {
			toolName = event.Metadata["tool_name"]
		}
		_, wpSpan := g.obsBridge.Tracer().Start(ctx, telemetry.OpD1SignalTaskWorkProof,
			tracer.WithSpanKind(tracer.SpanKindInternal),
			tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpD1SignalTaskWorkProof,
				tracer.Attribute{Key: "session.id", Value: session.SessionID},
				tracer.Attribute{Key: "tool.name", Value: toolName},
				tracer.Attribute{Key: "event.type", Value: event.Type},
			)...),
		)
		if wpSpan != nil {
			wpSpan.End()
		}
	}
}

func signalSpanOp(kind contracts.SignalKind) string {
	switch kind {
	case contracts.SignalThinking:
		return telemetry.OpD1SignalThinking
	case contracts.SignalTask:
		return telemetry.OpD1SignalTask
	case contracts.SignalConclusion:
		return telemetry.OpD1SignalConclusion
	default:
		return telemetry.OpD1SignalConclusion
	}
}

func (g *CommunicationGateway) startCapturePersistSpan(ctx context.Context, sessionID string) {
	if g.obsBridge == nil || g.obsBridge.Tracer() == nil {
		return
	}
	_, span := g.obsBridge.Tracer().Start(ctx, telemetry.OpD1CapturePersist,
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpD1CapturePersist,
			tracer.Attribute{Key: "session.id", Value: sessionID},
		)...),
	)
	if span != nil {
		span.End()
	}
}

func (g *CommunicationGateway) startDispatchRouteSpan(ctx context.Context, sessionID, target string) {
	if g.obsBridge == nil || g.obsBridge.Tracer() == nil {
		return
	}
	_, span := g.obsBridge.Tracer().Start(ctx, telemetry.OpD1DispatchRoute,
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpD1DispatchRoute,
			tracer.Attribute{Key: "session.id", Value: sessionID},
			tracer.Attribute{Key: "dispatch.target", Value: target},
		)...),
	)
	if span != nil {
		span.End()
	}
}

func enrichOutboundMetadata(meta map[string]string, sig contracts.IMOutboundSignal) map[string]string {
	return kernel.EnrichMetadata(meta, sig)
}
