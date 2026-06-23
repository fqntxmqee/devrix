package turn

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/audit"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/persist"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/token"
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/observability"
	obsruntime "github.com/devrix/devrix/internal/layers/observability/configure/runtime"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/contracts"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/textutil"
	"github.com/devrix/devrix/internal/shared/types"
)

// OrchestratorDeps holds the dependencies for the TurnOrchestrator.
type OrchestratorDeps struct {
	LLM              LLMInvoker
	Context          ContextPreparer
	Tools            ToolRoundExecutor
	Persist          SessionPersister
	// MaxTurns is an *optional safety net*, not the expected loop bound.
	// 0 / negative → unbounded: the loop only terminates on LLM natural
	// finish or one of the deterministic exit reasons below. The agent
	// matches claude-code semantics: the main conversation has no hard
	// turn limit; child agents (compact, extract-memories, etc.) set
	// their own MaxTurns based on expected workload.
	MaxTurns         int
	DefaultModel     string
	MaxContextTokens int
	ObsBridge        *observability.Bridge
	FocusHint        FocusHintProvider
	ResolveAwait     ResolveAwaiter
	// ToolResultStore persists oversized tool results to disk so they do
	// not blow up the LLM context budget (DM-20260620-001 / AC1). Nil
	// disables the cap (legacy behaviour).
	ToolResultStore *persist.ToolResultStore
	// MaxToolResultChars is the soft cap above which a tool result is
	// persisted. 0 → persist.DefaultMaxChars (12000).
	MaxToolResultChars int
	// MaxAssistantChars is the soft cap above which an assistant
	// message is folded head/tail (DM-20260620-001 / AC2). 0 →
	// persist.DefaultMaxAssistantChars (8000).
	MaxAssistantChars int
}

// ExitReason captures *why* the turn loop stopped. Surfaced on the final
// `complete` EngineEvent's Metadata["exit_reason"] and on the persisted
// turn record so SDK consumers, dashboards, and tests can distinguish a
// healthy LLM finish from a forced exit.
//
// The taxonomy mirrors claude-code's query-loop terminal reasons
// (clawcode/src/query.ts + 16-reason catalogue in
// docs/agent/clawcode/01-tools-queryloop §4). Only the subset relevant
// to devrix's D7 orchestrator is enumerated; missing reasons (e.g.
// prompt_too_long, image_error) are still surfaced via D2 enforce
// before reaching D7, so D7 does not need its own enum values for them.
type ExitReason string

const (
	// ExitReasonNatural: LLM emitted no tool calls (end_turn / stop).
	ExitReasonNatural ExitReason = "natural"
	// ExitReasonMaxTurns: MaxTurns safety net triggered. Only set when
	// MaxTurns > 0; an unbounded turn (MaxTurns ≤ 0) cannot hit this.
	ExitReasonMaxTurns ExitReason = "max_turns"
	// ExitReasonAbortedUser: ctx cancelled (user interrupt / deadline).
	ExitReasonAbortedUser ExitReason = "aborted_user"
	// ExitReasonAbortedLLM: invokeStream returned a fatal error.
	ExitReasonAbortedLLM ExitReason = "aborted_llm"
	// ExitReasonAbortedTool: ExecuteRound returned a fatal error.
	ExitReasonAbortedTool ExitReason = "aborted_tool"
	// ExitReasonRepeatedTool: same (tool_name|input) signature appeared
	// ≥ repeatedToolThreshold times in the last repeatedToolLookback
	// turns. Indicates the LLM is stuck retrying the same action.
	ExitReasonRepeatedTool ExitReason = "repeated_tool"
	// ExitReasonToolFailure: ≥ consecutiveToolErrorThreshold consecutive
	// tool errors with the same error fingerprint. Indicates the LLM
	// cannot recover from a tool failure pattern.
	ExitReasonToolFailure ExitReason = "tool_failure"
	// ExitReasonTokenDiminishing: cumulative token usage crossed the
	// 90% budget threshold AND the last two per-turn deltas were both
	// below the diminishing delta floor. Mirrors clawcode's
	// checkTokenBudget "marginal utility" stop condition.
	ExitReasonTokenDiminishing ExitReason = "token_diminishing"

	// Phase 4 PR-D2 additions (DM-20260623-002). The 8 reasons above are
	// unchanged for backward compatibility; the 6 below extend the
	// taxonomy with Verify-derived stop conditions so downstream D5
	// dashboards and Phase 5 Learn's ReputationEvidence can distinguish
	// "verifier said pass" from "verifier said partial" from "verifier
	// abstained" etc. See doc 45 §三 + doc 17/18 L1+L2 verifier.

	// ExitReasonPartialVerified: VerdictKind=Partial (some criteria met,
	// some missed — needs human review before downstream action).
	ExitReasonPartialVerified ExitReason = "partial_verified"
	// ExitReasonVerifierAbstain: VerdictKind=Indeterminate (verifier
	// could not reach a conclusion — parse failure / no consensus /
	// VerifyWithRetry exhausted 3 attempts). Requires human review.
	ExitReasonVerifierAbstain ExitReason = "verifier_abstain"
	// ExitReasonVerifierFail: VerdictKind=Fail (criteria violated — the
	// plan / artifact / evidence is rejected by the verifier).
	ExitReasonVerifierFail ExitReason = "verifier_fail"
	// ExitReasonSystemAnomaly: SystemAnomaly=true override (CatSystem
	// anomalies exceeded threshold). Forced UncertaintyCoord.Value=0.95
	// per Phase 4 PR-D4.
	ExitReasonSystemAnomaly ExitReason = "system_anomaly"
	// ExitReasonUnresolved: failure is recoverable on retry — surfaced
	// when Phase 3 Channel returns SideEffectInflight or Unknown and
	// Phase 4 Verifier cannot conclude. Distinct from VerifierFail
	// (definitive rejection) and VerifierAbstain (no signal).
	ExitReasonUnresolved ExitReason = "unresolved"
	// ExitReasonAbstain: explicit abstain (verifier decided NOT to
	// render judgement, e.g. out-of-scope plan). Distinct from
	// VerifierAbstain which is parser-level failure.
	ExitReasonAbstain ExitReason = "abstain"
)

// ParseExitReason reverses the wire format to recover the ExitReason enum
// value. Returns an error on unknown input. Phase 4 PR-D2 introduces this
// so persisted turn records and D5 dashboard filters can parse arbitrary
// reason strings without compile-time coupling.
func ParseExitReason(s string) (ExitReason, error) {
	switch s {
	case string(ExitReasonNatural):
		return ExitReasonNatural, nil
	case string(ExitReasonMaxTurns):
		return ExitReasonMaxTurns, nil
	case string(ExitReasonAbortedUser):
		return ExitReasonAbortedUser, nil
	case string(ExitReasonAbortedLLM):
		return ExitReasonAbortedLLM, nil
	case string(ExitReasonAbortedTool):
		return ExitReasonAbortedTool, nil
	case string(ExitReasonRepeatedTool):
		return ExitReasonRepeatedTool, nil
	case string(ExitReasonToolFailure):
		return ExitReasonToolFailure, nil
	case string(ExitReasonTokenDiminishing):
		return ExitReasonTokenDiminishing, nil
	case string(ExitReasonPartialVerified):
		return ExitReasonPartialVerified, nil
	case string(ExitReasonVerifierAbstain):
		return ExitReasonVerifierAbstain, nil
	case string(ExitReasonVerifierFail):
		return ExitReasonVerifierFail, nil
	case string(ExitReasonSystemAnomaly):
		return ExitReasonSystemAnomaly, nil
	case string(ExitReasonUnresolved):
		return ExitReasonUnresolved, nil
	case string(ExitReasonAbstain):
		return ExitReasonAbstain, nil
	default:
		return "", fmt.Errorf("turn: unknown ExitReason %q", s)
	}
}

// AllExitReasons returns the exhaustive list of 14 ExitReason enum values.
// Used by tests, dashboards, and verification tools that need to enumerate
// the complete taxonomy (e.g. D5 dashboard filter dropdowns).
func AllExitReasons() []ExitReason {
	return []ExitReason{
		ExitReasonNatural,
		ExitReasonMaxTurns,
		ExitReasonAbortedUser,
		ExitReasonAbortedLLM,
		ExitReasonAbortedTool,
		ExitReasonRepeatedTool,
		ExitReasonToolFailure,
		ExitReasonTokenDiminishing,
		ExitReasonPartialVerified,
		ExitReasonVerifierAbstain,
		ExitReasonVerifierFail,
		ExitReasonSystemAnomaly,
		ExitReasonUnresolved,
		ExitReasonAbstain,
	}
}

// Deterministic-exit thresholds. Aligned with clawcode's hard-coded
// constants in src/query/tokenBudget.ts and src/query.ts.
const (
	repeatedToolLookback             = 5
	repeatedToolThreshold            = 3
	consecutiveToolErrorThreshold    = 3
	tokenBudgetCompletionThreshold   = 0.9
	tokenBudgetDiminishingDelta      = 500
	tokenBudgetDiminishingChecks     = 2
)

// Metadata key on the final complete event carrying the exit reason.
const metadataKeyExitReason = "exit_reason"

// DefaultOrchestrator implements TurnOrchestrator with the canonical
// prepare→llm→tools→persist state machine (design.md §3).
type DefaultOrchestrator struct {
	llm              LLMInvoker
	context          ContextPreparer
	tools            ToolRoundExecutor
	persist          SessionPersister
	maxTurns         int
	defaultModel     string
	maxContextTokens int
	obsBridge        *observability.Bridge
	focusHint        FocusHintProvider
	resolveAwait     ResolveAwaiter
	toolResultStore  *persist.ToolResultStore
	maxToolResultCh  int
	maxAssistantCh   int
}

// NewOrchestrator creates a DefaultOrchestrator.
//
// MaxTurns ≤ 0 means *unbounded* — the loop runs until the LLM naturally
// finishes or one of the deterministic exit reasons fires. Match
// clawcode semantics where the main conversation has no hard turn
// ceiling; child agents that need a bound set it explicitly (see
// internal/layers/orchestration/delegatetools/builtin_agents.go).
func NewOrchestrator(deps OrchestratorDeps) *DefaultOrchestrator {
	// Leave deps.MaxTurns at 0 / negative — the orchestrator treats those
	// as "no safety net" rather than substituting a magic default. See
	// OrchestratorDeps.MaxTurns doc for the rationale.
	maxChars := deps.MaxToolResultChars
	if maxChars == 0 && deps.ToolResultStore != nil {
		maxChars = persist.DefaultMaxChars
	}
	assistChars := deps.MaxAssistantChars
	if assistChars == 0 && deps.ToolResultStore != nil {
		assistChars = persist.DefaultMaxAssistantChars
	}
	return &DefaultOrchestrator{
		llm:              deps.LLM,
		context:          deps.Context,
		tools:            deps.Tools,
		persist:          deps.Persist,
		maxTurns:         deps.MaxTurns,
		defaultModel:     deps.DefaultModel,
		maxContextTokens: deps.MaxContextTokens,
		obsBridge:        deps.ObsBridge,
		focusHint:        deps.FocusHint,
		resolveAwait:     deps.ResolveAwait,
		toolResultStore:  deps.ToolResultStore,
		maxToolResultCh:  maxChars,
		maxAssistantCh:   assistChars,
	}
}

// MaxTurns returns the orchestrator-level MaxTurns bound (0 = unbounded).
// Surfaced for diagnostics in the D7 bootstrap wiring log so the actual
// bound is observable in startup logs (the previous hardcoded 8 was
// misleading once the main conversation switched to unbounded).
func (o *DefaultOrchestrator) MaxTurns() int {
	return o.maxTurns
}

// RunTurn executes the full prepare→llm→tools→persist loop (D7-S2-A06).
//
// State machine (design.md §3):
//
//	START → PREPARE → LLM ↔ TOOL_ROUND → [exit-reason check] → PERSIST → COMPLETE
//	                    ↑                       │
//	                    └───────────────────────┘
//
// The loop runs until one of the ExitReason conditions fires:
//   - natural (LLM end_turn, no tool calls)
//   - max_turns (only when req.MaxTurns > 0 AND exceeded)
//   - aborted_user / aborted_llm / aborted_tool (fatal errors)
//   - repeated_tool / tool_failure / token_diminishing (deterministic safety)
//
// req.MaxTurns falls back to o.maxTurns only when o.maxTurns > 0; an
// orchestrator constructed with no MaxTurns runs unbounded.
func (o *DefaultOrchestrator) RunTurn(ctx context.Context, req TurnRequest) (<-chan *contracts.EngineEvent, error) {
	if req.SessionID == "" {
		return nil, fmt.Errorf("turn: SessionID is required")
	}
	if req.MaxTurns <= 0 && o.maxTurns > 0 {
		req.MaxTurns = o.maxTurns
	}

	ch := make(chan *contracts.EngineEvent, 32)
	go func() {
		defer close(ch)
		ctx, turnSpan := o.startSpan(ctx, telemetry.OpD7_S2_Orchestration_Turn_Run, tracer.SpanKindInternal,
			tracer.Attribute{Key: "session_id", Value: req.SessionID},
			tracer.Attribute{Key: "turn.scope", Value: string(req.Scope)},
			tracer.Attribute{Key: "turn.mode", Value: req.Mode},
			tracer.Attribute{Key: "turn.max_turns", Value: fmt.Sprintf("%d", req.MaxTurns)},
			tracer.Attribute{Key: "turn.model", Value: req.Model},
			tracer.Attribute{Key: "turn.nested", Value: boolStr(isNestedScope(req.Scope) || len(req.PreloadedMessages) > 0)},
			tracer.Attribute{Key: "turn.skip_persist", Value: boolStr(req.SkipPersist)},
			tracer.Attribute{Key: "context.caller", Value: "d7"},
			tracer.Attribute{Key: "context.runtime_path", Value: string(obsruntime.PathD7Turn)},
		)
		defer endSpan(turnSpan)
		o.runLoop(ctx, req, ch)
	}()
	return ch, nil
}

// runLoop is the internal state machine: PREPARE → LLM ↔ TOOL_ROUND → PERSIST.
// Cross-domain calls: D7→D2 (prepare/tools/persist), D7→D3 (LLM invoke).
//
// The main loop is `for { ... }` with no implicit turn bound (matching
// clawcode/src/query.ts:307 `while (true)`); the only hard cap is the
// optional MaxTurns safety net. Termination reasons are captured in the
// exitReason variable and surfaced on the final `complete` event's
// Metadata["exit_reason"] (see ExitReason constants).
func (o *DefaultOrchestrator) runLoop(ctx context.Context, req TurnRequest, out chan<- *contracts.EngineEvent) {
	start := time.Now()
	nested := isNestedScope(req.Scope) || len(req.PreloadedMessages) > 0

	var (
		prepared         PreparedContext
		err              error
		systemPrompt     string
		messages         []types.Message
		tools            []ToolSchema
		model            string
		maxContextTokens int
		persister        SessionPersister = o.persist
	)

	if nested {
		systemPrompt = strings.TrimSpace(req.SystemPrompt)
		messages = append([]types.Message{}, req.PreloadedMessages...)
		if len(req.UserMessage.Content) > 0 || req.UserMessage.Role != "" {
			messages = append(messages, req.UserMessage)
		}
		if len(req.OverrideTools) > 0 {
			tools = req.OverrideTools
		} else {
			ctx, prepSpan := o.startSpan(ctx, telemetry.OpD2_S2_Context_Process, tracer.SpanKindInternal,
				tracer.Attribute{Key: "session_id", Value: req.SessionID},
				tracer.Attribute{Key: "context.phase", Value: "prepare"},
				tracer.Attribute{Key: "context.caller", Value: "d7"},
				tracer.Attribute{Key: "context.worker_local", Value: boolStr(false)},
				tracer.Attribute{Key: "turn.scope", Value: string(req.Scope)},
				tracer.Attribute{Key: "turn.mode", Value: req.Mode},
			)
			prepared, err = o.context.Prepare(ctx, PrepareRequest{
				SessionID: req.SessionID,
				Message:   req.UserMessage,
				Mode:      req.Mode,
			})
			endSpan(prepSpan)
			if err != nil {
				o.emitError(out, req.SessionID,
					sharederrors.SanitizeForUser(fmt.Errorf("prepare failed: %w", err)),
					sharederrors.ErrorCode(err),
				)
				return
			}
			tools = prepared.Tools
		}
		model = req.Model
		if model == "" && prepared.Model != "" {
			model = prepared.Model
		}
		// DM-20260620-002 (AC1) — nested turns skip o.context.Prepare so
		// prepared stays at its zero value, which would leave
		// maxContextTokens=0 and disable runTokenAudit / proactive fold
		// / budgetTracker. Inject req.MaxContextTokens (carried by
		// SubTurnRunner from Cfg) and fall back to o.maxContextTokens
		// (Phase A wiring) when both are unset.
		maxContextTokens = req.MaxContextTokens
		if maxContextTokens <= 0 {
			maxContextTokens = o.maxContextTokens
		}
		if req.SkipPersist {
			persister = noopPersister{}
		}
	} else {
		// Step 1: PREPARE — D7 calls D2 for context assembly
		ctx, prepSpan := o.startSpan(ctx, telemetry.OpD2_S2_Context_Process, tracer.SpanKindInternal,
			tracer.Attribute{Key: "session_id", Value: req.SessionID},
			tracer.Attribute{Key: "context.phase", Value: "prepare"},
			tracer.Attribute{Key: "context.caller", Value: "d7"},
			tracer.Attribute{Key: "context.worker_local", Value: boolStr(false)},
			tracer.Attribute{Key: "turn.mode", Value: req.Mode},
		)
		prepared, err = o.context.Prepare(ctx, PrepareRequest{
			SessionID: req.SessionID,
			Message:   req.UserMessage,
			Mode:      req.Mode,
		})
		endSpan(prepSpan)
		if err != nil {
			o.emitError(out, req.SessionID,
				sharederrors.SanitizeForUser(fmt.Errorf("prepare failed: %w", err)),
				sharederrors.ErrorCode(err),
			)
			return
		}

		systemPrompt = mergeSystemPrompt(prepared.SystemPrompt, req.SystemPrompt)
		if o.focusHint != nil {
			if hint := strings.TrimSpace(o.focusHint.FocusHint(ctx, req.SessionID)); hint != "" {
				systemPrompt = mergeSystemPrompt(systemPrompt, hint)
			}
		}
		if o.resolveAwait != nil {
			if summary := strings.TrimSpace(o.resolveAwait.AwaitRunningChildren(ctx, req.SessionID)); summary != "" {
				systemPrompt = mergeSystemPrompt(systemPrompt, summary)
				out <- &contracts.EngineEvent{
					Type:      "resolve",
					Content:   summary,
					SessionID: req.SessionID,
				}
			}
		}

		// D-e: handle CompressHint — D7 calls D3 for summarization.
		if prepared.CompressHint != nil {
			compressCtx, compressSpan := o.startSpan(ctx, telemetry.OpD7_S2_Orchestration_LLM_Invoke, tracer.SpanKindClient,
				tracer.Attribute{Key: "session_id", Value: req.SessionID},
				tracer.Attribute{Key: "llm.purpose", Value: "compress"},
				tracer.Attribute{Key: "llm.trigger_reason", Value: "token_budget_exceeded"},
				tracer.Attribute{Key: "llm.target_token_budget", Value: fmt.Sprintf("%d", prepared.CompressHint.TargetTokenBudget)},
				tracer.Attribute{Key: "llm.messages_to_summarize", Value: fmt.Sprintf("%d", len(prepared.CompressHint.MessagesToSummarize))},
				tracer.Attribute{Key: "llm.model", Value: model},
			)
			result := o.runCompress(compressCtx, req, prepared.CompressHint)
			endSpan(compressSpan)
			prepared.Messages = []types.Message{{
				SessionID: req.SessionID,
				Role:      types.MessageRoleSystem,
				Content:   result.Summary,
			}}
			prepared.CompressHint = nil
		}

		messages = make([]types.Message, 0, len(prepared.Messages)+1)
		messages = append(messages, prepared.Messages...)
		messages = append(messages, req.UserMessage)
		tools = prepared.Tools
		model = prepared.Model
		maxContextTokens = prepared.MaxContextTokens
	}
	var totalUsage llmgateway.TokenUsage
	var lastPromptTokens int
	var finalText string
	// lastTurnText retains the most recent turn's LLM text (not accumulated
	// across turns). Surfaced on the complete event as meta["summary"] so
	// the IM "任务总结" card shows only the LLM's final synthesis, not the
	// full multi-turn report that the user already saw via streaming.
	var lastTurnText string
	// accumulatedText retains the LLM-emitted text from EVERY turn, not just
	// the last. Without this, the finalText/complete event only carries the
	// very last turn's content; a deep-review report emitted across many
	// earlier turns would be silently discarded at emitComplete, leaving the
	// IM card without its conclusion.
	var accumulatedText strings.Builder
	// lastThinkingTail retains the most recent LLM thinking content, post-strip.
	// Used as a finalText fallback at emitComplete time when the LLM never
	// emitted a clean summary (e.g. a provider without native reasoning emits
	// its working notes inside <think> tags and the splitter routed them to
	// thinking, leaving content empty — the user would otherwise see a blank
	// conclusion card).
	var lastThinkingTail strings.Builder

	// Step 2+3: LLM↔Tool loop. Termination is driven by the exitReason
	// local below; the loop body uses `break` (or `return` for fatal
	// aborts that already emit an error event) to land in the unified
	// finalize block at the bottom.
	exitReason := ExitReasonNatural
	turnCount := 0
	recentToolSignatures := make([]string, 0, repeatedToolLookback)
	var consecutiveErrorFP string
	consecutiveErrorCount := 0
	budgetTracker := newBudgetTracker(maxContextTokens)

	for {
		turnCount++

		// ctx cancellation / deadline — aborts with an explicit error
		// event so the IM adapter renders the cancellation rather than
		// waiting for an unobservable stream close.
		if err := ctx.Err(); err != nil {
			o.emitError(out, req.SessionID,
				sharederrors.SanitizeForUser(fmt.Errorf("turn cancelled: %w", err)),
				"CTX_CANCELLED",
			)
			return
		}

		// Safety net: only fires when the caller explicitly set a positive
		// MaxTurns. An unbounded turn (MaxTurns ≤ 0) is governed solely by
		// the natural-finish + deterministic-exit reasons below.
		if req.MaxTurns > 0 && turnCount > req.MaxTurns {
			exitReason = ExitReasonMaxTurns
			break
		}

		// DM-20260620-001 / AC4 + AC13: per-iteration token audit and
		// proactive fold. Runs BEFORE the LLM invoke so the request
		// payload is already trimmed. The audit result is attached to
		// the turn span and emitted as structured slog so post-hoc
		// analysis can correlate runaway sessions with budget pressure.
		//
		// AC3 (per-iter Prepare) is intentionally NOT moved inside the
		// loop: the Prepare → LLM → Tool pipeline is expensive to repeat
		// and the systemPrompt + Tools set is stable across a turn. The
		// audit is the high-leverage piece; a follow-up OpenSpec can
		// re-evaluate the Prepare cadence if needed.
		// streamRecoveryAttempts tracks how many times we've retried the stream
		// for max-output-token recovery in THIS turn. It is hoisted above the
		// turn + llm spans so the pre-invoke attributes capture the value
		// carried into the next iteration (most often 0; > 0 after a previous
		// iteration retried).
		streamRecoveryAttempts := 0

		turnCtx, turnSpan := o.startSpan(ctx, telemetry.OpD7_S2_Orchestration_Turn_Iteration, tracer.SpanKindInternal,
			tracer.Attribute{Key: "session_id", Value: req.SessionID},
			tracer.Attribute{Key: "turn.index", Value: fmt.Sprintf("%d", turnCount)},
			tracer.Attribute{Key: "turn.scope", Value: string(req.Scope)},
			tracer.Attribute{Key: "turn.mode", Value: req.Mode},
			tracer.Attribute{Key: "llm.model", Value: model},
			tracer.Attribute{Key: "context.max_context_tokens", Value: fmt.Sprintf("%d", maxContextTokens)},
			tracer.Attribute{Key: "context.message_count", Value: fmt.Sprintf("%d", len(messages))},
			tracer.Attribute{Key: "context.system_prompt_len", Value: fmt.Sprintf("%d", len(systemPrompt))},
			tracer.Attribute{Key: "context.tool_count", Value: fmt.Sprintf("%d", len(tools))},
			tracer.Attribute{Key: "context.nested", Value: boolStr(nested)},
			tracer.Attribute{Key: "context.budget_tracker_active", Value: boolStr(maxContextTokens > 0)},
		)
		o.runTokenAudit(ctx, systemPrompt, messages, maxContextTokens, turnCount, turnSpan)

		// D7→D3 LLM invoke (D7-S2-A07)
		turnCtx, llmSpan := o.startSpan(turnCtx, telemetry.OpD7_S2_Orchestration_LLM_Invoke, tracer.SpanKindClient,
			tracer.Attribute{Key: "session_id", Value: req.SessionID},
			tracer.Attribute{Key: "turn.index", Value: fmt.Sprintf("%d", turnCount)},
			tracer.Attribute{Key: "llm.purpose", Value: "turn"},
			tracer.Attribute{Key: "llm.model", Value: model},
			tracer.Attribute{Key: "llm.pre_message_count", Value: fmt.Sprintf("%d", len(messages))},
			tracer.Attribute{Key: "llm.pre_tool_count", Value: fmt.Sprintf("%d", len(tools))},
			tracer.Attribute{Key: "llm.max_context_tokens", Value: fmt.Sprintf("%d", maxContextTokens)},
			tracer.Attribute{Key: "llm.stream_recovery_attempts", Value: fmt.Sprintf("%d", streamRecoveryAttempts)},
		)
		var contentBuf strings.Builder
		var toolCalls []llmgateway.ToolCall
		var iterUsage llmgateway.TokenUsage
		var finishReason string
		// turnThinking accumulates all thinking chunks emitted in this turn.
		// After the inner stream loop, the most recent non-empty turn's
		// turnThinking.String() is preserved as lastThinkingTail for the
		// finalText fallback below (see emitComplete call sites).
		var turnThinking strings.Builder

	streamRecoveryLoop:
		for {
			chunkCh, err := o.invokeStreamWithRecovery(turnCtx, req, LLMInvokeRequest{
				SessionID:    req.SessionID,
				SystemPrompt: systemPrompt,
				Messages:     messages,
				Tools:        tools,
			})
			if err != nil {
				endSpanWithError(llmSpan, err)
				endSpan(turnSpan)
				o.emitError(out, req.SessionID,
					sharederrors.SanitizeForUser(fmt.Errorf("llm invoke failed: %w", err)),
					sharederrors.ErrorCode(err),
				)
				return
			}

			contentBuf.Reset()
			toolCalls = nil
			finishReason = ""
			iterUsage = llmgateway.TokenUsage{}
			turnThinking.Reset()
			var partial partialStreamEmit

			for chunk := range chunkCh {
				if chunk.FinishReason != "" {
					finishReason = chunk.FinishReason
				}
				if chunk.Thinking != "" {
					partial.hadThinking = true
					turnThinking.WriteString(chunk.Thinking)
					out <- &contracts.EngineEvent{
						Type:      "thinking",
						Content:   chunk.Thinking,
						SessionID: req.SessionID,
					}
				}
				if chunk.Content != "" {
					partial.hadText = true
					contentBuf.WriteString(chunk.Content)
					out <- &contracts.EngineEvent{
						Type:      "text",
						Content:   chunk.Content,
						SessionID: req.SessionID,
					}
				}
				if len(chunk.ToolCalls) > 0 {
					toolCalls = chunk.ToolCalls
					partial.toolCalls = chunk.ToolCalls
				}
				if chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0 {
					iterUsage = chunk.Usage
				} else if chunk.Usage.TotalTokens > 0 && iterUsage.PromptTokens == 0 && iterUsage.CompletionTokens == 0 {
					iterUsage = chunk.Usage
				}
			}

			if !NeedsMaxOutputTokenRecovery(finishReason) {
				break streamRecoveryLoop
			}
			if streamRecoveryAttempts >= maxOutputTokenRecoveryAttempts {
				break streamRecoveryLoop
			}
			emitStreamRecoveryTombstones(out, req.SessionID, partial)
			messages = append(messages, types.Message{
				SessionID: req.SessionID,
				Role:      types.MessageRoleUser,
				Content:   MaxOutputTokensRecoveryMessage,
			})
			streamRecoveryAttempts++
		}
		totalUsage.PromptTokens += iterUsage.PromptTokens
		totalUsage.CompletionTokens += iterUsage.CompletionTokens
		totalUsage.TotalTokens += iterUsage.TotalTokens
		if iterUsage.PromptTokens > 0 {
			lastPromptTokens = iterUsage.PromptTokens
		}
		endSpan(llmSpan)

		// Persist this turn's accumulated text into accumulatedText BEFORE
		// overwriting finalText, so the complete event carries the full
		// cross-turn report rather than only the last turn's content.
		if t := contentBuf.String(); t != "" {
			accumulatedText.WriteString(t)
			lastTurnText = t
		}
		finalText = accumulatedText.String()
		// Preserve this turn's accumulated thinking for the emitComplete
		// fallback below. Stash as lastThinkingTail AFTER finalText is set so
		// the most recent non-empty thinking is always the fallback source.
		if t := strings.TrimSpace(turnThinking.String()); t != "" {
			lastThinkingTail.Reset()
			lastThinkingTail.WriteString(t)
		}
		toolCalls = dedupeToolCalls(toolCalls)

		// No tool calls → natural LLM finish. The terminal `complete`
		// event below carries exit_reason=natural.
		if len(toolCalls) == 0 {
			exitReason = ExitReasonNatural
			break
		}

		// Repeated-tool detector: same (tool_name|input) signature
		// appears ≥ repeatedToolThreshold times in the last
		// repeatedToolLookback turns. The LLM is stuck retrying the
		// same action; continuing would burn tokens without progress.
		sig := toolCallsSignature(toolCalls)
		if isRepeatedToolSignature(sig, recentToolSignatures) {
			exitReason = ExitReasonRepeatedTool
			break
		}
		recentToolSignatures = append(recentToolSignatures, sig)
		if len(recentToolSignatures) > repeatedToolLookback {
			// Keep the most recent N signatures (drop the oldest).
			recentToolSignatures = recentToolSignatures[len(recentToolSignatures)-repeatedToolLookback:]
		}

		// Emit tool call events
		for _, tc := range toolCalls {
			out <- &contracts.EngineEvent{
				Type:      "tool_call",
				ToolName:  tc.Name,
				ToolInput: tc.Input,
				SessionID: req.SessionID,
				Metadata: map[string]string{
					"tool_name": tc.Name,
					"input":     tc.Input,
				},
			}
		}

		// D7→D2 tool execution
		_, toolSpan := o.startSpan(turnCtx, telemetry.OpD2_S5_Tool_Execute_Single, tracer.SpanKindInternal,
			tracer.Attribute{Key: "session_id", Value: req.SessionID},
			tracer.Attribute{Key: "tool.count", Value: fmt.Sprintf("%d", len(toolCalls))},
			tracer.Attribute{Key: "tool.names", Value: joinToolNames(toolCalls)},
			tracer.Attribute{Key: "turn.index", Value: fmt.Sprintf("%d", turnCount)},
			tracer.Attribute{Key: "context.caller", Value: "d7"},
			tracer.Attribute{Key: "context.runtime_path", Value: string(obsruntime.PathD7Turn)},
		)
		toolCtx := WithToolEventStream(turnCtx, func(ev *contracts.EngineEvent) {
			if ev == nil {
				return
			}
			select {
			case out <- ev:
			case <-turnCtx.Done():
			}
		})
		toolResult, err := o.tools.ExecuteRound(toolCtx, ToolRoundRequest{
			SessionID: req.SessionID,
			ToolCalls: toolCalls,
		})
		if err != nil {
			endSpanWithError(toolSpan, err)
			endSpan(turnSpan)
			o.emitError(out, req.SessionID,
				sharederrors.SanitizeForUser(fmt.Errorf("tool round failed: %w", err)),
				sharederrors.ErrorCode(err),
			)
			return
		}
		endSpan(toolSpan)

		// Emit tool result events
		for _, r := range toolResult.Results {
			content := r.Output
			if r.Error != "" {
				content = r.Error
			}
			toolName := toolNameForCallID(toolCalls, r.ToolCallID)
			out <- &contracts.EngineEvent{
				Type:      "tool_result",
				ToolName:  toolName,
				Content:   content,
				SessionID: req.SessionID,
				Metadata: map[string]string{
					"tool_name": toolName,
				},
			}
		}

		// Consecutive-tool-error detector: ≥ consecutiveToolErrorThreshold
		// consecutive turns produced at least one tool result with the
		// same non-empty error fingerprint. The LLM cannot recover from
		// this error pattern; further turns just repeat the same
		// rejection.
		if fp := toolResultErrorFingerprint(toolResult.Results); fp != "" {
			if fp == consecutiveErrorFP {
				consecutiveErrorCount++
			} else {
				consecutiveErrorFP = fp
				consecutiveErrorCount = 1
			}
			if consecutiveErrorCount >= consecutiveToolErrorThreshold {
				exitReason = ExitReasonToolFailure
				break
			}
		} else {
			consecutiveErrorFP = ""
			consecutiveErrorCount = 0
		}

		// Token-budget diminishing-returns detector. Mirrors
		// clawcode/src/query/tokenBudget.ts checkTokenBudget:
		// once cumulative usage crosses 90% of the context budget AND
		// the last two per-turn deltas are both below the floor,
		// continuing yields marginal value → stop. Disabled when
		// maxContextTokens is 0 / unset (the orchestrator has no
		// budget signal in that case).
		budgetTracker.observe(totalUsage)
		if budgetTracker.shouldStopDiminishing(maxContextTokens) {
			exitReason = ExitReasonTokenDiminishing
			break
		}

		// Build assistant tool-call message and tool result messages for the next turn.
		//
		// DM-20260620-001 / AC2: when the assistant text exceeds
		// maxAssistantChars (default 8K) the body is folded head/tail
		// style and the full content is persisted to disk.
		messages = append(messages, o.buildAssistantToolCallMsgFolded(req.SessionID, toolCalls, finalText, turnCount))
		for _, r := range toolResult.Results {
			toolName := toolNameForCallID(toolCalls, r.ToolCallID)
			messages = append(messages, o.buildToolResultMsgWithCap(req.SessionID, r, toolName))
		}

		// NOTE: finalText is intentionally NOT cleared here — it must
		// survive to the emitComplete call below, so MaxTurns-exceeded
		// (or any other non-natural exit after a tool round) emits the
		// last iteration's LLM text instead of an empty conclusion.
		endSpan(turnSpan)
	}

	// Finalize: persist + emit complete with the resolved exit reason.
	// The aborted_* paths already returned above with their own error
	// event; everything else (natural / max_turns / repeated_tool /
	// tool_failure / token_diminishing) reaches here and emits a
	// single terminal `complete` whose Metadata carries exit_reason.
	_, persistSpan := o.startSpan(ctx, telemetry.OpD2_S2_Context_Memory_Snapshot_Save, tracer.SpanKindInternal,
		tracer.Attribute{Key: "session_id", Value: req.SessionID},
		tracer.Attribute{Key: "context.caller", Value: "d7"},
		tracer.Attribute{Key: "context.runtime_path", Value: string(obsruntime.PathD7Turn)},
		tracer.Attribute{Key: string(metadataKeyExitReason), Value: string(exitReason)},
		tracer.Attribute{Key: "context.turn_count", Value: fmt.Sprintf("%d", turnCount)},
		tracer.Attribute{Key: "context.usage_prompt_tokens", Value: fmt.Sprintf("%d", totalUsage.PromptTokens)},
		tracer.Attribute{Key: "context.usage_completion_tokens", Value: fmt.Sprintf("%d", totalUsage.CompletionTokens)},
		tracer.Attribute{Key: "context.usage_total_tokens", Value: fmt.Sprintf("%d", totalUsage.TotalTokens)},
		tracer.Attribute{Key: "context.llm_model", Value: model},
		tracer.Attribute{Key: "context.max_context_tokens", Value: fmt.Sprintf("%d", maxContextTokens)},
		tracer.Attribute{Key: "context.final_text_len", Value: fmt.Sprintf("%d", len(finalText))},
	)
	resolvedFinal := resolveFinalText(finalText, lastThinkingTail.String(), exitReason, req.MaxTurns)
	// resolvedSummary is the brief conclusion that IM adapters render on the
	// standalone "任务总结" card. Distinct from resolvedFinal (the full
	// multi-turn transcript) so the card doesn't dump 75K chars of in-flight
	// tool-loop output that the user already saw via streaming text chunks.
	// We apply the same MaxTurns notice as resolvedFinal so the brief and
	// full paths agree on loop-bound signalling.
	resolvedSummary := resolveFinalText(lastTurnText, lastThinkingTail.String(), exitReason, req.MaxTurns)
	_ = persister.PersistTurn(ctx, PersistRequest{
		SessionID: req.SessionID,
		Messages:  messages,
		TurnCount: turnCount,
		Usage:     totalUsage,
		FinalText: resolvedFinal,
	})
	endSpan(persistSpan)
	o.emitComplete(out, req.SessionID, start, totalUsage, lastPromptTokens, model, maxContextTokens,
		resolvedFinal, resolvedSummary, exitReason)
}

// resolveFinalText promotes the most recent accumulated thinking into
// finalText when finalText is blank, so the IM adapter's conclusion card
// is never empty. The thinking content is <think>-stripped defensively
// in case the gateway splitter ever leaks a tag boundary.
//
// When the loop terminated on the MaxTurns safety net (exitReason ==
// ExitReasonMaxTurns), a truncation notice is prepended so the user can
// see the loop hit its bound rather than mistaking a quiet final-text
// for a normal end. Unbounded turns (maxTurns ≤ 0) never carry this
// notice regardless of how they exited.
func resolveFinalText(finalText, thinkingTail string, exitReason ExitReason, maxTurns int) string {
	promoted := finalText
	if strings.TrimSpace(promoted) == "" {
		promoted = textutil.StripThinkingTags(thinkingTail)
	}
	promoted = strings.TrimSpace(promoted)
	if exitReason == ExitReasonMaxTurns && maxTurns > 0 {
		notice := fmt.Sprintf("[max-turns reached after %d iterations; turn truncated]", maxTurns)
		if promoted == "" {
			return notice
		}
		return notice + "\n" + promoted
	}
	return promoted
}

// emitError emits a user-facing error event. The content MUST be pre-sanitized
// via sharederrors.SanitizeForUser — callers should NOT pass raw err.Error().
//
// DM-20260620-003 (AC2 + H1 + H4): variadic code parameter optionally carries
// the sentinel code (e.g. "LLM_AUTH_1004") so D1 IM adapters can render
// error-type-aware messages via event.Metadata["error_code"]. Existing call
// sites that pass only 3 args continue to work unchanged.
func (o *DefaultOrchestrator) emitError(out chan<- *contracts.EngineEvent, sessionID, content string, code ...string) {
	var metadata map[string]string
	if len(code) > 0 && code[0] != "" {
		metadata = map[string]string{"error_code": code[0]}
	}
	out <- &contracts.EngineEvent{
		Type:      "error",
		Content:   content,
		SessionID: sessionID,
		Metadata:  metadata,
	}
}

func (o *DefaultOrchestrator) emitComplete(
	out chan<- *contracts.EngineEvent,
	sessionID string,
	start time.Time,
	usage llmgateway.TokenUsage,
	lastPromptTokens int,
	model string,
	maxContextTokens int,
	finalText string,
	summary string,
	exitReason ExitReason,
) {
	if model == "" {
		model = o.defaultModel
	}
	if maxContextTokens <= 0 {
		maxContextTokens = o.maxContextTokens
	}
	meta := map[string]string{
		"duration": fmt.Sprintf("%d", time.Since(start).Milliseconds()),
		"usage":    fmt.Sprintf("%d", usageTokenTotal(usage)),
		metadataKeyExitReason: string(exitReason),
	}
	if model != "" {
		meta["model"] = model
	}
	if pct := contracts.ComputeCtxPct(lastPromptTokens, maxContextTokens); pct > 0 {
		meta["ctx_pct"] = fmt.Sprintf("%d", pct)
	}
	// summary is the brief conclusion surfaced on meta["summary"] so IM
	// adapters (Feishu) can render a concise "任务总结" card. Distinct from
	// Content (the full multi-turn transcript) which is preserved for the
	// session transcript jsonl + CLI stdout fallback path. When summary is
	// empty (e.g. provider that never emitted text), adapters fall back to
	// Content or the stats line so the user still gets a non-empty card.
	if summary != "" {
		meta["summary"] = summary
	}
	// Surface the last LLM-generated text on the complete event so IM adapters
	// (Feishu cardkit streaming finalize, CLI plain stdout) can render the
	// conclusion even when no interleaved text chunks were emitted (e.g. LLM
	// called tools, got result, ended the turn without an explicit summary).
	out <- &contracts.EngineEvent{
		Type:      "complete",
		Content:   finalText,
		SessionID: sessionID,
		Metadata:  meta,
	}
}

// CompressDegradation is the fallback level used when summarization fails.
type CompressDegradation int

const (
	CompressLLM        CompressDegradation = 0 // D3 summarization (primary)
	CompressTruncation CompressDegradation = 1 // keep recent messages
	CompressNone       CompressDegradation = 2 // pass through (no compression)
)

// compressResult wraps the summary with metadata about which strategy was used.
type compressResult struct {
	Summary     string
	Degradation CompressDegradation
	TruncatedTo int // number of messages kept (only for Truncation)
}

const maxTruncatedMessages = 20

// runCompress handles CompressHint with three-level degradation (D-e):
//
//	Level 1: D3 LLM summarization (primary path)
//	Level 2: Truncation — keep the most recent N messages if LLM fails
//	Level 3: Passthrough — if truncation is also empty, return the original content as-is
func (o *DefaultOrchestrator) runCompress(ctx context.Context, req TurnRequest, hint *CompressHint) compressResult {
	if hint == nil || len(hint.MessagesToSummarize) == 0 {
		return compressResult{Degradation: CompressNone}
	}

	// Level 1: D3 LLM summarization.
	systemPrompt := "Summarize the following conversation compactly, preserving key decisions, tool outputs, and facts. Keep the summary concise enough to fit within the remaining token budget."
	var contentBuilder strings.Builder
	for _, m := range hint.MessagesToSummarize {
		contentBuilder.WriteString(string(m.Role))
		contentBuilder.WriteString(": ")
		contentBuilder.WriteString(m.Content)
		contentBuilder.WriteString("\n")
	}

	chunkCh, err := o.llm.InvokeStream(ctx, LLMInvokeRequest{
		SessionID:    req.SessionID,
		Tier:         "",
		SystemPrompt: systemPrompt,
		Messages: []types.Message{
			{Role: types.MessageRoleUser, Content: contentBuilder.String()},
		},
	})
	if err == nil {
		var summaryBuilder strings.Builder
		for chunk := range chunkCh {
			if chunk.Content != "" {
				summaryBuilder.WriteString(chunk.Content)
			}
		}
		// Strip <think>...</think> blocks before storing. The LLM may emit
		// its working notes inside XML tags (minimax / DeepSeek-R1 w/o
		// native reasoning field) and those would otherwise be re-injected
		// into the next turn as a system message, polluting context and
		// teaching the LLM to wrap subsequent answers in <think> too.
		if summary := textutil.StripThinkingTags(summaryBuilder.String()); summary != "" {
			return compressResult{Summary: summary, Degradation: CompressLLM}
		}
	}

	// Level 2: Truncation fallback — keep the most recent messages.
	msgs := hint.MessagesToSummarize
	n := len(msgs)
	if n > maxTruncatedMessages {
		start := n - maxTruncatedMessages
		var buf strings.Builder
		for _, m := range msgs[start:] {
			buf.WriteString(string(m.Role))
			buf.WriteString(": ")
			buf.WriteString(m.Content)
			buf.WriteString("\n")
		}
		return compressResult{
			Summary:     buf.String(),
			Degradation: CompressTruncation,
			TruncatedTo: maxTruncatedMessages,
		}
	}

	// Level 3: Passthrough — return the original content (no compression).
	return compressResult{
		Summary:     contentBuilder.String(),
		Degradation: CompressNone,
	}
}

// buildAssistantToolCallMsg creates an assistant message containing tool call metadata.
// Uses the same metadata format as contextengine.tool_messages.go.
func buildAssistantToolCallMsg(sessionID string, calls []llmgateway.ToolCall, text string) types.Message {
	type serializedCall struct {
		ID       string `json:"id"`
		Type     string `json:"type"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}
	refs := make([]serializedCall, len(calls))
	for i, c := range calls {
		id := c.ID
		if id == "" {
			id = fmt.Sprintf("call_%d", i)
		}
		refs[i].ID = id
		refs[i].Type = "function"
		refs[i].Function.Name = c.Name
		args := c.Input
		if args == "" {
			args = "{}"
		}
		refs[i].Function.Arguments = args
	}
	raw, _ := json.Marshal(refs)
	return types.Message{
		SessionID: sessionID,
		Role:      types.MessageRoleAssistant,
		Content:   text,
		Metadata:  map[string]string{"tool_calls": string(raw)},
	}
}

// buildAssistantToolCallMsgFolded wraps buildAssistantToolCallMsg with
// DM-20260620-001 / AC2 head/tail folding when the text exceeds
// maxAssistantChars. The full content is persisted to disk via the
// shared ToolResultStore so it can be re-read on demand.
//
// A fold failure falls back to the raw message (no truncation) — better
// to send a long message than to drop the response entirely.
func (o *DefaultOrchestrator) buildAssistantToolCallMsgFolded(
	sessionID string,
	calls []llmgateway.ToolCall,
	text string,
	turnNum int,
) types.Message {
	msg := buildAssistantToolCallMsg(sessionID, calls, text)
	if o.toolResultStore == nil || o.maxAssistantCh <= 0 {
		return msg
	}
	if utf8.RuneCountInString(text) <= o.maxAssistantCh {
		return msg
	}
	folded, err := persist.FoldAssistantOutput(
		o.toolResultStore,
		sessionID,
		turnNum,
		"assistant",
		text,
		o.maxAssistantCh,
		0, // head → default
		0, // tail → default
	)
	if err != nil {
		slog.Warn("orchestrator: assistant output fold failed, leaving content untruncated",
			"session_id", sessionID, "turn", turnNum, "size", len(text), "error", err)
		return msg
	}
	msg.Content = folded
	return msg
}

// runTokenAudit implements DM-20260620-001 / AC4 + AC13: every iteration
// audits the in-loop messages against maxContextTokens and decides
// whether to proactively fold the largest assistant message.
//
// When ShouldFoldProactively fires, the largest assistant message is
// folded in-place (via buildAssistantToolCallMsgFolded reuse pattern,
// mutating the messages slice). The audit result is attached to the
// turn span and emitted as a structured slog entry so post-hoc analysis
// can correlate runaway sessions with budget pressure.
//
// Safe no-op when o.toolResultStore / o.maxAssistantCh / maxContextTokens
// are unset (legacy wiring).
func (o *DefaultOrchestrator) runTokenAudit(
	ctx context.Context,
	systemPrompt string,
	messages []types.Message,
	maxContextTokens int,
	turnNum int,
	turnSpan interface{ SetAttributes(...tracer.Attribute) },
) {
	if o.toolResultStore == nil || o.maxAssistantCh <= 0 || maxContextTokens <= 0 {
		return
	}
	counter := token.NewCounter()
	res := audit.AuditMessages(counter, systemPrompt, messages, maxContextTokens)
	proactive := audit.ShouldFoldProactively(res, o.maxAssistantCh, audit.DefaultProactiveFoldPercent)

	// AC13: span attributes + structured slog.
	attrs := []tracer.Attribute{
		{Key: "audit.total_tokens", Value: res.TotalTokens},
		{Key: "audit.system_tokens", Value: res.SystemTokens},
		{Key: "audit.messages_tokens", Value: res.MessagesTokens},
		{Key: "audit.largest_msg_tokens", Value: res.LargestMsgTokens},
		{Key: "audit.budget_percent", Value: res.BudgetPercent},
		{Key: "audit.over_budget", Value: res.OverBudget},
		{Key: "audit.proactive_fold_triggered", Value: proactive},
	}
	if turnSpan != nil {
		turnSpan.SetAttributes(attrs...)
	}
	slog.Info("orchestrator: token audit",
		"turn", turnNum,
		"total_tokens", res.TotalTokens,
		"system_tokens", res.SystemTokens,
		"messages_tokens", res.MessagesTokens,
		"largest_msg_tokens", res.LargestMsgTokens,
		"largest_msg_idx", res.LargestMsgIdx,
		"budget", maxContextTokens,
		"budget_percent", res.BudgetPercent,
		"over_budget", res.OverBudget,
		"proactive_fold", proactive,
	)

	if !proactive || res.LargestMsgIdx < 0 || res.LargestMsgIdx >= len(messages) {
		return
	}
	target := &messages[res.LargestMsgIdx]
	if target.Role != types.MessageRoleAssistant {
		return // AC4 only folds assistant messages.
	}
	folded, err := persist.FoldAssistantOutput(
		o.toolResultStore,
		"", // session ID is not in scope here; persist under root.
		turnNum,
		"assistant",
		target.Content,
		o.maxAssistantCh,
		0, 0,
	)
	if err != nil || folded == target.Content {
		return
	}
	slog.Info("orchestrator: proactive fold applied",
		"turn", turnNum,
		"msg_idx", res.LargestMsgIdx,
		"orig_chars", len(target.Content),
		"folded_chars", len(folded),
	)
	target.Content = folded
}

// buildToolResultMsg creates a tool result message.
func buildToolResultMsg(sessionID string, r ToolResult) types.Message {
	content := r.Output
	if r.Error != "" {
		content = r.Error
	}
	return types.Message{
		SessionID: sessionID,
		Role:      types.MessageRoleTool,
		Content:   content,
		Metadata:  map[string]string{"tool_call_id": r.ToolCallID},
	}
}

// buildToolResultMsgWithCap is DM-20260620-001 / AC1: when the result
// belongs to a size-capped tool and exceeds maxToolResultChars, the
// content is persisted to disk via toolResultStore and replaced with a
// small preview marker so the LLM context budget does not blow up.
//
// Falls back to a head truncation (no disk write) when the store call
// errors — a transient I/O failure must not abort the turn.
//
// Tool errors are routed through FormatToolResultContent so the existing
// error sanitisation applies (config-path stripping, length cap).
func (o *DefaultOrchestrator) buildToolResultMsgWithCap(sessionID string, r ToolResult, toolName string) types.Message {
	content := FormatToolResultContentForLLM(toolName, r.Output, r.Error)
	if o.toolResultStore != nil && o.maxToolResultCh > 0 &&
		persist.ShouldCap(toolName) &&
		utf8.RuneCountInString(content) > o.maxToolResultCh {
		previewed, err := o.toolResultStore.Persist(context.Background(), sessionID, toolName, r.ToolCallID, content, o.maxToolResultCh)
		if err != nil {
			slog.Warn("orchestrator: tool result persist failed, falling back to head truncation",
				"tool", toolName, "session_id", sessionID, "size", len(content), "error", err)
			content = truncatePreview(content, o.maxToolResultCh) + "\n...[truncated, persist failed]"
		} else {
			content = previewed
		}
	}
	return types.Message{
		SessionID: sessionID,
		Role:      types.MessageRoleTool,
		Content:   content,
		Metadata:  map[string]string{"tool_call_id": r.ToolCallID},
	}
}

// FormatToolResultContentForLLM centralises the formatting rules for
// tool result content. Errors are shortened via the existing helper; for
// success-only results the content is returned unchanged.
func FormatToolResultContentForLLM(toolName, output, errMsg string) string {
	if strings.TrimSpace(errMsg) == "" {
		return output
	}
	return conversation.FormatToolResultContent(toolName, output, errMsg)
}

// truncatePreview returns the first n runes of s followed by a tail
// marker. Used as a fallback when the persistent store is unavailable.
func truncatePreview(s string, n int) string {
	if n <= 0 {
		return s
	}
	count := 0
	for i := range s {
		if count == n {
			return s[:i]
		}
		count++
	}
	return s
}

func mergeSystemPrompt(prepared, extra string) string {
	prepared = strings.TrimSpace(prepared)
	extra = strings.TrimSpace(extra)
	switch {
	case prepared == "":
		return extra
	case extra == "":
		return prepared
	default:
		return prepared + "\n" + extra
	}
}

func usageTokenTotal(u llmgateway.TokenUsage) int {
	if u.PromptTokens > 0 || u.CompletionTokens > 0 {
		return u.PromptTokens + u.CompletionTokens
	}
	return u.TotalTokens
}

func toolNameForCallID(calls []llmgateway.ToolCall, id string) string {
	id = strings.TrimSpace(id)
	for _, tc := range calls {
		if strings.TrimSpace(tc.ID) == id {
			return tc.Name
		}
	}
	return ""
}

func dedupeToolCalls(calls []llmgateway.ToolCall) []llmgateway.ToolCall {
	if len(calls) <= 1 {
		return calls
	}
	seen := make(map[string]struct{}, len(calls))
	out := make([]llmgateway.ToolCall, 0, len(calls))
	for i, c := range calls {
		id := strings.TrimSpace(c.ID)
		if id == "" {
			id = fmt.Sprintf("call_%d", i)
			c.ID = id
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, c)
	}
	return out
}

// toolCallsSignature produces a stable "name|input" signature for a batch
// of tool calls. Parallel tool calls are sorted by name+input so two
// semantically equivalent batches always hash to the same signature.
// The signature is what the repeated-tool detector compares turn-over-turn.
func toolCallsSignature(calls []llmgateway.ToolCall) string {
	if len(calls) == 0 {
		return ""
	}
	parts := make([]string, 0, len(calls))
	for _, c := range calls {
		parts = append(parts, c.Name+"|"+c.Input)
	}
	sort.Strings(parts)
	return strings.Join(parts, ";")
}

// isRepeatedToolSignature returns true when the given signature has
// already appeared ≥ repeatedToolThreshold times in the lookback window.
// The LLM is considered stuck when it keeps retrying the same action
// regardless of intervening context — the orchestrator breaks the loop
// and surfaces ExitReasonRepeatedTool on the final complete event.
func isRepeatedToolSignature(sig string, history []string) bool {
	if sig == "" {
		return false
	}
	count := 0
	for _, prev := range history {
		if prev == sig {
			count++
		}
	}
	return count >= repeatedToolThreshold
}

// toolResultErrorFingerprint builds a stable fingerprint over the error
// text of a tool round. Returns "" when the round had no errors (a
// clean round resets the consecutive-error counter). Different error
// strings produce different fingerprints so the counter only fires on
// the *same* error pattern repeating.
func toolResultErrorFingerprint(results []ToolResult) string {
	var firstErr string
	var count int
	for _, r := range results {
		if strings.TrimSpace(r.Error) == "" {
			continue
		}
		if firstErr == "" {
			firstErr = r.Error
		}
		count++
	}
	if count == 0 {
		return ""
	}
	return fmt.Sprintf("%s|count=%d", firstErr, count)
}

// budgetTracker observes per-turn token usage deltas and decides when
// the cumulative spend has crossed the context budget *and* the last
// two increments were both small enough that further turns add
// marginal value at best. Mirrors clawcode/src/query/tokenBudget.ts
// checkTokenBudget (90% threshold + 500-token floor + 2 consecutive
// small-delta observations).
type budgetTracker struct {
	lastTotal    int
	recentDeltas [tokenBudgetDiminishingChecks]int
	deltasFilled int
}

func newBudgetTracker(_ int) *budgetTracker {
	return &budgetTracker{}
}

// observe records a cumulative usage snapshot. The delta between the
// previous and current total tokens is stored in the rolling window.
func (b *budgetTracker) observe(usage llmgateway.TokenUsage) {
	total := usageTokenTotal(usage)
	delta := total - b.lastTotal
	if delta < 0 {
		// Provider reset / first observation; treat the new total as
		// a fresh baseline rather than a negative delta.
		delta = total
	}
	b.lastTotal = total
	if b.deltasFilled < tokenBudgetDiminishingChecks {
		b.recentDeltas[b.deltasFilled] = delta
		b.deltasFilled++
		return
	}
	// Shift left, append at the end. Keeps the most recent N deltas.
	for i := 0; i < tokenBudgetDiminishingChecks-1; i++ {
		b.recentDeltas[i] = b.recentDeltas[i+1]
	}
	b.recentDeltas[tokenBudgetDiminishingChecks-1] = delta
}

// shouldStopDiminishing returns true when both conditions hold:
//   - cumulative usage has crossed tokenBudgetCompletionThreshold of
//     maxContextTokens (i.e. ≥ 90% of the context window is consumed);
//   - the most recent tokenBudgetDiminishingChecks per-turn deltas are
//     all below tokenBudgetDiminishingDelta (each turn is adding < 500
//     tokens of useful work).
//
// When maxContextTokens is 0 / unset the orchestrator has no budget
// signal and the detector stays disabled.
func (b *budgetTracker) shouldStopDiminishing(maxContextTokens int) bool {
	if maxContextTokens <= 0 || b.deltasFilled < tokenBudgetDiminishingChecks {
		return false
	}
	if b.lastTotal < int(float64(maxContextTokens)*tokenBudgetCompletionThreshold) {
		return false
	}
	for _, d := range b.recentDeltas[:tokenBudgetDiminishingChecks] {
		if d >= tokenBudgetDiminishingDelta {
			return false
		}
	}
	return true
}

// boolStr renders a boolean as a span-attribute friendly string.
func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// joinToolNames concatenates tool-call names for span attributes. Caps the
// rendered length at 8 names + an overflow suffix so the attribute stays
// cheap to index in Jaeger even for batches of many tool calls.
func joinToolNames(tcs []llmgateway.ToolCall) string {
	const max = 8
	if len(tcs) == 0 {
		return ""
	}
	parts := make([]string, 0, max+1)
	for i, tc := range tcs {
		if i >= max {
			parts = append(parts, fmt.Sprintf("+%d_more", len(tcs)-max))
			break
		}
		parts = append(parts, tc.Name)
	}
	return strings.Join(parts, ",")
}
