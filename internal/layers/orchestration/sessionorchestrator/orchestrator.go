package sessionorchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/decisionplanning"
	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"

	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/layers/orchestration/escape"
	"github.com/devrix/devrix/internal/layers/orchestration/hardening"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// SessionOrchestrator is the D7-S2 entry point that replaces D1→D2.Process.
//
// D1 RouteInbound (when d7_enabled=true) calls ProcessMessage. The
// orchestrator:
//  1. Classifies the intent (decisionplanning.IntentClassifier; default rule-based).
//  2. Routes per the routing matrix:
//     - orchtypes.IntentSkip                                  → close channel
//     - orchtypes.IntentCommand                               → CommandHandler (zero LLM, plan/task CLI)
//     - orchtypes.IntentFast | orchtypes.IntentOrchestrate    → RunSessionTurnLoop (WorkTree + per-WorkItem MUPS)
//  3. Handles interrupts (HandleInterrupt) for /stop and D1 Stop.
//
// User instructions (except `/` commands and skip-eligible empty messages)
// flow through RunSessionTurnLoop → ItemPipelineRunner (Observe→Plan→
// Execute→Verify→Learn→Decide) backed by WorkTree.
//
// See d7-domain.md §Orchestration Routing Matrix.
type SessionOrchestrator struct {
	cfg              *orchtypes.Config
	classifier decisionplanning.IntentClassifier
	workModel  WorkModel
	validator        AdvisoryValidator
	sink             EventPublisher
	validationMetric *ValidationMetrics
	obsBridge        *observability.Bridge

	commandHandler *CommandHandler
	turnToolExec   *TurnToolExecutor

	// taskManager is the D7 task store injected at construction (DM-20260617-008 W4).
	// nil → a fresh in-memory taskmanager is created in NewSessionOrchestrator.
	taskManager *workmodel.TaskManager

	// turnState (DM-20260628-003, D7-S15) serializes turns per session_id
	// so a second ProcessMessage call for the same session waits for the
	// first one's RunSessionTurnLoop goroutine to drain `out`. nil →
	// disabled (legacy/test path; equivalent to pre-D7-S15 behavior).
	turnState *TurnState

	// transcriptReader (DM-20260628-003, D7-S15) reads the recent
	// complete-event Bodies from a session's transcript jsonl so the
	// next turn's directive can be enriched with <prior-output-summary>.
	// nil → no injection.
	transcriptReader *TranscriptReader

	// priorContextRounds (DM-20260628-003, D7-S15) controls how many
	// recent turns' finalText to inject. 0 disables injection (default,
	// backward compatible). >0 reads transcript + builds summary block.
	priorContextRounds int

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

	// itemPipeline runs per-WorkItem MUPS (required for user messages).
	itemPipeline *ItemPipelineRunner

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

// WithTurnToolExecutor wires the loop-first turn tool executor for
// enter_plan_mode and D2 tool delegation inside WorkItemExecutor.
func WithTurnToolExecutor(e *TurnToolExecutor) OrchestratorOption {
	return func(o *SessionOrchestrator) { o.turnToolExec = e }
}

// WithItemPipelineRunner wires per-WorkItem MUPS pipeline (required in production).
func WithItemPipelineRunner(r *ItemPipelineRunner) OrchestratorOption {
	return func(o *SessionOrchestrator) { o.itemPipeline = r }
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

// WithTurnState injects a pre-built TurnState (typically used by tests
// that want to share state across orchestrator instances, or by
// bootstrap to share state across orchestrator rebuilds on config
// reload). Production code should prefer WithPriorContextRounds which
// constructs an isolated TurnState automatically.
func WithTurnState(ts *TurnState) OrchestratorOption {
	return func(o *SessionOrchestrator) { o.turnState = ts }
}

// WithTranscriptDir overrides the default transcript directory used by
// the embedded TranscriptReader. Empty string → default
// ~/.devrix/transcripts (matching bootstrap.NewTranscriptWriter).
// No-op unless WithPriorContextRounds(n>0) is also applied.
func WithTranscriptDir(dir string) OrchestratorOption {
	return func(o *SessionOrchestrator) { o.transcriptReader = NewTranscriptReader(dir) }
}

// WithPriorContextRounds enables prior-output-summary injection into
// each turn's directive.
//
// n <= 0 (default) → injection disabled; turnState and transcriptReader
// are NOT constructed. Equivalent to pre-D7-S15 behavior.
//
// n > 0 → construct a fresh TurnState + TranscriptReader on this
// orchestrator. Subsequent calls to WithTurnState or WithTranscriptDir
// can override the freshly built instances (applied later wins because
// functional options run in caller order).
func WithPriorContextRounds(n int) OrchestratorOption {
	return func(o *SessionOrchestrator) {
		o.priorContextRounds = n
		if n > 0 {
			if o.turnState == nil {
				o.turnState = NewTurnState()
			}
			if o.transcriptReader == nil {
				o.transcriptReader = NewTranscriptReader("")
			}
		}
	}
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

// NewSessionOrchestrator builds the orchestrator with options. The
// classifier defaults to decisionplanning.NewRuleClassifier(cfg) (rule-only)
// but can be replaced via WithClassifier (tests, LLM-first v1.1+).
func NewSessionOrchestrator(cfg *orchtypes.Config, _ TurnExecutor, opts ...OrchestratorOption) *SessionOrchestrator {
	if cfg == nil {
		cfg = orchtypes.DefaultConfig()
	}
	o := &SessionOrchestrator{
		cfg:            cfg,
		classifier:     decisionplanning.NewRuleClassifier(cfg),
		activeSessions: make(map[string]context.CancelFunc),
	}
	for _, opt := range opts {
		opt(o)
	}
	if o.taskManager == nil {
		o.taskManager = workmodel.NewTaskManager()
	}
	if o.commandHandler == nil {
		o.commandHandler = newDefaultCommandHandler(o.workModel, o.sink, o.taskManager)
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
			// DM-20260629-001 PR-6 t-span-coverage (T41): emit the
			// D7_AdaptivePrior_Inject span so LP-1 closure traces show the
			// actual Beta(alpha, beta) being injected at the Observe
			// boundary. The span fires only on the success path
			// (cold-start Bootstrap / Reputation fallback path emits no
			// span — they fall back to DefaultDeveloperPrior and the
			// caller already sees the warning slog).
			trackMode := req.TrackMode
			if trackMode == "" {
				trackMode = string(learn.TrackModeDeveloper)
			}
			endInject := hardening.EmitAdaptivePriorInject(
				ctx,
				req.SessionID,
				"Observe",
				float64(prior.PriorBeta.Alpha),
				float64(prior.PriorBeta.Beta),
			)
			endInject(nil)
			_ = trackMode // reserved for follow-up prior.source_track_mode attr
		}
	}
	return orchtypes.NewObserveRequest(req.SessionID, req.Message, prior)
}

// Routing:
//   - skip        → return empty channel (inlined, no executor)
//   - command     → CommandHandler.Handle (D7-internal CLI, zero LLM)
//   - fast|orchestrate → RunSessionTurnLoop (WorkTree + per-WorkItem MUPS)
//
// Phase 6 PR-F2 (D7-S12-A42-T05): at entry, buildObserveRequest calls
// Learner.Inject (when wired) to obtain an AdaptivePrior. The prior
// is threaded into intent classification via ClassifyWithPrior (LP-1
// closed loop). When learner is nil, prior defaults to
// DefaultDeveloperPrior (Beta(5,3)) and ClassifyWithPrior degenerates
// to a baseline-equivalent path.
func (o *SessionOrchestrator) ProcessMessage(ctx context.Context, req orchtypes.ProcessRequest) (<-chan *contracts.EngineEvent, error) {
	if req.UserID == "" {
		req.UserID = effectiveUserID(ctx, req)
	}
	ctx, sessionSpan := o.startSpan(ctx, telemetry.OpD7_S2_Orchestration_Session_Process, tracer.SpanKindInternal,
		tracer.Attribute{Key: "session_id", Value: req.SessionID},
		tracer.Attribute{Key: "message.len", Value: fmt.Sprintf("%d", len(req.Message))},
		tracer.Attribute{Key: "context.caller", Value: "d7"},
	)
	// Keep sessionCtx for downstream routing. classifySpan mutates ctx; using
	// that ctx after End() would attach Turn/Wave spans to the ended classify
	// span instead of Session_Process (registry: Turn is sibling of Classify).
	sessionCtx := ctx

	// DM-20260628-003 (D7-S15): wait for any in-flight turn on this session
	// to fully drain its out channel. This is the architectural fix for the
	// 2026-06-28 17:31 panic (sess_1782638991113_5000) where two goroutines
	// for the same session_id raced on executor.Emit. PR #271 added the
	// defensive recover + per-Run overwrite; this WaitTurn gate ensures the
	// race cannot happen in the first place.
	//
	// nil turnState (legacy/test path) → no wait, equivalent to v6.0.0.
	if o.turnState != nil {
		if err := o.turnState.WaitTurn(sessionCtx, req.SessionID); err != nil {
			endSpanWithError(sessionSpan, err)
			return nil, fmt.Errorf("orchestrator: wait turn: %w", err)
		}
	}

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
		// DM-20260628-003 (D7-S15): enrich directive with prior-output-summary
		// so turn N+1's LLM sees turn N's finalText. Read failures are
		// non-fatal — a missing transcript just means fresh session / no
		// history; we fall through with req.Message as-is.
		if o.priorContextRounds > 0 && o.transcriptReader != nil {
			texts, rerr := o.transcriptReader.ReadRecent(sessionCtx, req.SessionID, o.priorContextRounds)
			if rerr != nil {
				slog.Warn("orchestrator: transcript reader failed, skipping prior context injection",
					"session_id", req.SessionID, "err", rerr)
			} else if len(texts) > 0 {
				summary := o.transcriptReader.BuildPriorOutputSummary(texts)
				enriched := summary + "\n\n" + req.Message
				_, _ = o.taskManager.Tree().EnsureGoal(req.SessionID, enriched)
			}
		}
	}

	// Advisory validation; outcome is observed (per R2 §5 P1 #6) but
	// does not block the request — pass/fail is informational, timeout
	// and panic are surfaced via the 4-counter + alert hook.
	o.callAdvisoryValidator(sessionCtx, intent, req.SessionID)

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
		// `/`-prefixed D7-internal commands stay on CommandHandler (zero-LLM,
		// no 5-node overhead). All other user instructions — regardless of
		// classifier confidence or previous Fast/Orchestrate classification —
		// All other user instructions route through RunSessionTurnLoop
		// (WorkTree focus loop + per-WorkItem MUPS: Observe → Plan →
		// Execute → Verify → Learn). See ProcessMessage doc comment above.
		if o.commandHandler == nil {
			err = fmt.Errorf("orchestrator: orchtypes.IntentCommand received but commandHandler is nil (bootstrap missing wiring)")
		} else {
			ch, err = o.commandHandler.Handle(sessionCtx, req, intent)
		}
	case orchtypes.IntentFast, orchtypes.IntentOrchestrate:
		if o.itemPipeline == nil {
			err = fmt.Errorf("orchestrator: item pipeline not wired (WithItemPipelineRunner)")
		} else {
			ch, err = o.RunSessionTurnLoop(sessionCtx, req, intent)
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
// CommandHandler.Handle and RunSessionTurnLoop replaced the old FastPath /
// OrchestratePath ingress paths.

// ProcessMessageContract satisfies contracts.IOrchestrationEntry. It is the
// D1 gateway-facing seam (string args instead of orchtypes.ProcessRequest).
func (o *SessionOrchestrator) ProcessMessageContract(ctx context.Context, sessionID, message string) (<-chan *contracts.EngineEvent, error) {
	return o.ProcessMessage(ctx, orchtypes.ProcessRequest{
		SessionID: sessionID,
		Message:   message,
		UserID:    effectiveUserID(ctx, orchtypes.ProcessRequest{SessionID: sessionID, Message: message}),
	})
}

// TaskManager returns the wired work-item store (nil if not configured).
func (o *SessionOrchestrator) TaskManager() *workmodel.TaskManager {
	if o == nil {
		return nil
	}
	return o.taskManager
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
	return e.SessionOrchestrator.ProcessMessage(ctx, orchtypes.ProcessRequest{
		SessionID: sessionID,
		Message:   message,
		UserID:    effectiveUserID(ctx, orchtypes.ProcessRequest{SessionID: sessionID, Message: message}),
	})
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

// SetCommandHandler replaces the CommandHandler.
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
