package capture

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/capture/signal"
	"github.com/devrix/devrix/internal/layers/communication/capture/transcript"
	"github.com/devrix/devrix/internal/layers/communication/delivery/eventbus"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// DeliverOrphanEngineEvent handles engine events from background agents when no
// inbound ProcessMessage turn is active.
func (g *CommunicationGateway) DeliverOrphanEngineEvent(ctx context.Context, session *types.Session, event *EngineEvent) {
	g.handleEngineEvent(ctx, session, event)
}

func (g *CommunicationGateway) handleEngineEvents(ctx context.Context, session *types.Session, events <-chan *EngineEvent) {
	slog.Info("gateway: handleEngineEvents started", "sessionID", session.SessionID)
	if g.EventBusEnabled() {
		g.handleEngineEventsViaBus(ctx, session, events)
		return
	}
	for {
		select {
		case <-ctx.Done():
			slog.Info("gateway: handleEngineEvents ctx done", "sessionID", session.SessionID)
			for {
				select {
				case ev, ok := <-events:
					if !ok {
						return
					}
					g.handleEngineEvent(ctx, session, ev)
				default:
					return
				}
			}
		case event, ok := <-events:
			if !ok {
				slog.Info("gateway: handleEngineEvents channel closed", "sessionID", session.SessionID)
				return
			}
			g.handleEngineEvent(ctx, session, event)
		}
	}
}

func (g *CommunicationGateway) handleEngineEventsViaBus(ctx context.Context, session *types.Session, events <-chan *EngineEvent) {
	subscribe := extractBusSubscribe(g.eventDispatcher.bus)
	var ch <-chan eventbus.Event
	var doneSub <-chan struct{}
	var cancelSub func()
	if subscribe != nil {
		_, ch, doneSub, cancelSub = subscribe(session.SessionID)
		g.eventDispatcher.SetSubCancel(cancelSub)
	}

	consumerDone := make(chan struct{})
	g.processes.Add(1)
	go func() {
		defer g.processes.Done()
		defer close(consumerDone)
		g.handleEngineEventsBusConsumer(ctx, session, ch, doneSub)
	}()

	producerDone := make(chan struct{})
	go func() {
		defer close(producerDone)
		slog.Info("gateway: bus producer started", "sessionID", session.SessionID)
		defer slog.Info("gateway: bus producer exited", "sessionID", session.SessionID)
		for {
			select {
			case <-ctx.Done():
				for {
					select {
					case ev, ok := <-events:
						if !ok {
							return
						}
						g.handleEngineEvent(ctx, session, ev)
					default:
						return
					}
				}
			case event, ok := <-events:
				if !ok {
					return
				}
				slog.Info("gateway: bus producer publish", "type", event.Type, "sessionID", session.SessionID)
				g.eventDispatcher.Publish(ctx, event)
				slog.Info("gateway: bus producer publish done", "type", event.Type, "sessionID", session.SessionID)
			}
		}
	}()

	<-producerDone
	if g.eventDispatcher != nil && g.eventDispatcher.bus != nil {
		bus := g.eventDispatcher.bus
		deadline := time.Now().Add(2 * time.Second)
		for bus.Backlog() > 0 && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
	}
	if cancelSub != nil {
		cancelSub()
	}
	<-consumerDone
}

func (g *CommunicationGateway) handleEngineEventsBusConsumer(
	ctx context.Context,
	session *types.Session,
	ch <-chan eventbus.Event,
	doneSub <-chan struct{},
) {
	for {
		select {
		case <-ctx.Done():
			return

		case <-doneSub:
			deadline := time.Now().Add(1 * time.Second)
			for time.Now().Before(deadline) {
				select {
				case ev, ok := <-ch:
					if !ok {
						return
					}
					if ev.EngineEvent != nil {
						g.handleEngineEvent(ctx, session, ev.EngineEvent)
					}
				default:
					time.Sleep(time.Millisecond)
				}
			}
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if ev.EngineEvent != nil {
				g.handleEngineEvent(ctx, session, ev.EngineEvent)
			}
		}
	}
}

// PublishEngineEvent delivers an engine event for a session (async worker progress, etc.).
func (g *CommunicationGateway) PublishEngineEvent(ev *EngineEvent) {
	if g == nil || ev == nil || ev.SessionID == "" {
		return
	}
	session, err := g.GetSession(ev.SessionID)
	if err != nil || session == nil {
		slog.Debug("gateway: PublishEngineEvent session not found", "sessionID", ev.SessionID)
		return
	}
	go g.handleEngineEvent(context.Background(), session, ev)
}

func (g *CommunicationGateway) handleEngineEvent(ctx context.Context, session *types.Session, event *EngineEvent) {
	slog.Info("gateway: handleEngineEvent", "type", event.Type, "sessionID", session.SessionID)

	if g.eventHandler == nil {
		ApplyNilHandlerState(session, event.Type)
		return
	}

	ctx, evSpan := g.startEngineEventSpan(ctx, session, event.Type)
	if evSpan != nil {
		defer evSpan.End()
	}

	var sig contracts.IMOutboundSignal
	hasSig := false
	if g.turnTracker != nil {
		var chain signal.ChainReport
		var ok bool
		sig, chain, ok = g.turnTracker.Next(session.SessionID, event)
		if ok {
			hasSig = true
			g.emitOutboundSignalSpans(ctx, session, sig, chain, event)
		}
	}

	if w := g.writer; w != nil {
		var transcriptSpan tracer.Span
		if g.obsBridge != nil && g.obsBridge.Tracer() != nil {
			_, transcriptSpan = g.obsBridge.Tracer().Start(ctx, telemetry.OpD1_S13_Capture_Transcript_Append,
				tracer.WithSpanKind(tracer.SpanKindInternal),
				tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpD1_S13_Capture_Transcript_Append,
					tracer.Attribute{Key: "session.id", Value: session.SessionID},
					tracer.Attribute{Key: "event.type", Value: event.Type},
				)...),
			)
		}
		g.appendTranscriptEvent(w, session.SessionID, event)
		if transcriptSpan != nil {
			transcriptSpan.SetStatus(tracer.StatusCodeOk, "")
			transcriptSpan.End()
		}
	}

	if g.metricOutboundMsgs != nil {
		g.metricOutboundMsgs.Inc()
	}

	in := SignalInput{
		Session:   session,
		Event:     event,
		Signal:    sig,
		HasSignal: hasSig,
	}
	emit := g.eventHandler

	switch event.Type {
	case "error":
		suppress := false
		if _, stopped := g.stoppedSessions.LoadAndDelete(session.SessionID); stopped {
			slog.Debug("gateway: suppressing error event for stopped session",
				"sessionID", session.SessionID,
				"content", event.Content,
			)
			suppress = true
		}
		var dispatchSpan tracer.Span
		if g.obsBridge != nil && g.obsBridge.Tracer() != nil {
			_, dispatchSpan = g.obsBridge.Tracer().Start(ctx, telemetry.OpD1_S13_Capture_Dispatch,
				tracer.WithSpanKind(tracer.SpanKindClient),
				tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpD1_S13_Capture_Dispatch,
					tracer.Attribute{Key: "session.id", Value: session.SessionID},
					tracer.Attribute{Key: "event.type", Value: event.Type},
					tracer.Attribute{Key: "dispatch.suppressed", Value: fmt.Sprintf("%t", suppress)},
				)...),
			)
		}
		g.presenter.DispatchError(in, emit, suppress)
		if dispatchSpan != nil {
			dispatchSpan.SetStatus(tracer.StatusCodeOk, "")
			dispatchSpan.End()
		}
	default:
		var dispatchSpan tracer.Span
		if g.obsBridge != nil && g.obsBridge.Tracer() != nil {
			_, dispatchSpan = g.obsBridge.Tracer().Start(ctx, telemetry.OpD1_S13_Capture_Dispatch,
				tracer.WithSpanKind(tracer.SpanKindClient),
				tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpD1_S13_Capture_Dispatch,
					tracer.Attribute{Key: "session.id", Value: session.SessionID},
					tracer.Attribute{Key: "event.type", Value: event.Type},
				)...),
			)
		}
		g.presenter.Dispatch(in, emit)
		if dispatchSpan != nil {
			dispatchSpan.SetStatus(tracer.StatusCodeOk, "")
			dispatchSpan.End()
		}
	}
}

func (g *CommunicationGateway) appendTranscriptEvent(w *transcript.Writer, sessionID string, event *EngineEvent) {
	if w == nil || sessionID == "" || event == nil {
		return
	}
	var kind string
	var body string
	switch event.Type {
	case "text":
		kind = "assistant"
		body = event.Content
	case "thinking":
		kind = "thinking"
		body = event.Content
	case "tool_call":
		kind = "tool_call"
		body = event.ToolInput
		if event.ToolName != "" {
			body = event.ToolName + " " + body
		}
	case "tool_result":
		kind = "tool_result"
		body = event.Content
	case "complete":
		kind = "complete"
		body = event.Content
	default:
		return
	}
	if body == "" {
		return
	}
	_ = w.Append(sessionID, transcript.Event{
		Kind: kind,
		Role: "assistant",
		Body: body,
	})
}

func (g *CommunicationGateway) startEngineEventSpan(ctx context.Context, session *types.Session, eventType string) (context.Context, tracer.Span) {
	if g.obsBridge == nil || g.obsBridge.Tracer() == nil {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpD1_S13_Capture_EngineEvent_Handle,
			tracer.Attribute{Key: "session.id", Value: session.SessionID},
			tracer.Attribute{Key: "event.type", Value: eventType},
		)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return g.obsBridge.Tracer().Start(ctx, telemetry.OpD1_S13_Capture_EngineEvent_Handle, opts...)
}
