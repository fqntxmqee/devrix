package capture

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/capture/signal"
	"github.com/devrix/devrix/internal/layers/communication/capture/transcript"
	"github.com/devrix/devrix/internal/layers/communication/kernel"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/metrics"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// EventHandler defines the interface for handling gateway events.
type EventHandler interface {
	OnMessage(msg *types.OutboundMessage)
	OnPermissionRequest(req *types.PermissionRequest) bool
	OnError(err error, sessionID string)
	OnStatus(sessionID string, state types.SessionState)
}

// EngineEvent is the L1 alias for engine events.
type EngineEvent = contracts.EngineEvent

// CommunicationGateway routes messages between adapters and D7 orchestration.
//
// DSAFT: D1-S13-A03 DispatchToAgent
type CommunicationGateway struct {
	sessionStore       SessionStore
	eventHandler       EventHandler
	permissionMgr      *PermissionManager
	config             *config.CommunicationConfig
	obsBridge          *observability.Bridge
	orchestrationEntry contracts.IOrchestrationEntry
	snapshotExporter   contracts.ISessionSnapshotExporter

	mu              sync.RWMutex
	sessions        map[string]*types.Session
	activeProcesses map[string]context.CancelFunc
	processes       sync.WaitGroup
	stoppedSessions sync.Map

	metricInboundMsgs    metrics.Counter
	metricOutboundMsgs   metrics.Counter
	metricSessionsTotal  metrics.Counter
	metricActiveSessions metrics.Gauge

	eventDispatcher *eventDispatcher
	beforeDispatch  func(ctx context.Context, session *types.Session) error

	turnTracker *signal.TurnTracker
	clock       clock
	presenter   SignalRouter
	writer      *transcript.Writer
}

// NewCommunicationGateway creates a new CommunicationGateway.
func NewCommunicationGateway(
	sessionStore SessionStore,
	eventHandler EventHandler,
	permissionMgr *PermissionManager,
	cfg *config.CommunicationConfig,
	writer *transcript.Writer,
) *CommunicationGateway {
	return &CommunicationGateway{
		sessionStore:    sessionStore,
		eventHandler:    eventHandler,
		permissionMgr:   permissionMgr,
		config:          cfg,
		sessions:        make(map[string]*types.Session),
		activeProcesses: make(map[string]context.CancelFunc),
		turnTracker:     signal.NewTurnTracker(),
		clock:           realClock{},
		writer:          writer,
	}
}

// SetBeforeDispatch wires a hook invoked before D7 ProcessMessage (bootstrap sessionagents).
func (g *CommunicationGateway) SetBeforeDispatch(fn func(ctx context.Context, session *types.Session) error) {
	g.beforeDispatch = fn
}

// BeforeDispatch returns the currently wired pre-dispatch hook, or nil.
func (g *CommunicationGateway) BeforeDispatch() func(ctx context.Context, session *types.Session) error {
	if g == nil {
		return nil
	}
	return g.beforeDispatch
}

// HasActiveProcess reports whether a session has an in-flight D7 dispatch goroutine.
func (g *CommunicationGateway) HasActiveProcess(sessionID string) bool {
	if g == nil {
		return false
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	_, ok := g.activeProcesses[sessionID]
	return ok
}

// PermissionDefaultTimeout implements sessionagents.PermissionRouter.
func (g *CommunicationGateway) PermissionDefaultTimeout() time.Duration {
	if g.config == nil {
		return 60 * time.Second
	}
	return g.config.Permission.DefaultTimeout
}

// SetOrchestrationEntry wires D7 as the mandatory sole inbound dispatch target.
func (g *CommunicationGateway) SetOrchestrationEntry(entry contracts.IOrchestrationEntry) {
	g.orchestrationEntry = entry
	if entry != nil {
		slog.Info("gateway: D1→D7.ProcessMessage path active")
	}
}

// OrchestrationEntry returns the wired D7 entry, or nil if D7 is not active.
func (g *CommunicationGateway) OrchestrationEntry() contracts.IOrchestrationEntry {
	return g.orchestrationEntry
}

// SetSessionSnapshotExporter wires optional session context export after process.
func (g *CommunicationGateway) SetSessionSnapshotExporter(exp contracts.ISessionSnapshotExporter) {
	g.snapshotExporter = exp
}

// StopProcess cancels the active context engine process for the given session.
func (g *CommunicationGateway) StopProcess(sessionID string) error {
	return g.Stop(sessionID)
}

// Stop implements commands.Stopper — cancels the active D7 process.
func (g *CommunicationGateway) Stop(sessionID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if cancel, ok := g.activeProcesses[sessionID]; ok {
		cancel()
		delete(g.activeProcesses, sessionID)
	}
	g.stoppedSessions.Store(sessionID, struct{}{})
	if g.orchestrationEntry != nil {
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

// WaitForProcesses blocks until all in-flight RouteInbound goroutines have completed.
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

// RouteOutbound sends an outbound message to the adapter.
func (g *CommunicationGateway) RouteOutbound(msg *types.OutboundMessage) error {
	g.eventHandler.OnMessage(msg)
	return nil
}

// RoutePermission handles a permission request.
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

// RouteError sends an error to the adapter.
func (g *CommunicationGateway) RouteError(err error, sessionID string) {
	g.eventHandler.OnError(err, sessionID)
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

func (g *CommunicationGateway) startStoreUpdateSpan(ctx context.Context, sessionID string) (context.Context, tracer.Span) {
	if g.obsBridge == nil || g.obsBridge.Tracer() == nil {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpD1_S13_Capture_Store_Update,
			tracer.Attribute{Key: "session.id", Value: sessionID},
		)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return g.obsBridge.Tracer().Start(ctx, telemetry.OpD1_S13_Capture_Store_Update, opts...)
}

func (g *CommunicationGateway) startPermissionSpan(ctx context.Context, req *types.PermissionRequest) (context.Context, tracer.Span) {
	if g.obsBridge == nil || g.obsBridge.Tracer() == nil {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpD1_S13_Capture_Permission_Check,
			tracer.Attribute{Key: "session.id", Value: req.SessionID},
			tracer.Attribute{Key: "tool.name", Value: req.ToolName},
			tracer.Attribute{Key: "risk_level", Value: string(req.RiskLevel)},
		)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return g.obsBridge.Tracer().Start(ctx, telemetry.OpD1_S13_Capture_Permission_Check, opts...)
}

func (g *CommunicationGateway) startInboundSpan(ctx context.Context, msg *types.InboundMessage) (context.Context, func()) {
	if g.obsBridge == nil || g.obsBridge.Tracer() == nil {
		return ctx, func() {}
	}

	tr := g.obsBridge.Tracer()
	ctx, span := tr.Start(ctx, telemetry.OpD1_S13_Capture_Message_Receive,
		tracer.WithSpanKind(tracer.SpanKindServer),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpD1_S13_Capture_Message_Receive,
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
