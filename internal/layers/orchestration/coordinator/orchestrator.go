package coordinator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// SessionOrchestrator is the D7-S2 entry point that replaces D1→D2.Process.
//
// D1 RouteInbound (when d7_enabled=true) calls ProcessMessage. The
// orchestrator:
//  1. Classifies the intent (rule-only in v1.0).
//  2. Routes to one of 4 real execution paths per the routing matrix:
//     - IntentSkip        → close channel
//     - IntentCommand     → CommandHandler (zero LLM, plan/task CLI)
//     - IntentFast        → FastPath.Run (D2 single-turn LLM↔Tool loop)
//     - IntentOrchestrate → OrchestratePath (SynthesizeTaskGraph → Wave)
//  3. Handles interrupts (HandleInterrupt) for /stop and D1 Stop.
//
// v1.1.0: each IntentKind has its own execution chain (orthogonal paths).
// v1.0 had Command/Orchestrate collapsed to FastPath with system-prompt
// hints; that v1.0 simplification is removed.
//
// See d7-domain.md §Orchestration Routing Matrix.
type SessionOrchestrator struct {
	cfg              *Config
	classifier       IntentClassifier
	executor         QueryLoopExecutor
	fastPath         *FastPath
	workModel        WorkModel
	validator        AdvisoryValidator
	sink             EventPublisher
	validationMetric *ValidationMetrics
	obsBridge        *observability.Bridge
	// shadowClassifier wraps classifier (when wired) with an async LLM
	// shadow on the IntentOrchestrate tail. nil → behavior unchanged.
	shadowClassifier *ShadowClassifier

	// v1.1.0+ orthogonal paths
	commandHandler  *CommandHandler
	orchestratePath *OrchestratePath
	turnToolExec    *TurnToolExecutor

	// llmDecomposer is the D7-S5-A03 LLM-augmented task synthesizer.
	// When non-nil, the default OrchestratePath's TaskDecomposer uses it
	// before falling back to rule-based decomposition.
	llmDecomposer LLMTaskDecomposer

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

// WithSink wires a communication event publisher.
func WithSink(s EventPublisher) OrchestratorOption {
	return func(o *SessionOrchestrator) { o.sink = s }
}

// WithValidator wires the optional advisory validator.
func WithValidator(v AdvisoryValidator) OrchestratorOption {
	return func(o *SessionOrchestrator) { o.validator = v }
}

// WithWorkModel wires a custom WorkModel. v1.0 uses NewDelegatedWorkModel()
// which forwards to D2 TaskManager.
func WithWorkModel(w WorkModel) OrchestratorOption {
	return func(o *SessionOrchestrator) { o.workModel = w }
}

// WithMetrics wires the validation metrics sink. The metrics record
// the 4 outcomes (pass / fail / timeout / error) and a sliding-window
// timeout_rate; nil metric is treated as no-op.
func WithMetrics(m *ValidationMetrics) OrchestratorOption {
	return func(o *SessionOrchestrator) { o.validationMetric = m }
}

// WithShadowClassifier wires the optional LLM classify shadow. The
// shadow runs asynchronously on the IntentOrchestrate tail (~20% of
// messages the rule does not fast-match); v1.0 decision path is the
// rule + command-first matrix, so the shadow never affects the
// ProcessMessage return value. See R2 §5 命题 C.
func WithShadowClassifier(s *ShadowClassifier) OrchestratorOption {
	return func(o *SessionOrchestrator) { o.shadowClassifier = s }
}

// WithCommandHandler wires the IntentCommand explicit-dispatch path.
// v1.1.0+ orthogonal dispatch (devrix-d7-orthogonal-intent-paths).
func WithCommandHandler(h *CommandHandler) OrchestratorOption {
	return func(o *SessionOrchestrator) { o.commandHandler = h }
}

// WithOrchestratePath wires the IntentOrchestrate explicit-orchestration
// path. v1.1.0+ orthogonal dispatch.
func WithOrchestratePath(p *OrchestratePath) OrchestratorOption {
	return func(o *SessionOrchestrator) { o.orchestratePath = p }
}

// WithTurnToolExecutor wires the loop-first turn tool executor for
// delegate_wave / enter_plan_mode. SetOrchestratePath updates its path.
func WithTurnToolExecutor(e *TurnToolExecutor) OrchestratorOption {
	return func(o *SessionOrchestrator) { o.turnToolExec = e }
}

// WithLLMDecomposer wires an LLM-augmented task synthesizer into the
// default OrchestratePath. When WithOrchestratePath is also used, that
// path takes precedence and the LLM decomposer is ignored.
//
// D7-S5-A03: with this option wired, SynthesizeTaskGraph first asks the
// LLM to produce a JSON task DAG; if the LLM call fails, returns no
// JSON, or yields invalid task nodes, the rule-based fallback runs.
func WithLLMDecomposer(d LLMTaskDecomposer) OrchestratorOption {
	return func(o *SessionOrchestrator) { o.llmDecomposer = d }
}

// WithClassifier replaces the default RuleClassifier. The default is
// NewRuleClassifier(cfg) (rule-only). Tests use this to inject stubs;
// v1.1+ may inject a LLM-first classifier that satisfies
// IntentClassifier directly.
//
// Invariant: the option must be applied before any ProcessMessage
// call. Re-invoking replaces the active classifier; if a ShadowClassifier
// is also wired, WithShadowClassifier takes precedence at the call site
// (ProcessMessage checks shadow first).
func WithClassifier(c IntentClassifier) OrchestratorOption {
	return func(o *SessionOrchestrator) { o.classifier = c }
}

// NewSessionOrchestrator builds the orchestrator with the given
// query-loop executor and options. The classifier defaults to
// NewRuleClassifier(cfg) (rule-only) but can be replaced via
// WithClassifier (tests, LLM-first v1.1+).
//
// v1.1.0+ orthogonal paths: if WithCommandHandler / WithOrchestratePath
// are not provided, defaults are constructed (CommandHandler bound to
// workmodel.GlobalTaskManager + a fresh PlanMode; OrchestratePath bound
// to a fresh TaskDecomposer + fresh WaveScheduler). Tests that want
// control over the wave scheduler or the plan mode should still wire
// the options explicitly. Bootstrap does NOT need to wire these in
// production — defaults are sufficient for d7_enabled=true.
//
// Options are applied in order. WithSink and WithValidator must be passed
// before the orchestrator constructs the FastPath (i.e. via the returned
// instance), so the FastPath is built lazily on first ProcessMessage.
// This avoids the bug where WithSink would have been ignored because the
// FastPath was constructed before the option was applied.
func NewSessionOrchestrator(cfg *Config, executor QueryLoopExecutor, opts ...OrchestratorOption) *SessionOrchestrator {
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

	// v1.1.0+ orthogonal paths: provide lazy defaults so callers (and
	// tests) can rely on the 4-way switch without explicitly wiring
	// the new options. Production code may still wire explicitly via
	// WithCommandHandler / WithOrchestratePath to inject a real
	// WaveScheduler or a custom PlanMode.
	if o.commandHandler == nil {
		o.commandHandler = newDefaultCommandHandler(o.workModel, o.sink)
	}
	if o.orchestratePath == nil {
		o.orchestratePath = newDefaultOrchestratePath(o.sink, o.llmDecomposer)
	}
	if o.orchestratePath != nil {
		o.orchestratePath.SetObsBridge(o.obsBridge)
	}
	return o
}

// ProcessMessage is the D1→D7 entry point.
//
// Routing (v1.1.0+ orthogonal dispatch, see
// devrix-d7-orthogonal-intent-paths):
//   - skip        → return empty channel (inlined, no executor)
//   - command     → CommandHandler.Handle (D7-internal CLI, zero LLM)
//   - fast        → FastPath.Run (D2 single-turn LLM↔Tool loop)
//   - orchestrate → OrchestratePath.Run (SynthesizeTaskGraph → Wave)
//
// Each IntentKind maps to an independent execution chain. v1.0 closure
// had Command/Orchestrate collapsed to FastPath with system-prompt hints;
// that v1.0 simplification is removed (see design.md §2.5).
func (o *SessionOrchestrator) ProcessMessage(ctx context.Context, req ProcessRequest) (<-chan *contracts.EngineEvent, error) {
	ctx, sessionSpan := o.startSpan(ctx, telemetry.OpD7_S2_Orchestration_Session_Process, tracer.SpanKindInternal,
		tracer.Attribute{Key: "session_id", Value: req.SessionID},
		tracer.Attribute{Key: "message.len", Value: fmt.Sprintf("%d", len(req.Message))},
	)
	// Keep sessionCtx for downstream routing. classifySpan mutates ctx; using
	// that ctx after End() would attach Turn/Wave spans to the ended classify
	// span instead of Session_Process (registry: Turn is sibling of Classify).
	sessionCtx := ctx

	classifySource := "rule"
	_, classifySpan := o.startSpan(sessionCtx, telemetry.OpD7_S2_Orchestration_Intent_Classify, tracer.SpanKindInternal)
	var (
		intent IntentClassification
		err    error
	)
	if o.shadowClassifier != nil {
		classifySource = "shadow"
		intent, err = o.shadowClassifier.Classify(ctx, req.Message)
	} else {
		intent, err = o.classifier.Classify(ctx, req.Message)
	}
	if classifySpan != nil {
		classifySpan.SetAttributes(telemetry.SpanAttrs(telemetry.OpD7_S2_Orchestration_Intent_Classify,
			intentClassifyAttrs(intent, classifySource)...)...)
		if err != nil {
			classifySpan.RecordError(err)
			classifySpan.SetStatus(tracer.StatusCodeError, err.Error())
		} else {
			classifySpan.SetStatus(tracer.StatusCodeOk, "")
		}
		classifySpan.End()
	}
	if err != nil {
		endSpanWithError(sessionSpan, err)
		return nil, fmt.Errorf("orchestrator: classify: %w", err)
	}
	if sessionSpan != nil {
		sessionSpan.SetAttributes(tracer.Attribute{Key: "orchestration.route", Value: routeLabel(intent)})
	}

	// Advisory validation; outcome is observed (per R2 §5 P1 #6) but
	// does not block the request — pass/fail is informational, timeout
	// and panic are surfaced via the 4-counter + alert hook.
	o.callAdvisoryValidator(sessionCtx, intent, req.SessionID)

	// D7-S5-A01-T01: FastPath confidence threshold gating (rule_orchestrate only).
	if !o.cfg.IsLoopFirst() && intent.Kind == IntentFast && intent.Confidence < o.cfg.FastPathThreshold {
		intent = IntentClassification{
			Kind:       IntentOrchestrate,
			Confidence: intent.Confidence,
			Reason:     fmt.Sprintf("fast confidence %d < threshold %d: %s", intent.Confidence, o.cfg.FastPathThreshold, intent.Reason),
			Command:    intent.Command,
		}
	}

	var ch <-chan *contracts.EngineEvent
	switch intent.Kind {
	case IntentSkip:
		skipCh := make(chan *contracts.EngineEvent)
		close(skipCh)
		ch = skipCh
	case IntentCommand:
		if o.commandHandler == nil {
			endSpanWithError(sessionSpan, fmt.Errorf("orchestrator: IntentCommand received but commandHandler is nil (bootstrap missing wiring)"))
			return nil, fmt.Errorf("orchestrator: IntentCommand received but commandHandler is nil (bootstrap missing wiring)")
		}
		ch, err = o.commandHandler.Handle(sessionCtx, req, intent)
	case IntentFast:
		ch, err = o.fastPath.Run(sessionCtx, req, turnSystemPrompt(o.cfg, ""))
	case IntentOrchestrate:
		if o.orchestratePath == nil {
			endSpanWithError(sessionSpan, fmt.Errorf("orchestrator: IntentOrchestrate received but orchestratePath is nil (bootstrap missing wiring)"))
			return nil, fmt.Errorf("orchestrator: IntentOrchestrate received but orchestratePath is nil (bootstrap missing wiring)")
		}
		ch, err = o.orchestratePath.Run(sessionCtx, req, intent)
	default:
		err = fmt.Errorf("orchestrator: unknown intent kind %q", intent.Kind)
	}
	if err != nil {
		endSpanWithError(sessionSpan, err)
		return nil, err
	}
	if sessionSpan != nil {
		sessionSpan.SetStatus(tracer.StatusCodeOk, "")
	}
	return endSpanWhenChannelClosed(ch, sessionSpan), nil
}

// callAdvisoryValidator invokes the optional advisory validator, times
// the call, and dispatches the outcome to the validation metrics sink.
//
// Outcomes:
//   - elapsed > 2*timeout → error counter (gross fault)
//   - elapsed > timeout   → timeout counter (advisory pass per contract)
//   - result.Pass         → pass counter
//   - else                → fail counter
//
// A panic inside the validator is recovered and recorded as error.
// If validator or validationMetric is nil, the call is a no-op
// (backward compatible with the v1.0 pre-metric orchestrator).
func (o *SessionOrchestrator) callAdvisoryValidator(ctx context.Context, intent IntentClassification, sessionID string) {
	if o.validator == nil {
		return
	}
	timeoutMs := o.cfg.AdvisoryValidationTimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = 50
	}
	timeout := durationOrDefault(timeoutMs)
	vctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	var (
		result ValidationResult
	)
	func() {
		defer func() {
			if r := recover(); r != nil {
				if o.validationMetric != nil {
					o.validationMetric.RecordError(start.Add(time.Since(start)))
				}
			}
		}()
		result = o.validator.ValidateOrchestration(vctx, OrchestrationDecision{
			Intent:    intent,
			SessionID: sessionID,
		})
	}()
	elapsed := time.Since(start)

	if o.validationMetric == nil {
		return
	}
	switch {
	case elapsed > 2*timeout:
		o.validationMetric.RecordError(start.Add(elapsed))
	case elapsed > timeout:
		o.validationMetric.RecordTimeout(start.Add(elapsed))
	case result.Pass:
		o.validationMetric.RecordPass(start.Add(elapsed))
	default:
		o.validationMetric.RecordFail(start.Add(elapsed))
	}
}

// handleCommand and orchestrate were removed in v1.1.0 (devrix-d7-orthogonal-intent-paths).
// Their old behavior (FastPath.Run with a system-prompt hint) is replaced
// by CommandHandler.Handle and OrchestratePath.Run respectively. The two
// switch cases in ProcessMessage now call those independent paths directly.

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

// SetOrchestratePath replaces the OrchestratePath. v1.1.0+ orthogonal
// dispatch: by default NewSessionOrchestrator installs a lazy
// OrchestratePath bound to a zero-deps WaveScheduler (which is unsafe
// for production). Production bootstrap and integration tests use this
// setter to inject a fully-wired or fake scheduler.
func (o *SessionOrchestrator) SetOrchestratePath(p *OrchestratePath) {
	o.orchestratePath = p
	if o.turnToolExec != nil {
		o.turnToolExec.Orchestrate = p
	}
}

// SetCommandHandler replaces the CommandHandler. Same rationale as
// SetOrchestratePath — primarily a test seam so integration tests can
// verify the orthogonal dispatch without depending on the lazy default
// binding to workmodel.GlobalTaskManager.
func (o *SessionOrchestrator) SetCommandHandler(h *CommandHandler) {
	o.commandHandler = h
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
