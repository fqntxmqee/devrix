package sessionorchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/decisionplanning"
	"github.com/devrix/devrix/internal/layers/orchestration/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"

	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/layers/orchestration/escape"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// SessionOrchestrator is the D7-S2 entry point that replaces D1→D2.Process.
//
// D1 RouteInbound (when d7_enabled=true) calls ProcessMessage. The
// orchestrator:
//  1. Classifies the intent (decisionplanning.IntentClassifier; default rule-based).
//  2. Routes to one of 4 real execution paths per the routing matrix:
//     - orchtypes.IntentSkip        → close channel
//     - orchtypes.IntentCommand     → CommandHandler (zero LLM, plan/task CLI)
//     - orchtypes.IntentFast        → FastPath.Run (D2 single-turn LLM↔Tool loop)
//     - orchtypes.IntentOrchestrate → OrchestratePath (SynthesizeTaskGraph → Wave)
//  3. Handles interrupts (HandleInterrupt) for /stop and D1 Stop.
//
// Each orchtypes.IntentKind has its own execution chain (orthogonal paths).
//
// See d7-domain.md §Orchestration Routing Matrix.
type SessionOrchestrator struct {
	cfg              *orchtypes.Config
	classifier       decisionplanning.IntentClassifier
	executor         TurnExecutor
	fastPath         *FastPath
	workModel        WorkModel
	validator        AdvisoryValidator
	sink             EventPublisher
	validationMetric *ValidationMetrics
	obsBridge        *observability.Bridge

	// v1.1.0+ orthogonal paths
	commandHandler  *CommandHandler
	orchestratePath *OrchestratePath
	turnToolExec    *TurnToolExecutor

	// taskManager is the D7 task store injected at construction (DM-20260617-008 W4).
	// nil → a fresh in-memory taskmanager is created in NewSessionOrchestrator.
	taskManager *workmodel.TaskManager

	// learner is the Phase 6 PR-F2 (D7-S12-A42-T04) LP-1 closed-loop
	// component. When non-nil, ProcessMessage calls learner.Inject at
	// entry to obtain the AdaptivePrior for the next Observe call.
	// nil → no prior injection (ObserveRequest.EffectivePrior returns
	// DefaultDeveloperPrior as fail-safe).
	learner learn.Learner

	// escapeEngine is the MUPS v5 (DM-20260625-003, PR-V5.5) 5-node
	// escape evaluator. nil → no escape evaluation (V5.4- behavior).
	// When wired, ProcessMessage invokes Evaluate at 3-4 wiring points
	// (1a/1b/2/3) to enforce the unified depth-limit / circuit-breaker /
	// PlanKind-switch policies. See escape.Engine for details.
	escapeEngine *escape.EscapeEngine

	// llmDecomposer is the D7-S5-A03 LLM-augmented task synthesizer.
	// When non-nil, the default OrchestratePath's decisionplanning.TaskDecomposer uses it
	// before falling back to rule-based decomposition.
	llmDecomposer decisionplanning.LLMTaskDecomposer

	// activeSessions tracks the running orchtypes.ProcessRequest per sessionID so
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

// WithWorkModel wires a custom WorkModel. v1.1 defaults to nil (D7-S1
// workmodel.TaskManager is the canonical storage; the WorkModel facade is
// only needed when callers want to query the unified work plan).
func WithWorkModel(w WorkModel) OrchestratorOption {
	return func(o *SessionOrchestrator) { o.workModel = w }
}

// WithMetrics wires the validation metrics sink. The metrics record
// the 4 outcomes (pass / fail / timeout / error) and a sliding-window
// timeout_rate; nil metric is treated as no-op.
func WithMetrics(m *ValidationMetrics) OrchestratorOption {
	return func(o *SessionOrchestrator) { o.validationMetric = m }
}

// WithCommandHandler wires the orchtypes.IntentCommand explicit-dispatch path.
// v1.1.0+ orthogonal dispatch (devrix-d7-orthogonal-intent-paths).
func WithCommandHandler(h *CommandHandler) OrchestratorOption {
	return func(o *SessionOrchestrator) { o.commandHandler = h }
}

// WithOrchestratePath wires the orchtypes.IntentOrchestrate explicit-orchestration
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
func WithLLMDecomposer(d decisionplanning.LLMTaskDecomposer) OrchestratorOption {
	return func(o *SessionOrchestrator) { o.llmDecomposer = d }
}

// WithTaskManager wires the *workmodel.TaskManager that backs /task CLI
// commands and the default CommandHandler. When this option is omitted,
// NewSessionOrchestrator creates a fresh in-memory TaskManager.
func WithTaskManager(tm *workmodel.TaskManager) OrchestratorOption {
	return func(o *SessionOrchestrator) { o.taskManager = tm }
}

// WithLearner wires the Phase 5 LP-1 closed-loop Learner. When wired,
// ProcessMessage calls o.learner.Inject(ctx, req.SessionID) at entry
// to obtain an AdaptivePrior and threads it into intent classification
// via ClassifyWithPrior.
//
// Phase 6 PR-F2 (D7-S12-A42-T04). When omitted, no prior injection
// happens (ObserveRequest.EffectivePrior returns DefaultDeveloperPrior
// as fail-safe). Pass nil explicitly to remove a previously-wired
// learner.
func WithLearner(l learn.Learner) OrchestratorOption {
	return func(o *SessionOrchestrator) { o.learner = l }
}

// WithEscapeEngine wires the MUPS v5 (DM-20260625-003) EscapeEngine.
//
// PR-V5.5 wiring points (5-node pipeline):
//   - 1a: Plan fails (classifier error) → Evaluate → may ForceExit
//   - 1b: Plan前 (before dispatch)      → Evaluate → may ForceExit
//   - 2:  Execute fails (path error)    → Evaluate → may ForceExit
//   - 3:  Verify fails (FAIL/INDET)     → Evaluate → may ForceExit
//
// nil → no escape evaluation (V5.4- behavior, backward compatible).
// Pass nil explicitly to remove a previously-wired engine.
func WithEscapeEngine(e *escape.EscapeEngine) OrchestratorOption {
	return func(o *SessionOrchestrator) { o.escapeEngine = e }
}

// WithClassifier replaces the default decisionplanning.RuleClassifier. The default is
// decisionplanning.NewRuleClassifier(cfg) (rule-only). Tests use this to inject stubs;
// v1.1+ may inject a LLM-first classifier that satisfies
// decisionplanning.IntentClassifier directly.
//
// Invariant: the option must be applied before any ProcessMessage
// call. Re-invoking replaces the active classifier.
func WithClassifier(c decisionplanning.IntentClassifier) OrchestratorOption {
	return func(o *SessionOrchestrator) { o.classifier = c }
}

// NewSessionOrchestrator builds the orchestrator with the given
// query-loop executor and options. The classifier defaults to
// decisionplanning.NewRuleClassifier(cfg) (rule-only) but can be replaced via
// WithClassifier (tests, LLM-first v1.1+).
//
// v1.1.0+ orthogonal paths: if WithCommandHandler / WithOrchestratePath
// are not provided, defaults are constructed (CommandHandler bound to
// workmodel.TaskManager + a fresh PlanMode; OrchestratePath bound
// to a fresh decisionplanning.TaskDecomposer + fresh WaveScheduler). Tests that want
// control over the wave scheduler or the plan mode should still wire
// the options explicitly. Bootstrap does NOT need to wire these in
// production — defaults are sufficient for d7_enabled=true.
//
// Options are applied in order. WithSink and WithValidator must be passed
// before the orchestrator constructs the FastPath (i.e. via the returned
// instance), so the FastPath is built lazily on first ProcessMessage.
// This avoids the bug where WithSink would have been ignored because the
// FastPath was constructed before the option was applied.
func NewSessionOrchestrator(cfg *orchtypes.Config, executor TurnExecutor, opts ...OrchestratorOption) *SessionOrchestrator {
	if cfg == nil {
		cfg = orchtypes.DefaultConfig()
	}
	o := &SessionOrchestrator{
		cfg:            cfg,
		classifier:     decisionplanning.NewRuleClassifier(cfg),
		executor:       executor,
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
	if o.taskManager == nil {
		o.taskManager = workmodel.NewTaskManager()
	}
	if o.commandHandler == nil {
		o.commandHandler = newDefaultCommandHandler(o.workModel, o.sink, o.taskManager)
	}
	if o.orchestratePath == nil {
		o.orchestratePath = newDefaultOrchestratePath(o.sink, o.llmDecomposer)
	}
	if o.orchestratePath != nil {
		o.orchestratePath.SetObsBridge(o.obsBridge)
	}
	return o
}

// buildObserveRequest constructs the ObserveRequest for the next Observe
// call, with AdaptivePrior injection from Learner.Inject. This is the
// Phase 6 PR-F2 (D7-S12-A42-T05) LP-1 closed-loop entry point.
//
// Phase 7 PR-7.2 (D7-S13-A48-T05): plumbs req.TrackMode into Learner.Inject
// so the per-request hint ("developer" / "operator" / "") can select the
// prior Beta. Reputation's stored TrackMode (if any) takes precedence over
// the hint (see learn/learner.go:DefaultLearner.Inject).
//
// Failure handling (3-layer fail-safe, ordered):
//  1. learner == nil → ObserveRequest.Prior stays nil →
//     EffectivePrior() returns DefaultDeveloperPrior (Beta(5,3)).
//  2. learner != nil but Inject returns err → log warning + ObserveRequest.Prior
//     stays nil → EffectivePrior() returns DefaultDeveloperPrior.
//  3. learner != nil and Inject succeeds → ObserveRequest.Prior set to injected
//     AdaptivePrior (may still be Beta(0,0) if ReputationStore has no row).
//
// The returned error is reserved for ObserveRequest validation failures
// (e.g. empty SessionID); in normal operation, buildObserveRequest
// always returns a non-nil error only on the underlying NewObserveRequest
// validation, which is a fail-fast on the D1 gateway contract.
func (o *SessionOrchestrator) buildObserveRequest(ctx context.Context, req orchtypes.ProcessRequest) (orchtypes.ObserveRequest, error) {
	var prior *learn.AdaptivePrior
	if o.learner != nil {
		injected, err := o.learner.Inject(ctx, req.SessionID, req.TrackMode)
		if err != nil {
			slog.Warn("orchestrator: learner.Inject failed, using DefaultDeveloperPrior",
				"session_id", req.SessionID, "track_mode", req.TrackMode, "err", err)
		} else {
			prior = injected
		}
	}
	return orchtypes.NewObserveRequest(req.SessionID, req.Message, prior)
}

// Routing (v1.1.0+ orthogonal dispatch, see
// devrix-d7-orthogonal-intent-paths):
//   - skip        → return empty channel (inlined, no executor)
//   - command     → CommandHandler.Handle (D7-internal CLI, zero LLM)
//   - fast        → FastPath.Run (D2 single-turn LLM↔Tool loop)
//   - orchestrate → OrchestratePath.Run (SynthesizeTaskGraph → Wave)
//
// Each orchtypes.IntentKind maps to an independent execution chain.
//
// Phase 6 PR-F2 (D7-S12-A42-T05): at entry, buildObserveRequest calls
// Learner.Inject (when wired) to obtain an AdaptivePrior. The prior
// is threaded into intent classification via ClassifyWithPrior (LP-1
// closed loop). When learner is nil, prior defaults to
// DefaultDeveloperPrior (Beta(5,3)) and ClassifyWithPrior degenerates
// to a baseline-equivalent path.
func (o *SessionOrchestrator) ProcessMessage(ctx context.Context, req orchtypes.ProcessRequest) (<-chan *contracts.EngineEvent, error) {
	ctx, sessionSpan := o.startSpan(ctx, telemetry.OpD7_S2_Orchestration_Session_Process, tracer.SpanKindInternal,
		tracer.Attribute{Key: "session_id", Value: req.SessionID},
		tracer.Attribute{Key: "message.len", Value: fmt.Sprintf("%d", len(req.Message))},
		tracer.Attribute{Key: "context.caller", Value: "d7"},
	)
	// Keep sessionCtx for downstream routing. classifySpan mutates ctx; using
	// that ctx after End() would attach Turn/Wave spans to the ended classify
	// span instead of Session_Process (registry: Turn is sibling of Classify).
	sessionCtx := ctx

	// Phase 6 PR-F2 LP-1: build observe request with AdaptivePrior injection
	// BEFORE classification. Failures from learner.Inject are logged but do
	// not block — EffectivePrior() returns DefaultDeveloperPrior (Beta(5,3)).
	observeReq, err := o.buildObserveRequest(ctx, req)
	if err != nil {
		endSpanWithError(sessionSpan, err)
		return nil, fmt.Errorf("orchestrator: build observe request: %w", err)
	}
	prior := observeReq.EffectivePrior()

	// PR-V5.6 (DM-20260625-003, D7-S14-A50-T12): T2 ResumeSession 续跑入口.
	// 在 buildObserveRequest 之后 / classify 之前检查 PendingResolutionStore.
	// - terminal decision (B/C) → 短路返回 "complete" EngineEvent, 不走 5 节点
	// - A user_continue / 未找到 / nil engine → fall through (不破坏主链路)
	if resumeCh, shortCircuit, _ := o.applyResumeSession(ctx, req, sessionSpan); shortCircuit {
		// H-2 (DM-20260625-004 review-fixes): short-circuit path also writes prior
		// attrs so D5 trace has consistent learn.prior.{alpha,beta,mean,track_mode,
		// injected_at} for resume decisions (user_accept / user_cancel short-circuit
		// would otherwise permanently miss these attrs in Jaeger trace).
		if sessionSpan != nil {
			sessionSpan.SetAttributes(priorSessionSpanAttrs(prior, observeReq, req)...)
		}
		endSpan(sessionSpan)
		return resumeCh, nil
	}
	// Phase 7 PR-7.3 (D7-S13-A49-T06): sessionSpan carries 5 prior attributes
	// (alpha, beta, mean, track_mode, injected_at) for D5 observability.
	if sessionSpan != nil {
		sessionSpan.SetAttributes(priorSessionSpanAttrs(prior, observeReq, req)...)
	}

	_, classifySpan := o.startSpan(sessionCtx, telemetry.OpD7_S2_Orchestration_Intent_Classify, tracer.SpanKindInternal)
	var (
		intent orchtypes.IntentClassification
	)
	// Phase 6 PR-F2: ClassifyWithPrior (LP-1 closed loop) replaces
	// Classify. When prior is nil or zero-mean, ClassifyWithPrior
	// degenerates to the baseline Classify behavior.
	intent, err = o.classifier.ClassifyWithPrior(ctx, req.Message, prior)
	// Phase 7 PR-7.3 (D7-S13-A49-T06): mirror classifier_source onto the
	// sessionSpan for D5 observability. Rule classifier is the only source.
	if sessionSpan != nil {
		sessionSpan.SetAttributes(
			tracer.Attribute{Key: "learn.classifier_source", Value: "rule"},
		)
	}
	if classifySpan != nil {
		classifySpan.SetAttributes(telemetry.SpanAttrs(telemetry.OpD7_S2_Orchestration_Intent_Classify,
			intentClassifyAttrs(intent, "rule")...)...)
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
		// PR-V5.5 wiring point 1a: Plan fails (classifier error).
		// Evaluate escape decision; may ForceExit.
		if o.escapeEngine != nil {
			loopCtx := o.buildEscapeLoopContext(req.SessionID, 0, "")
			decision := o.escapeEngine.Evaluate(sessionCtx, loopCtx)
			if term, augErr := o.processEscapeDecision(decision, err); term {
				return nil, fmt.Errorf("orchestrator: classify: %w", augErr)
			}
		}
		return nil, fmt.Errorf("orchestrator: classify: %w", err)
	}
	if sessionSpan != nil {
		sessionSpan.SetAttributes(tracer.Attribute{Key: "orchestration.route", Value: routeLabel(intent)})
	}

	if o.taskManager != nil && req.SessionID != "" && strings.TrimSpace(req.Message) != "" && intent.Kind != orchtypes.IntentSkip {
		_, _ = o.taskManager.Tree().EnsureGoal(req.SessionID, req.Message)
	}

	// Advisory validation; outcome is observed (per R2 §5 P1 #6) but
	// does not block the request — pass/fail is informational, timeout
	// and panic are surfaced via the 4-counter + alert hook.
	o.callAdvisoryValidator(sessionCtx, intent, req.SessionID)

	// D7-S5-A01-T01: FastPath confidence threshold gating (rule_orchestrate only).
	if !o.cfg.IsLoopFirst() && intent.Kind == orchtypes.IntentFast && intent.Confidence < o.cfg.FastPathThreshold {
		intent = orchtypes.IntentClassification{
			Kind:       orchtypes.IntentOrchestrate,
			Confidence: intent.Confidence,
			Reason:     fmt.Sprintf("fast confidence %d < threshold %d: %s", intent.Confidence, o.cfg.FastPathThreshold, intent.Reason),
			Command:    intent.Command,
		}
	}

	// PR-V5.5 wiring point 1b: Plan前 (before dispatch).
	// Evaluate escape decision; may ForceExit.
	if o.escapeEngine != nil {
		loopCtx := o.buildEscapeLoopContext(req.SessionID, planKindFromIntent(intent.Kind), "")
		decision := o.escapeEngine.Evaluate(sessionCtx, loopCtx)
		if term, augErr := o.processEscapeDecision(decision, nil); term {
			escErr := escapeErr(decision.Reason)
			if augErr != nil {
				escErr = fmt.Errorf("%w: %w", escErr, augErr)
			}
			endSpanWithError(sessionSpan, escErr)
			return nil, escErr
		}
	}

	var ch <-chan *contracts.EngineEvent
	err = nil
	switch intent.Kind {
	case orchtypes.IntentSkip:
		skipCh := make(chan *contracts.EngineEvent)
		close(skipCh)
		ch = skipCh
	case orchtypes.IntentCommand:
		if o.commandHandler == nil {
			err = fmt.Errorf("orchestrator: orchtypes.IntentCommand received but commandHandler is nil (bootstrap missing wiring)")
		} else {
			ch, err = o.commandHandler.Handle(sessionCtx, req, intent)
		}
	case orchtypes.IntentFast:
		ch, err = o.fastPath.Run(sessionCtx, req, decisionplanning.TurnSystemPrompt(o.cfg, ""))
	case orchtypes.IntentOrchestrate:
		if o.orchestratePath == nil {
			err = fmt.Errorf("orchestrator: orchtypes.IntentOrchestrate received but orchestratePath is nil (bootstrap missing wiring)")
		} else {
			ch, err = o.orchestratePath.Run(sessionCtx, req, intent)
		}
	default:
		err = fmt.Errorf("orchestrator: unknown intent kind %q", intent.Kind)
	}

	// PR-V5.5 wiring point 2: Execute fails (path error).
	if err != nil && o.escapeEngine != nil {
		loopCtx := o.buildEscapeLoopContext(req.SessionID, planKindFromIntent(intent.Kind), err.Error())
		decision := o.escapeEngine.Evaluate(sessionCtx, loopCtx)
		if term, augErr := o.processEscapeDecision(decision, err); term {
			escErr := fmt.Errorf("orchestrator: escape_force_exit_post_execute: %w", augErr)
			endSpanWithError(sessionSpan, escErr)
			return nil, escErr
		}
	}
	if err != nil {
		endSpanWithError(sessionSpan, err)
		return nil, err
	}
	if sessionSpan != nil {
		sessionSpan.SetStatus(tracer.StatusCodeOk, "")
	}
	// Phase 7 PR-7.1 (D7-S13-A47-T01/T02): processAutoClose wraps the
	// path-returned channel so that, after the channel closes, the last
	// EngineEvent.Type is synthesized into a Verdict and learner.Learn is
	// called asynchronously. This closes the LP-1 loop in production
	// without requiring tests to manually invoke Learn. When o.learner is
	// nil, processAutoClose falls through to endSpanWhenChannelClosed (the
	// v1.0 behavior, unchanged).
	return o.processAutoClose(ch, sessionCtx, sessionSpan, req.SessionID, intent), nil
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
func (o *SessionOrchestrator) callAdvisoryValidator(ctx context.Context, intent orchtypes.IntentClassification, sessionID string) {
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
// D1 gateway-facing seam (string args instead of orchtypes.ProcessRequest).
func (o *SessionOrchestrator) ProcessMessageContract(ctx context.Context, sessionID, message string) (<-chan *contracts.EngineEvent, error) {
	return o.ProcessMessage(ctx, orchtypes.ProcessRequest{SessionID: sessionID, Message: message})
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
	return e.SessionOrchestrator.ProcessMessage(ctx, orchtypes.ProcessRequest{SessionID: sessionID, Message: message})
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
// verify the orthogonal dispatch without depending on the lazy default.
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
