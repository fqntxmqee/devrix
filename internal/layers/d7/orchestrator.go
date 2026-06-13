package d7

import (
	"context"
	"fmt"
	"sync"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// SessionOrchestrator is the D7-S2 entry point that replaces D1→D2.Process.
//
// D1 RouteInbound (when d7_enabled=true) calls ProcessMessage. The
// orchestrator:
//  1. Classifies the intent (rule-only in v1.0).
//  2. Routes to FastPath or OrchestratePath per the routing matrix.
//  3. Handles interrupts (HandleInterrupt) for /stop and D1 Stop.
//
// See d7-domain.md §Orchestration Routing Matrix.
type SessionOrchestrator struct {
	cfg        *Config
	classifier IntentClassifier
	executor   D2Executor
	fastPath   *FastPath
	workModel  WorkModel
	validator  D6Validator
	sink       D1EventSink

	// activeSessions tracks the running ProcessRequest per sessionID so
	// HandleInterrupt can cancel them. Protected by mu.
	mu             sync.Mutex
	activeSessions map[string]context.CancelFunc

	// interruptHandler is the cancel sequencer. Bootstrap wires it; if not
	// wired, Cancel() lazily constructs a no-op handler.
	interruptHandler *InterruptHandler
}

// OrchestratorOption is the functional-options constructor pattern.
type OrchestratorOption func(*SessionOrchestrator)

// WithSink wires a D1 event sink.
func WithSink(s D1EventSink) OrchestratorOption {
	return func(o *SessionOrchestrator) { o.sink = s }
}

// WithValidator wires the optional D6 advisory validator.
func WithValidator(v D6Validator) OrchestratorOption {
	return func(o *SessionOrchestrator) { o.validator = v }
}

// WithWorkModel wires a custom WorkModel. v1.0 uses NewDelegatedWorkModel()
// which forwards to D2 TaskManager.
func WithWorkModel(w WorkModel) OrchestratorOption {
	return func(o *SessionOrchestrator) { o.workModel = w }
}

// NewSessionOrchestrator builds the orchestrator with the given D2 executor
// and options. The classifier is built from cfg via NewRuleClassifier.
//
// Options are applied in order. WithSink and WithValidator must be passed
// before the orchestrator constructs the FastPath (i.e. via the returned
// instance), so the FastPath is built lazily on first ProcessMessage.
// This avoids the bug where WithSink would have been ignored because the
// FastPath was constructed before the option was applied.
func NewSessionOrchestrator(cfg *Config, executor D2Executor, opts ...OrchestratorOption) *SessionOrchestrator {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	o := &SessionOrchestrator{
		cfg:            cfg,
		classifier:     NewRuleClassifier(cfg),
		executor:       executor,
		workModel:      NewDelegatedWorkModel(),
		activeSessions: make(map[string]context.CancelFunc),
	}
	for _, opt := range opts {
		opt(o)
	}
	// Build FastPath now that sink (if any) has been applied.
	o.fastPath = NewFastPath(cfg, executor, o.sink)
	return o
}

// ProcessMessage is the D1→D7 entry point.
//
// Routing:
//   - skip        → return empty channel
//   - command     → handleCommand (v1.0: passthrough to D2 with a
//     system-prompt hint that this is a command)
//   - fast        → FastPath.Run (D2.RunQueryLoop direct)
//   - orchestrate → OrchestratePath (v1.0: route to PlanMode if active,
//     else to a single-task D2 call; Wave is a v1.1+
func (o *SessionOrchestrator) ProcessMessage(ctx context.Context, req ProcessRequest) (<-chan *contracts.EngineEvent, error) {
	intent, err := o.classifier.Classify(ctx, req.Message)
	if err != nil {
		return nil, fmt.Errorf("d7: classify: %w", err)
	}
	// Advisory D6 validation; timeout is treated as pass (D6 contract).
	if o.validator != nil {
		vctx, cancel := context.WithTimeout(ctx, durationOrDefault(o.cfg.D6ValidationTimeoutMs))
		_ = vctx
		cancel()
		// Per R2 P1, the D5 timeout counter is the observability hook.
		// Recording happens in the validator implementation.
		_ = o.validator.ValidateOrchestration(vctx, OrchestrationDecision{
			Intent:    intent,
			SessionID: req.SessionID,
		})
	}
	switch intent.Kind {
	case IntentSkip:
		ch := make(chan *contracts.EngineEvent)
		close(ch)
		return ch, nil
	case IntentCommand:
		return o.handleCommand(ctx, req, intent)
	case IntentFast:
		return o.fastPath.Run(ctx, req, "")
	case IntentOrchestrate:
		return o.orchestrate(ctx, req, intent)
	default:
		return nil, fmt.Errorf("d7: unknown intent kind %q", intent.Kind)
	}
}

// handleCommand is the command-first path. v1.0 simply forwards to D2 with
// a system prompt hint; the command semantics are handled by D2/CLI
// downstream (e.g. /plan is a PlanMode trigger inside D2 currently, and
// BackgroundTask is a D2 BackgroundRun).
func (o *SessionOrchestrator) handleCommand(ctx context.Context, req ProcessRequest, intent IntentClassification) (<-chan *contracts.EngineEvent, error) {
	return o.fastPath.Run(ctx, req, "[command:"+intent.Command+"]")
}

// orchestrate is the multi-step path. v1.0 supports a single D2 task; the
// Wave/Plan integration is wired but Plan creation requires v1.1
// SynthesizeTaskGraph. In v1.0 we route to FastPath with a system prompt
// that asks the LLM to plan internally.
func (o *SessionOrchestrator) orchestrate(ctx context.Context, req ProcessRequest, _ IntentClassification) (<-chan *contracts.EngineEvent, error) {
	return o.fastPath.Run(ctx, req, "[orchestrate: please decompose and execute step by step]")
}

// ProcessMessageContract satisfies contracts.IOrchestrationEntry. It is the
// D1 gateway-facing seam (string args instead of ProcessRequest).
func (o *SessionOrchestrator) ProcessMessageContract(ctx context.Context, sessionID, message string) (<-chan *contracts.EngineEvent, error) {
	return o.ProcessMessage(ctx, ProcessRequest{SessionID: sessionID, Message: message})
}

// Entry is the adapter that exposes SessionOrchestrator as
// contracts.IOrchestrationEntry. D1 gateway takes the Entry pointer, not
// the SessionOrchestrator, so the contract surface is decoupled from the
// orchestrator internals.
type Entry struct {
	*SessionOrchestrator
}

// NewEntry wraps an orchestrator in the IOrchestrationEntry adapter.
func NewEntry(o *SessionOrchestrator) *Entry {
	return &Entry{SessionOrchestrator: o}
}

// ProcessMessage implements contracts.IOrchestrationEntry.
func (e *Entry) ProcessMessage(ctx context.Context, sessionID, message string) (<-chan *contracts.EngineEvent, error) {
	return e.SessionOrchestrator.ProcessMessage(ctx, ProcessRequest{SessionID: sessionID, Message: message})
}

// Cancel implements contracts.IOrchestrationEntry. It is the gateway's
// StopProcess / HandleInterrupt entry point. The InterruptHandler runs the
// full Wave→D4→Process sequence plus the stopped event (per R2 命题 E).
func (e *Entry) Cancel(ctx context.Context, sessionID string) error {
	if e.interruptHandler == nil {
		// Lazy-construct a minimal interrupt handler if the gateway calls
		// Cancel without going through bootstrap wiring.
		e.interruptHandler = NewInterruptHandler(e.SessionOrchestrator, InterruptOptions{})
	}
	return e.interruptHandler.Handle(ctx, sessionID)
}

// SetInterruptHandler lets bootstrap install a fully-wired interrupt
// handler (with Wave/D4 cancelers and sink).
func (o *SessionOrchestrator) SetInterruptHandler(h *InterruptHandler) {
	o.interruptHandler = h
}

// registerInterrupt installs a cancel func keyed by sessionID. The same
// session can only have one active orchestrator process.
func (o *SessionOrchestrator) registerInterrupt(sessionID string, cancel context.CancelFunc) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if prev, ok := o.activeSessions[sessionID]; ok {
		// Best-effort cancel the previous run; idempotent.
		prev()
	}
	o.activeSessions[sessionID] = cancel
}

func (o *SessionOrchestrator) unregisterInterrupt(sessionID string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.activeSessions, sessionID)
}
