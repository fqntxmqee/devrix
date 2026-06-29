package sessionorchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/executionflow/verify"
	obsruntime "github.com/devrix/devrix/internal/layers/observability/configure/runtime"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/contracts"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

// runLoopState is the loop-local mutable state of runLoop. Threaded
// through the prepare, runLLMStream, executeToolRound, exit-condition,
// and finalize helpers so the orchestration reads as a pipeline of
// small operations rather than a 500-line god function.
//
// Split out from turn_orchestrator.go in DM-20260629-001 PR-2 (turn-fn-split
// first batch) along with RunTurn, runLoop, startTurnSpan, and the
// pre/post-tool exit checkers so the package surface maps 1:1 to the
// D7-S2-A06..A09 sub-activities registered in t-registry.
type runLoopState struct {
	// Prepare-phase output (immutable across the loop).
	systemPrompt     string
	messages         []types.Message
	tools            []ToolSchema
	model            string
	maxContextTokens int
	persister        SessionPersister
	nested           bool

	// Cross-turn accumulators (mutated each iteration).
	totalUsage       llmgateway.TokenUsage
	lastPromptTokens int
	// finalText is rebuilt from accumulatedText each iteration. Surfaced
	// on the complete event's Content.
	finalText string
	// lastTurnText retains only the most recent turn's LLM text.
	// Surfaced on the complete event as meta["summary"] so the IM
	// "任务总结" card shows only the LLM's final synthesis, not the
	// full multi-turn report that the user already saw via streaming.
	lastTurnText string
	// accumulatedText retains the LLM-emitted text from EVERY turn, not
	// just the last. Without this, the finalText/complete event only
	// carries the very last turn's content; a deep-review report emitted
	// across many earlier turns would be silently discarded at
	// emitComplete, leaving the IM card without its conclusion.
	accumulatedText strings.Builder
	// lastThinkingTail retains the most recent LLM thinking content,
	// post-strip. Used as a finalText fallback at emitComplete time when
	// the LLM never emitted a clean summary (e.g. a provider without
	// native reasoning emits its working notes inside <think> tags and
	// the splitter routed them to thinking, leaving content empty — the
	// user would otherwise see a blank conclusion card).
	lastThinkingTail strings.Builder

	// Deterministic-exit state (mutated each iteration).
	exitReason            verify.ExitReason
	turnCount             int
	recentToolSignatures  []string
	consecutiveErrorFP    string
	consecutiveErrorCount int
	budgetTracker         *budgetTracker
	// userContextPrepend applied at LLM invoke only (not persisted).
	userContextPrepend map[string]string
}

// RunTurn executes the full prepare→llm→tools→persist loop (D7-S2-A06).
//
// State machine (design.md §3):
//
//	START → PREPARE → LLM ↔ TOOL_ROUND → [exit-reason check] → PERSIST → COMPLETE
//	                    ↑                       │
//	                    └───────────────────────┘
//
// The loop runs until one of the verify.ExitReason conditions fires:
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
// optional MaxTurns safety net. Termination reasons are captured in
// st.exitReason and surfaced on the final `complete` event's
// Metadata["exit_reason"] (see verify.ExitReason constants).
//
// Refactored in DM-20260625-012 from a 513-line god function into a slim
// driver that calls prepareContext, runLLMStream, executeToolRound, the
// exit-condition helpers, and finalizeLoop. State flows through
// runLoopState. PR-2 (DM-20260629-001) lifted RunTurn + runLoop +
// runLoopState + startTurnSpan + pre/post-tool exit checks out of
// turn_orchestrator.go into this file; see design.md §turn-fn-split.
func (o *DefaultOrchestrator) runLoop(ctx context.Context, req TurnRequest, out chan<- *contracts.EngineEvent) {
	start := time.Now()

	st, err := o.prepareContext(ctx, req, out)
	if err != nil {
		return // error already emitted
	}
	st.budgetTracker = newBudgetTracker(st.maxContextTokens)

	for {
		st.turnCount++

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
		if req.MaxTurns > 0 && st.turnCount > req.MaxTurns {
			st.exitReason = verify.ExitReasonMaxTurns
			break
		}

		// turn span + per-iter token audit
		turnCtx, turnSpan := o.startTurnSpan(ctx, req, st)
		o.runTokenAudit(ctx, st.systemPrompt, st.messages, st.maxContextTokens, st.turnCount, turnSpan)

		// LLM stream (with max-output-token recovery)
		text, toolCalls, usage, err := o.runLLMStream(turnCtx, req, st, out)
		if err != nil {
			endSpan(turnSpan)
			return // error already emitted
		}
		st.totalUsage.PromptTokens += usage.PromptTokens
		st.totalUsage.CompletionTokens += usage.CompletionTokens
		st.totalUsage.TotalTokens += usage.TotalTokens
		if usage.PromptTokens > 0 {
			st.lastPromptTokens = usage.PromptTokens
		}

		// Persist this turn's accumulated text into accumulatedText BEFORE
		// overwriting finalText, so the complete event carries the full
		// cross-turn report rather than only the last turn's content.
		if t := text; t != "" {
			st.accumulatedText.WriteString(t)
			st.lastTurnText = t
		}
		st.finalText = st.accumulatedText.String()
		// Preserve this turn's accumulated thinking for the emitComplete
		// fallback below. Stash as lastThinkingTail AFTER finalText is set so
		// the most recent non-empty thinking is always the fallback source.
		if t := strings.TrimSpace(st.lastThinkingTail.String()); t != "" {
			st.lastThinkingTail.Reset()
			st.lastThinkingTail.WriteString(t)
		}
		toolCalls = dedupeToolCalls(toolCalls)

		// Pre-tool exit conditions (natural / repeated tool).
		if o.checkPreToolExits(st, toolCalls) {
			endSpan(turnSpan)
			break
		}

		// Emit tool call events + execute tool round + emit results.
		toolResult, err := o.executeToolRound(turnCtx, req, st, toolCalls, out)
		if err != nil {
			endSpan(turnSpan)
			return // error already emitted
		}

		// Post-tool exit conditions (consecutive error / diminishing returns).
		if o.checkPostToolExits(st, toolResult) {
			endSpan(turnSpan)
			break
		}

		// Build assistant tool-call message and tool result messages for
		// the next turn. DM-20260620-001 / AC2: when the assistant text
		// exceeds maxAssistantChars (default 8K) the body is folded
		// head/tail style and the full content is persisted to disk.
		st.messages = append(st.messages,
			o.buildAssistantToolCallMsgFolded(req.SessionID, toolCalls, st.finalText, st.turnCount))
		for _, r := range toolResult.Results {
			toolName := toolNameForCallID(toolCalls, r.ToolCallID)
			st.messages = append(st.messages, o.buildToolResultMsgWithCap(req.SessionID, r, toolName))
		}

		// NOTE: finalText is intentionally NOT cleared here — it must
		// survive to the emitComplete call below, so MaxTurns-exceeded
		// (or any other non-natural exit after a tool round) emits the
		// last iteration's LLM text instead of an empty conclusion.
		endSpan(turnSpan)
	}

	o.finalizeLoop(ctx, req, st, out, start)
}

// startTurnSpan opens the per-iteration telemetry span and attaches the
// cross-turn attributes (turn index, scope, model, context sizes). The
// stream-recovery counter is initialized to 0 here (carried into the
// pre-invoke attributes for the LLM span).
func (o *DefaultOrchestrator) startTurnSpan(ctx context.Context, req TurnRequest, st *runLoopState) (context.Context, tracer.Span) {
	turnCtx, turnSpan := o.startSpan(ctx, telemetry.OpD7_S2_Orchestration_Turn_Iteration, tracer.SpanKindInternal,
		tracer.Attribute{Key: "session_id", Value: req.SessionID},
		tracer.Attribute{Key: "turn.index", Value: fmt.Sprintf("%d", st.turnCount)},
		tracer.Attribute{Key: "turn.scope", Value: string(req.Scope)},
		tracer.Attribute{Key: "turn.mode", Value: req.Mode},
		tracer.Attribute{Key: "llm.model", Value: st.model},
		tracer.Attribute{Key: "context.max_context_tokens", Value: fmt.Sprintf("%d", st.maxContextTokens)},
		tracer.Attribute{Key: "context.message_count", Value: fmt.Sprintf("%d", len(st.messages))},
		tracer.Attribute{Key: "context.system_prompt_len", Value: fmt.Sprintf("%d", len(st.systemPrompt))},
		tracer.Attribute{Key: "context.tool_count", Value: fmt.Sprintf("%d", len(st.tools))},
		tracer.Attribute{Key: "context.nested", Value: boolStr(st.nested)},
		tracer.Attribute{Key: "context.budget_tracker_active", Value: boolStr(st.maxContextTokens > 0)},
	)
	return turnCtx, turnSpan
}

// checkPreToolExits applies the pre-tool-round exit detectors: natural
// finish (no tool calls → verify.ExitReasonNatural) and the
// repeated-tool detector (same signature ≥ repeatedToolThreshold
// times in the last repeatedToolLookback turns → verify.ExitReasonRepeatedTool).
// Updates st.recentToolSignatures as a side effect. Returns true if
// the loop should break.
func (o *DefaultOrchestrator) checkPreToolExits(st *runLoopState, toolCalls []llmgateway.ToolCall) bool {
	// No tool calls → natural LLM finish. The terminal `complete`
	// event below carries exit_reason=natural.
	if len(toolCalls) == 0 {
		st.exitReason = verify.ExitReasonNatural
		return true
	}

	// Repeated-tool detector: same (tool_name|input) signature
	// appears ≥ repeatedToolThreshold times in the last
	// repeatedToolLookback turns. The LLM is stuck retrying the
	// same action; continuing would burn tokens without progress.
	sig := toolCallsSignature(toolCalls)
	if isRepeatedToolSignature(sig, st.recentToolSignatures) {
		st.exitReason = verify.ExitReasonRepeatedTool
		return true
	}
	st.recentToolSignatures = append(st.recentToolSignatures, sig)
	if len(st.recentToolSignatures) > repeatedToolLookback {
		// Keep the most recent N signatures (drop the oldest).
		st.recentToolSignatures = st.recentToolSignatures[len(st.recentToolSignatures)-repeatedToolLookback:]
	}
	return false
}

// checkPostToolExits applies the post-tool-round exit detectors: the
// consecutive-tool-error detector (≥ consecutiveToolErrorThreshold
// turns with the same error fingerprint → verify.ExitReasonToolFailure) and
// the token-budget diminishing-returns detector (cumulative usage
// crosses 90% of the context budget AND last two deltas below the
// floor → verify.ExitReasonTokenDiminishing). Updates
// st.consecutiveErrorFP/Count and st.budgetTracker as side effects.
// Returns true if the loop should break.
func (o *DefaultOrchestrator) checkPostToolExits(st *runLoopState, toolResult ToolRoundResult) bool {
	// Consecutive-tool-error detector: ≥ consecutiveToolErrorThreshold
	// consecutive turns produced at least one tool result with the
	// same non-empty error fingerprint. The LLM cannot recover from
	// this error pattern; further turns just repeat the same
	// rejection.
	if fp := toolResultErrorFingerprint(toolResult.Results); fp != "" {
		if fp == st.consecutiveErrorFP {
			st.consecutiveErrorCount++
		} else {
			st.consecutiveErrorFP = fp
			st.consecutiveErrorCount = 1
		}
		if st.consecutiveErrorCount >= consecutiveToolErrorThreshold {
			st.exitReason = verify.ExitReasonToolFailure
			return true
		}
	} else {
		st.consecutiveErrorFP = ""
		st.consecutiveErrorCount = 0
	}

	// Token-budget diminishing-returns detector. Mirrors
	// clawcode/src/query/tokenBudget.ts checkTokenBudget:
	// once cumulative usage crosses 90% of the context budget AND
	// the last two per-turn deltas are both below the floor,
	// continuing yields marginal value → stop. Disabled when
	// maxContextTokens is 0 / unset (the orchestrator has no
	// budget signal in that case).
	st.budgetTracker.observe(st.totalUsage)
	if st.budgetTracker.shouldStopDiminishing(st.maxContextTokens) {
		st.exitReason = verify.ExitReasonTokenDiminishing
		return true
	}
	return false
}