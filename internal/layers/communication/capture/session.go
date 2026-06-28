package capture

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/capture/transcript"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

// CreateSession creates a new session.
func (g *CommunicationGateway) CreateSession(chatID, workDir string) (*types.Session, error) {
	if workDir == "" {
		if wd, err := os.Getwd(); err == nil {
			workDir = wd
		}
	}

	_, createSpan := g.startSessionCreateSpan(context.Background(), chatID, workDir)

	sessionID := generateSessionID()
	session := types.NewSession(sessionID, "cli", workDir)
	session.ChatID = chatID

	if err := g.sessionStore.Create(session); err != nil {
		if createSpan != nil {
			createSpan.End()
		}
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	g.mu.Lock()
	g.sessions[sessionID] = session
	g.mu.Unlock()

	if g.metricSessionsTotal != nil {
		g.metricSessionsTotal.Inc()
	}
	if g.metricActiveSessions != nil {
		g.metricActiveSessions.Inc()
	}
	if createSpan != nil {
		createSpan.SetAttributes(tracer.Attribute{Key: "session.id", Value: sessionID})
		createSpan.End()
	}

	return session, nil
}

// GetSession returns a session by ID.
func (g *CommunicationGateway) GetSession(sessionID string) (*types.Session, error) {
	session, err := g.sessionStore.Get(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	if session == nil {
		return nil, errors.NewSessionNotFoundError(sessionID)
	}
	return session, nil
}

// ResolveSessionByChatID returns the most recently active session for chatID
// that has not exceeded the idle timeout. Used to recover context after restart.
func (g *CommunicationGateway) ResolveSessionByChatID(chatID string) (*types.Session, error) {
	if chatID == "" {
		return nil, nil
	}

	sessions, err := g.sessionStore.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	var best *types.Session
	var bestScore int64
	for _, session := range sessions {
		if session == nil || session.ChatID != chatID {
			continue
		}
		score := sessionRestoreScore(session)
		if best == nil || score > bestScore {
			best = session
			bestScore = score
		}
	}
	if best == nil {
		return nil, nil
	}

	g.mu.Lock()
	g.sessions[best.SessionID] = best
	g.mu.Unlock()

	slog.Info("gateway: restored session from store",
		"sessionID", best.SessionID,
		"chatID", chatID,
		"snapshotBytes", len(best.ContextSnapshot),
	)
	return best, nil
}

func sessionRestoreScore(session *types.Session) int64 {
	if session == nil {
		return 0
	}
	const maxSnapshotBoost = 1_000_000
	snapshotBoost := int64(len(session.ContextSnapshot))
	if snapshotBoost > maxSnapshotBoost {
		snapshotBoost = maxSnapshotBoost
	}
	return session.LastMessageAt.Unix()*1_000_000 + snapshotBoost
}

// ExpireSession marks a session as expired.
func (g *CommunicationGateway) ExpireSession(sessionID string) error {
	_, expireSpan := g.startSessionExpireSpan(context.Background(), sessionID)

	session, err := g.sessionStore.Get(sessionID)
	if err != nil {
		if expireSpan != nil {
			expireSpan.End()
		}
		return fmt.Errorf("failed to get session: %w", err)
	}
	if session == nil {
		if expireSpan != nil {
			expireSpan.End()
		}
		return errors.NewSessionNotFoundError(sessionID)
	}

	session.State = types.SessionStateFailed
	if err := g.sessionStore.Update(session); err != nil {
		if expireSpan != nil {
			expireSpan.End()
		}
		return fmt.Errorf("failed to update session: %w", err)
	}

	g.mu.Lock()
	delete(g.sessions, sessionID)
	g.mu.Unlock()

	if w := g.writer; w != nil {
		_ = w.Append(sessionID, transcript.Event{
			Kind: "session_close",
			Body: "expired",
		})
	}

	if err := g.sessionStore.Delete(sessionID); err != nil {
		// Log but don't fail - session is already removed from memory
	}

	if g.metricActiveSessions != nil {
		g.metricActiveSessions.Dec()
	}
	if expireSpan != nil {
		expireSpan.SetAttributes(tracer.Attribute{Key: "adapter", Value: session.AdapterID})
		expireSpan.End()
	}

	return nil
}

// StartCleanupRoutine starts a background goroutine that periodically cleans up expired sessions.
func (g *CommunicationGateway) StartCleanupRoutine(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				g.cleanupExpiredSessions()
			}
		}
	}()
}

func (g *CommunicationGateway) cleanupExpiredSessions() {
	g.mu.Lock()
	defer g.mu.Unlock()

	timeout := g.config.Session.IdleTimeout
	now := time.Now()

	for sessionID, session := range g.sessions {
		if _, active := g.activeProcesses[sessionID]; active {
			continue
		}
		if now.Sub(session.LastMessageAt) > timeout {
			slog.Debug("cleaning up expired session", "sessionID", sessionID)
			delete(g.sessions, sessionID)
		}
	}
}

func (g *CommunicationGateway) getOrCreateSession(ctx context.Context, msg *types.InboundMessage) (*types.Session, error) {
	if msg.SessionID != "" {
		_, getSpan := g.startSessionGetSpan(ctx, msg.SessionID)
		session, err := g.sessionStore.Get(msg.SessionID)
		if getSpan != nil {
			getSpan.End()
		}
		if err != nil {
			return nil, err
		}
		if session != nil {
			return session, nil
		}
	}

	return g.CreateSession(msg.ChatID, msg.Metadata["work_dir"])
}

func generateSessionID() string {
	return fmt.Sprintf("sess_%d_%d", time.Now().UnixMilli(), time.Now().UnixNano()%10000)
}

func (g *CommunicationGateway) startSessionExpireSpan(ctx context.Context, sessionID string) (context.Context, tracer.Span) {
	if g.obsBridge == nil || g.obsBridge.Tracer() == nil {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpD1_S13_Capture_Session_Expire,
			tracer.Attribute{Key: "session.id", Value: sessionID},
		)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return g.obsBridge.Tracer().Start(ctx, telemetry.OpD1_S13_Capture_Session_Expire, opts...)
}

func (g *CommunicationGateway) startSessionGetSpan(ctx context.Context, sessionID string) (context.Context, tracer.Span) {
	if g.obsBridge == nil || g.obsBridge.Tracer() == nil {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpD1_S13_Capture_Session_Get,
			tracer.Attribute{Key: "session.id", Value: sessionID},
		)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return g.obsBridge.Tracer().Start(ctx, telemetry.OpD1_S13_Capture_Session_Get, opts...)
}

func (g *CommunicationGateway) startSessionCreateSpan(ctx context.Context, chatID, workDir string) (context.Context, tracer.Span) {
	if g.obsBridge == nil || g.obsBridge.Tracer() == nil {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpD1_S13_Capture_Session_Create,
			tracer.Attribute{Key: "adapter", Value: "cli"},
			tracer.Attribute{Key: "work_dir", Value: workDir},
		)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return g.obsBridge.Tracer().Start(ctx, telemetry.OpD1_S13_Capture_Session_Create, opts...)
}

// recordSessionLifecycle emits a session lifecycle span (metrics bridge).
func (g *CommunicationGateway) recordSessionLifecycle(sessionID, adapter, action string) {
	if g.obsBridge == nil || g.obsBridge.Tracer() == nil {
		return
	}
	if adapter == "" {
		adapter = "unknown"
	}
	_, span := g.obsBridge.Tracer().Start(context.Background(), telemetry.OpD1_S13_Capture_Session_Lifecycle,
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpD1_S13_Capture_Session_Lifecycle,
			tracer.Attribute{Key: "session.action", Value: action},
			tracer.Attribute{Key: "session.id", Value: sessionID},
			tracer.Attribute{Key: "adapter", Value: adapter},
		)...),
	)
	span.End()
}
