package capture

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/communication/capture/signal"
	"github.com/devrix/devrix/internal/layers/communication/kernel"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
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
	_, span := g.obsBridge.Tracer().Start(ctx, telemetry.OpD1_S16_UserFeedback_ConclusionRejected,
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpD1_S16_UserFeedback_ConclusionRejected,
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
	_, chainSpan := g.obsBridge.Tracer().Start(ctx, telemetry.OpD1_S14_Signal_ChainIntegrity,
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpD1_S14_Signal_ChainIntegrity,
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
		_, wpSpan := g.obsBridge.Tracer().Start(ctx, telemetry.OpD1_S15_Signal_TaskWorkProof,
			tracer.WithSpanKind(tracer.SpanKindInternal),
			tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpD1_S15_Signal_TaskWorkProof,
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
		return telemetry.OpD1_S14_Signal_Thinking
	case contracts.SignalTask:
		return telemetry.OpD1_S15_Signal_Task
	case contracts.SignalConclusion:
		return telemetry.OpD1_S16_Signal_Conclusion
	default:
		return telemetry.OpD1_S16_Signal_Conclusion
	}
}

func (g *CommunicationGateway) startCapturePersistSpan(ctx context.Context, sessionID string) {
	if g.obsBridge == nil || g.obsBridge.Tracer() == nil {
		return
	}
	_, span := g.obsBridge.Tracer().Start(ctx, telemetry.OpD1_S13_Capture_Persist,
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpD1_S13_Capture_Persist,
			tracer.Attribute{Key: "session.id", Value: sessionID},
		)...),
	)
	if span != nil {
		span.End()
	}
}

func (g *CommunicationGateway) startDispatchRouteSpan(ctx context.Context, sessionID, target string) (context.Context, func()) {
	if g.obsBridge == nil || g.obsBridge.Tracer() == nil {
		return ctx, func() {}
	}
	ctx, span := g.obsBridge.Tracer().Start(ctx, telemetry.OpD1_S13_Dispatch_Route,
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpD1_S13_Dispatch_Route,
			tracer.Attribute{Key: "session.id", Value: sessionID},
			tracer.Attribute{Key: "dispatch.target", Value: target},
		)...),
	)
	return ctx, func() {
		if span != nil {
			span.SetStatus(tracer.StatusCodeOk, "")
			span.End()
		}
	}
}

func enrichOutboundMetadata(meta map[string]string, sig contracts.IMOutboundSignal) map[string]string {
	return kernel.EnrichMetadata(meta, sig)
}
