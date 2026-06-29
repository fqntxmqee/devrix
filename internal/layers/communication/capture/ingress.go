package capture

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

// RouteInbound processes an inbound message from an adapter.
//
// DSAFT: D1-S13 CaptureUserIntent — A01 Accept, A02 Persist, A03 DispatchToAgent.
func (g *CommunicationGateway) RouteInbound(ctx context.Context, msg *types.InboundMessage) error {
	ctx, endSpan := g.startInboundSpan(ctx, msg)
	ctx = g.seedInboundBaggage(ctx, msg)

	slog.Info("gateway: RouteInbound called", "sessionID", msg.SessionID, "content", msg.Content, "chatID", msg.ChatID)

	if g.metricInboundMsgs != nil {
		g.metricInboundMsgs.Inc()
	}

	if msg.Content == "" {
		return errors.NewMessageEmptyError()
	}
	if len(msg.Content) > 64000 {
		return errors.WithCode("COMM_MESSAGE_TOO_LONG", "message too long", errors.ErrMessageTooLong)
	}

	session, err := g.getOrCreateSession(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to get or create session: %w", err)
	}

	if g.config.Session.IdleTimeout > 0 && session.IsIdle(g.config.Session.IdleTimeout) {
		slog.Info("gateway: resuming idle session on inbound message", "sessionID", session.SessionID)
	}

	session.LastMessageAt = g.clock.Now()
	session.RequestID = msg.MessageID
	g.mu.Lock()
	if cached, ok := g.sessions[session.SessionID]; ok && cached != nil {
		cached.LastMessageAt = session.LastMessageAt
		cached.RequestID = session.RequestID
	} else {
		g.sessions[session.SessionID] = session
	}
	g.mu.Unlock()

	_, storeSpan := g.startStoreUpdateSpan(ctx, session.SessionID)
	if err := g.sessionStore.Update(session); err != nil {
		if storeSpan != nil {
			storeSpan.RecordError(err)
			storeSpan.End()
		}
		slog.Warn("failed to update session", "sessionID", session.SessionID)
	} else if storeSpan != nil {
		storeSpan.End()
	}

	g.startCapturePersistSpan(ctx, session.SessionID)

	if feedback, reason := contracts.ParseConclusionFeedback(msg.Content); feedback {
		g.recordConclusionFeedback(ctx, session, reason)
		endSpan()
		return nil
	}

	g.beginInboundTurn(session.SessionID, msg.MessageID)

	if g.orchestrationEntry == nil {
		return fmt.Errorf("orchestration entry not configured")
	}
	if g.beforeDispatch != nil {
		if err := g.beforeDispatch(ctx, session); err != nil {
			return err
		}
	}

	processCtx, cancel := context.WithCancel(ctx)
	g.registerProcess(session.SessionID, cancel)

	processCtx, endDispatch := g.startDispatchRouteSpan(processCtx, session.SessionID, "d7")
	ch, err := g.orchestrationEntry.ProcessMessage(processCtx, session.SessionID, msg.Content)
	if err != nil {
		endDispatch()
		cancel()
		g.unregisterProcess(session.SessionID)
		return fmt.Errorf("d7 entry ProcessMessage: %w", err)
	}

	g.processes.Add(1)
	go func() {
		defer g.processes.Done()
		defer endDispatch()
		defer endSpan()
		defer cancel()
		defer g.endInboundTurn(session.SessionID)
		g.handleEngineEvents(processCtx, session, ch)
		g.unregisterProcess(session.SessionID)
		g.persistSessionAfterProcess(session)
	}()
	return nil
}
