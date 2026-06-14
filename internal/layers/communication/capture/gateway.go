package capture

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/delivery/eventbus"
	"github.com/devrix/devrix/internal/layers/communication/capture/signal"
	"github.com/devrix/devrix/internal/layers/communication/kernel"
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/metrics"
	"github.com/devrix/devrix/internal/layers/observability/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/tracer"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

// EventHandler defines the interface for handling gateway events
type EventHandler interface {
	OnMessage(msg *types.OutboundMessage)
	OnPermissionRequest(req *types.PermissionRequest) bool
	OnError(err error, sessionID string)
	OnStatus(sessionID string, state types.SessionState)
}

// IContextEngine is the L1 alias for the cross-layer engine contract.
type IContextEngine = contracts.IEngine

// EngineEvent is the L1 alias for engine events.
type EngineEvent = contracts.EngineEvent

// CommunicationGateway routes messages between adapters and D7 orchestration.
//
// DSAFT: D1-S13-A03 DispatchToAgent
type CommunicationGateway struct {
	sessionStore     SessionStore
	eventHandler     EventHandler
	permissionMgr    *PermissionManager
	config           *config.CommunicationConfig
	obsBridge        *observability.Bridge
	orchestrationEntry contracts.IOrchestrationEntry
	snapshotExporter contracts.ISessionSnapshotExporter

	mu                   sync.RWMutex
	sessions             map[string]*types.Session
	activeProcesses      map[string]context.CancelFunc
	processes            sync.WaitGroup
	stoppedSessions      sync.Map // sessionID → struct{}, set by Stop() to suppress post-stop errors
	agentFactory         multiagent.IAgentFactory
	agentObserverFactory func(ctx context.Context, session *types.Session) multiagent.AgentObserver
	sessionAgents        map[string]multiagent.Agent

	// metrics
	metricInboundMsgs    metrics.Counter
	metricOutboundMsgs   metrics.Counter
	metricSessionsTotal  metrics.Counter
	metricActiveSessions metrics.Gauge

	// eventDispatcher routes engine events through the backpressure
	// bus when wired (DM-20260611-003). When nil, events flow
	// synchronously through handleEngineEvents as before.
	eventDispatcher *eventDispatcher

	turnTracker *signal.TurnTracker
	clock       clock
	presenter   SignalRouter
}

// NewCommunicationGateway creates a new CommunicationGateway
func NewCommunicationGateway(
	sessionStore SessionStore,
	eventHandler EventHandler,
	permissionMgr *PermissionManager,
	cfg *config.CommunicationConfig,
) *CommunicationGateway {
	gw := &CommunicationGateway{
		sessionStore:    sessionStore,
		eventHandler:    eventHandler,
		permissionMgr:   permissionMgr,
		config:          cfg,
		sessions:        make(map[string]*types.Session),
		activeProcesses: make(map[string]context.CancelFunc),
		turnTracker:     signal.NewTurnTracker(),
		clock:           realClock{},
	}
	return gw
}

// SetOrchestrationEntry wires D7 as the sole non-agent inbound dispatch target.
func (g *CommunicationGateway) SetOrchestrationEntry(entry contracts.IOrchestrationEntry) {
	g.orchestrationEntry = entry
	if entry != nil {
		slog.Info("gateway: D1→D7.ProcessMessage path active")
	}
}

// SetSessionSnapshotExporter wires optional session context export after process.
func (g *CommunicationGateway) SetSessionSnapshotExporter(exp contracts.ISessionSnapshotExporter) {
	g.snapshotExporter = exp
}

// StopProcess cancels the active context engine process for the given session.
func (g *CommunicationGateway) StopProcess(sessionID string) error {
	return g.Stop(sessionID)
}

// Stop implements commands.Stopper — cancels the active context engine Process.
//
// When D7 is enabled, Stop also calls orchestrationEntry.Cancel which runs the
// full Wave→D4→Process sequence plus the stopped event (per R2 命题 E).
func (g *CommunicationGateway) Stop(sessionID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if cancel, ok := g.activeProcesses[sessionID]; ok {
		cancel()
		delete(g.activeProcesses, sessionID)
	}
	g.stoppedSessions.Store(sessionID, struct{}{})
	// D7 cancel is best-effort. The interrupt handler is idempotent.
	if g.orchestrationEntry != nil {
		// Use a fresh context with a short timeout to avoid blocking Stop.
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := g.orchestrationEntry.Cancel(ctx, sessionID); err != nil {
			slog.Warn("gateway: d7 Cancel failed", "sessionID", sessionID, "err", err)
		}
	}
	return nil
}

func (g *CommunicationGateway) registerProcess(sessionID string, cancel context.CancelFunc) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if prev, ok := g.activeProcesses[sessionID]; ok {
		prev()
	}
	g.activeProcesses[sessionID] = cancel
}

func (g *CommunicationGateway) unregisterProcess(sessionID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.activeProcesses, sessionID)
}

// WaitForProcesses blocks until all in-flight RouteInbound goroutines
// (including the post-persist session store writes) have completed. Intended
// for tests that need deterministic shutdown before t.TempDir cleanup.
func (g *CommunicationGateway) WaitForProcesses() {
	g.processes.Wait()
}

// SetObservability wires tracing/metrics into the capture.
func (g *CommunicationGateway) SetObservability(obs *observability.Observability) {
	if obs == nil {
		g.obsBridge = nil
		return
	}
	g.obsBridge = observability.NewBridge(obs)
	g.initMetrics(obs)
	if g.permissionMgr != nil {
		g.permissionMgr.SetObservability(obs)
	}
}

// SetEventBus wires a BackpressureEventBus into the capture. When set,
// engine events are routed through the bus (Publish for Normal/Low,
// PublishCritical for complete/error). When nil, the gateway falls back
// to its original synchronous fanout path — fully wire-compatible.
//
// This is a bootstrap-time setter; calling it while processes are
// in-flight is not supported.
func (g *CommunicationGateway) SetEventBus(bus EventBusPort) {
	if g.eventDispatcher == nil {
		g.eventDispatcher = newEventDispatcher(bus)
	} else {
		g.eventDispatcher.SetBus(bus)
	}
}

// EventBusEnabled reports whether a backpressure event bus is wired in.
func (g *CommunicationGateway) EventBusEnabled() bool {
	return g.eventDispatcher != nil && g.eventDispatcher.IsEnabled()
}

func (g *CommunicationGateway) initMetrics(obs *observability.Observability) {
	if g.obsBridge == nil || g.obsBridge.Meter() == nil {
		return
	}
	m := g.obsBridge.Meter()
	g.metricInboundMsgs, _ = m.Int64Counter("gateway_inbound_messages", metrics.WithLabels(metrics.LabelMap{
		"adapter": "all",
	}))
	g.metricOutboundMsgs, _ = m.Int64Counter("gateway_outbound_messages", metrics.WithLabels(metrics.LabelMap{
		"event_type": "all",
	}))
	g.metricSessionsTotal, _ = m.Int64Counter("gateway_sessions_total", metrics.WithLabels(metrics.LabelMap{
		"adapter": "all",
	}))
	sessionBridge := observability.NewSessionBridge(obs)
	if sessionBridge != nil {
		g.metricActiveSessions, _ = sessionBridge.ActiveSessions("all")
	}
}

// StartCleanupRoutine starts a background goroutine that periodically cleans up expired sessions
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

// cleanupExpiredSessions removes expired sessions from the in-memory cache
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

// RouteInbound processes an inbound message from an adapter
func (g *CommunicationGateway) RouteInbound(ctx context.Context, msg *types.InboundMessage) error {
	ctx, endSpan := g.startInboundSpan(ctx, msg)
	ctx = g.seedInboundBaggage(ctx, msg)

	slog.Info("gateway: RouteInbound called", "sessionID", msg.SessionID, "content", msg.Content, "chatID", msg.ChatID)

	// Record inbound metric
	if g.metricInboundMsgs != nil {
		g.metricInboundMsgs.Inc()
	}

	// Validate message
	if msg.Content == "" {
		return errors.NewMessageEmptyError()
	}

	if len(msg.Content) > 64000 {
		return errors.WithCode("COMM_MESSAGE_TOO_LONG", "message too long", errors.ErrMessageTooLong)
	}

	// Get or create session
	session, err := g.getOrCreateSession(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to get or create session: %w", err)
	}

	// Inbound message means the user is active again — resume idle sessions instead
	// of rejecting. Background cleanup (StartCleanupRoutine) handles abandoned sessions.
	if g.config.Session.IdleTimeout > 0 && session.IsIdle(g.config.Session.IdleTimeout) {
		slog.Info("gateway: resuming idle session on inbound message", "sessionID", session.SessionID)
	}

	// Update session
	session.LastMessageAt = time.Now()
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
	} else {
		if storeSpan != nil {
			storeSpan.End()
		}
	}

	g.startCapturePersistSpan(ctx, session.SessionID)

	if feedback, reason := contracts.ParseConclusionFeedback(msg.Content); feedback {
		g.recordConclusionFeedback(ctx, session, reason)
		endSpan()
		return nil
	}

	g.beginInboundTurn(session.SessionID, msg.MessageID)

	if g.agentFactory != nil {
		return g.routeInboundViaAgent(ctx, msg, session, endSpan)
	}

	if g.orchestrationEntry == nil {
		return fmt.Errorf("orchestration entry not configured")
	}

	processCtx, cancel := context.WithCancel(ctx)
	g.registerProcess(session.SessionID, cancel)

	g.startDispatchRouteSpan(ctx, session.SessionID, "d7")
	ch, err := g.orchestrationEntry.ProcessMessage(processCtx, session.SessionID, msg.Content)
	if err != nil {
		cancel()
		g.unregisterProcess(session.SessionID)
		return fmt.Errorf("d7 entry ProcessMessage: %w", err)
	}
	eventChan := ch

	// Handle events from context engine
	g.processes.Add(1)
	go func() {
		defer g.processes.Done()
		defer endSpan()
		defer cancel()
		defer g.endInboundTurn(session.SessionID)
		g.handleEngineEvents(processCtx, session, eventChan)
		g.unregisterProcess(session.SessionID)
		g.persistSessionAfterProcess(session)
	}()

	return nil
}

// handleEngineEvents processes events from the context engine.
//
// When an event bus is wired, events are pushed through the bus
// (g.eventDispatcher.Publish) and a dedicated consumer goroutine
// dispatches them back to handleEngineEvent — giving us backpressure
// at the bus boundary. When no bus is wired, the original synchronous
// fanout path is preserved.
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

			// Drain remaining buffered events so stop notification and
			// final events from the engine are still processed.
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

// handleEngineEventsViaBus pushes engine events into the bus and runs
// a consumer goroutine that pulls fanout events back and dispatches
// them via handleEngineEvent. Returns ONLY after both the producer
// (events channel) and the consumer (bus subscription) have completed —
// this preserves the original ordering: handleEngineEvents → unregister
// → persist in the RouteInbound goroutine, with no concurrent
// session-state mutation.
//
// Teardown sequence (deterministic, no sleeps):
//  1. Producer drains `events` into the bus until the channel closes.
//  2. After producerDone, we call cancelSub. cancelSub atomically
//     (under subsMu) removes the sub from the bus and closes sub.done.
//     - New fanout calls won't include this sub (removed from subs).
//     - In-flight fanout calls see done-closed and skip the send.
//  3. The consumer observes done-closed, drains any remaining events
//     already buffered in sub.ch (non-blocking), and exits.
//  4. We wait for consumerDone before returning — guarantees all
//     session-state mutations (e.g. SetState) have completed.
func (g *CommunicationGateway) handleEngineEventsViaBus(ctx context.Context, session *types.Session, events <-chan *EngineEvent) {
	// Subscribe SYNCHRONOUSLY before starting the consumer goroutine
	// so the bus's fanout path can see the subscription.
	subscribe := extractBusSubscribe(g.eventDispatcher.bus)
	var ch <-chan eventbus.Event
	var doneSub <-chan struct{}
	var cancelSub func()
	if subscribe != nil {
		_, ch, doneSub, cancelSub = subscribe(session.SessionID)
		g.eventDispatcher.SetSubCancel(cancelSub)
	}

	// consumerDone signals that the consumer has fully drained the
	// bus's subscriber channel and exited.
	consumerDone := make(chan struct{})
	g.processes.Add(1)
	go func() {
		defer g.processes.Done()
		defer close(consumerDone)
		g.handleEngineEventsBusConsumer(ctx, session, ch, doneSub)
	}()

	// Producer: forward from the context engine's event channel into
	// the bus. This is a tiny, non-blocking pump. We exit when the
	// source channel closes or ctx fires.
	producerDone := make(chan struct{})
	go func() {
		defer close(producerDone)
		slog.Info("gateway: bus producer started", "sessionID", session.SessionID)
		defer slog.Info("gateway: bus producer exited", "sessionID", session.SessionID)
		for {
			select {
			case <-ctx.Done():
				// Drain remaining buffered events so stop notification and
				// final events from the engine are still processed.
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

	// Wait for the producer to finish forwarding all events into the
	// bus. The bus's fanout + monitor goroutines will pick them up
	// and deliver to sub.ch asynchronously.
	<-producerDone
	// Wait for the bus's monitor to drain all Normal/Low events that
	// the producer just enqueued into normalCh. This eliminates the
	// race where cancelSub() removes our subscription from the bus
	// BEFORE the monitor's fanout has had a chance to deliver the
	// event to sub.ch (without this wait, a slow monitor would
	// observe the sub-already-removed on its next fanout and skip
	// the send — silently dropping the event).
	//
	// We bound the wait to a few seconds to avoid hanging on a
	// pathological monitor stall. The monitor's normalCh-receive
	// path runs on a 20ms ticker, so backlog=0 is expected within
	// a few iterations of the producer finishing.
	if g.eventDispatcher != nil && g.eventDispatcher.bus != nil {
		bus := g.eventDispatcher.bus
		deadline := time.Now().Add(2 * time.Second)
		for bus.Backlog() > 0 && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
	}
	// Cancel the subscription. This atomically (under subsMu) removes
	// the sub from the bus and closes sub.done — so no new events can
	// be fanned to this consumer and any in-flight fanout will see
	// done-closed and skip the send.
	if cancelSub != nil {
		cancelSub()
	}
	// Wait for the consumer to finish processing all fanned-out
	// events (which mutates session state) before returning.
	<-consumerDone
}

// handleEngineEventsBusConsumer is the consumer side of the bus-pump.
// It reads events from the bus's subscriber channel and dispatches
// them to handleEngineEvent. It exits when ANY of:
//   - ctx.Done() fires (caller cancellation)
//   - doneSub is closed (the bus's cancel was called, and the
//     consumer has drained everything currently in ch — including
//     a bounded settle period to absorb any in-flight fanout)
//   - ch is closed (the bus itself was Close()d)
//
// When doneSub is closed, the consumer first waits a small "settle"
// period (so any in-flight fanout that has already started can
// complete and deliver to ch), then drains ch (non-blocking) before
// exiting. This is the critical correctness window for
// complete/error events: the publisher's PublishCritical rendezvous
// is already complete by the time the event lands in ch, so the
// event must be in our buffer and dispatched. The settle period is
// bounded to keep the gateway's shutdown latency small.
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
			// Bus cancelled the subscription. Poll for up to
			// 1 second to give any in-flight monitor fanout a
			// chance to deliver Normal events to ch before
			// we exit. The fanout is non-blocking with a
			// default case, but it can be preempted between
			// PublishCritical release and the next monitor
			// select. The polling loop closes the race window
			// without a fixed sleep (S4-Gate review fix,
			// 2026-06-12).
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

// handleEngineEvent handles a single engine event
func (g *CommunicationGateway) handleEngineEvent(ctx context.Context, session *types.Session, event *EngineEvent) {
	slog.Info("gateway: handleEngineEvent", "type", event.Type, "sessionID", session.SessionID)

	// If the gateway was constructed without an eventHandler (e.g. for
	// idle-resume / session-store integration tests that only assert on
	// gateway-side state, not on delivered OutboundMessages), still
	// process state transitions + metrics, but skip the handler
	// callbacks. This preserves the v0.9 test contract where nil
	// handler was acceptable.
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

	// Record outbound metric
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
		g.presenter.DispatchError(in, emit, suppress)
	default:
		g.presenter.Dispatch(in, emit)
	}
}

// RouteOutbound sends an outbound message to the adapter
func (g *CommunicationGateway) RouteOutbound(msg *types.OutboundMessage) error {
	g.eventHandler.OnMessage(msg)
	return nil
}

// RoutePermission handles a permission request
func (g *CommunicationGateway) RoutePermission(req *types.PermissionRequest) (bool, error) {
	_, permSpan := g.startPermissionSpan(context.Background(), req)
	approved := g.eventHandler.OnPermissionRequest(req)
	if permSpan != nil {
		permSpan.SetAttributes(
			tracer.Attribute{Key: "permission.result", Value: fmt.Sprintf("%t", approved)},
		)
		permSpan.SetStatus(tracer.StatusCodeOk, "")
		permSpan.End()
	}
	return approved, nil
}

// RouteError sends an error to the adapter
func (g *CommunicationGateway) RouteError(err error, sessionID string) {
	g.eventHandler.OnError(err, sessionID)
}

// CreateSession creates a new session
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

	// Record session creation metric
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

// GetSession returns a session by ID
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

// sessionRestoreScore ranks sessions for post-restart recovery.
// Recency is primary; snapshot size is a tiebreaker within the same second so we
// don't resurrect a stale large snapshot over a newer empty session.
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

// ExpireSession marks a session as expired
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

	// Also delete from persistent store to prevent storage leak
	if err := g.sessionStore.Delete(sessionID); err != nil {
		// Log but don't fail - session is already removed from memory
		// The store implementation may not support delete or already cleaned up
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

func (g *CommunicationGateway) startSessionExpireSpan(ctx context.Context, sessionID string) (context.Context, tracer.Span) {
	if g.obsBridge == nil || g.obsBridge.Tracer() == nil {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpGatewaySessionExpire,
			tracer.Attribute{Key: "session.id", Value: sessionID},
		)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return g.obsBridge.Tracer().Start(ctx, telemetry.OpGatewaySessionExpire, opts...)
}

// getOrCreateSession gets an existing session or creates a new one
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

func (g *CommunicationGateway) startSessionGetSpan(ctx context.Context, sessionID string) (context.Context, tracer.Span) {
	if g.obsBridge == nil || g.obsBridge.Tracer() == nil {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpGatewaySessionGet,
			tracer.Attribute{Key: "session.id", Value: sessionID},
		)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return g.obsBridge.Tracer().Start(ctx, telemetry.OpGatewaySessionGet, opts...)
}

// generateSessionID generates a unique session ID
func generateSessionID() string {
	return fmt.Sprintf("sess_%d_%d", time.Now().UnixMilli(), time.Now().UnixNano()%10000)
}

func generateMessageID() string {
	return kernel.NewMessageID()
}

func outboundMetadata(eventType string, event *EngineEvent) map[string]string {
	var meta map[string]string
	if event != nil {
		meta = event.Metadata
	}
	return kernel.OutboundMetadata(eventType, meta)
}

// parseRiskLevel parses a risk level string
func parseRiskLevel(level string) types.RiskLevel {
	switch level {
	case "LOW":
		return types.RiskLevelLow
	case "MEDIUM":
		return types.RiskLevelMedium
	case "HIGH":
		return types.RiskLevelHigh
	case "CRITICAL":
		return types.RiskLevelCritical
	default:
		return types.RiskLevelMedium
	}
}

func (g *CommunicationGateway) recordSessionLifecycle(sessionID, adapter, action string) {
	if g.obsBridge == nil || g.obsBridge.Tracer() == nil {
		return
	}
	if adapter == "" {
		adapter = "unknown"
	}
	_, span := g.obsBridge.Tracer().Start(context.Background(), telemetry.OpGatewaySessionLifecycle,
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpGatewaySessionLifecycle,
			tracer.Attribute{Key: "session.action", Value: action},
			tracer.Attribute{Key: "session.id", Value: sessionID},
			tracer.Attribute{Key: "adapter", Value: adapter},
		)...),
	)
	span.End()
}

func (g *CommunicationGateway) startEngineEventSpan(ctx context.Context, session *types.Session, eventType string) (context.Context, tracer.Span) {
	if g.obsBridge == nil || g.obsBridge.Tracer() == nil {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpGatewayEngineEvent,
			tracer.Attribute{Key: "session.id", Value: session.SessionID},
			tracer.Attribute{Key: "event.type", Value: eventType},
		)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return g.obsBridge.Tracer().Start(ctx, telemetry.OpGatewayEngineEvent, opts...)
}

func (g *CommunicationGateway) startStoreUpdateSpan(ctx context.Context, sessionID string) (context.Context, tracer.Span) {
	if g.obsBridge == nil || g.obsBridge.Tracer() == nil {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpGatewayStoreUpdate,
			tracer.Attribute{Key: "session.id", Value: sessionID},
		)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return g.obsBridge.Tracer().Start(ctx, telemetry.OpGatewayStoreUpdate, opts...)
}

func (g *CommunicationGateway) startSessionCreateSpan(ctx context.Context, chatID, workDir string) (context.Context, tracer.Span) {
	if g.obsBridge == nil || g.obsBridge.Tracer() == nil {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpGatewaySessionCreate,
			tracer.Attribute{Key: "adapter", Value: "cli"},
			tracer.Attribute{Key: "work_dir", Value: workDir},
		)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return g.obsBridge.Tracer().Start(ctx, telemetry.OpGatewaySessionCreate, opts...)
}

func (g *CommunicationGateway) startPermissionSpan(ctx context.Context, req *types.PermissionRequest) (context.Context, tracer.Span) {
	if g.obsBridge == nil || g.obsBridge.Tracer() == nil {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpGatewayPermissionCheck,
			tracer.Attribute{Key: "session.id", Value: req.SessionID},
			tracer.Attribute{Key: "tool.name", Value: req.ToolName},
			tracer.Attribute{Key: "risk_level", Value: string(req.RiskLevel)},
		)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return g.obsBridge.Tracer().Start(ctx, telemetry.OpGatewayPermissionCheck, opts...)
}

func (g *CommunicationGateway) startInboundSpan(ctx context.Context, msg *types.InboundMessage) (context.Context, func()) {
	if g.obsBridge == nil || g.obsBridge.Tracer() == nil {
		return ctx, func() {}
	}

	tr := g.obsBridge.Tracer()
	ctx, span := tr.Start(ctx, telemetry.OpGatewayMessageReceive,
		tracer.WithSpanKind(tracer.SpanKindServer),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpGatewayMessageReceive,
			tracer.Attribute{Key: "session.id", Value: msg.SessionID},
			tracer.Attribute{Key: "message.adapter_id", Value: msg.AdapterID},
			tracer.Attribute{Key: "message.chat_id", Value: msg.ChatID},
			tracer.Attribute{Key: "message.user_id", Value: msg.UserID},
			tracer.Attribute{Key: "message.len", Value: fmt.Sprintf("%d", len(msg.Content))},
		)...),
	)

	return ctx, func() {
		span.SetStatus(tracer.StatusCodeOk, "")
		span.End()
	}
}

func (g *CommunicationGateway) seedInboundBaggage(ctx context.Context, msg *types.InboundMessage) context.Context {
	if msg == nil {
		return ctx
	}
	bm := tracer.DefaultBaggageManager
	ctx = bm.Set(ctx, "session.id", msg.SessionID)
	if msg.UserID != "" {
		ctx = bm.Set(ctx, "user.id", msg.UserID)
	}
	return ctx
}
