package sessionorchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/persist"
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/orchestration/executionflow/verify"
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
	LLM     LLMInvoker
	Context ContextPreparer
	Tools   ToolRoundExecutor
	Persist SessionPersister
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
	// PromptLanguage controls LLM-facing compression prompts (zh-CN | en-US).
	PromptLanguage string
	// FallbackModel is the optional secondary model used when the primary
	// model returns RateLimit/ServerError ≥ 2 consecutive times.
	//
	// DM-20260628-001 (FR-13, AC3 partial): field reservation only. Empty =
	// fallback disabled. Full retry-loop wiring is the P0-2 follow-up
	// (`devrix-streaming-fallback`); S4 only logs fallback_trigger_candidate
	// + fallback_model_set_but_not_yet_wired for observability.
	FallbackModel string
}

// verify.ExitReason is defined in exit_reason.go.

// Deterministic-exit thresholds. Aligned with clawcode's hard-coded
// constants in src/query/tokenBudget.ts and src/query.ts.
const (
	repeatedToolLookback           = 5
	repeatedToolThreshold          = 3
	consecutiveToolErrorThreshold  = 3
	tokenBudgetCompletionThreshold = 0.9
	tokenBudgetDiminishingDelta    = 500
	tokenBudgetDiminishingChecks   = 2
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
	promptLanguage   string
	// fallbackModel — DM-20260628-001 (FR-13). Empty = fallback disabled.
	fallbackModel string
	// consecutiveServerErrors counts consecutive APICodeRateLimit/ServerError
	// responses from the primary model; reset on success or non-retryable error.
	consecutiveServerErrors int
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
		promptLanguage:   deps.PromptLanguage,
		fallbackModel:    deps.FallbackModel,
	}
}

// MaxTurns returns the orchestrator-level MaxTurns bound (0 = unbounded).
// Surfaced for diagnostics in the D7 bootstrap wiring log so the actual
// bound is observable in startup logs (the previous hardcoded 8 was
// misleading once the main conversation switched to unbounded).
func (o *DefaultOrchestrator) MaxTurns() int {
	return o.maxTurns
}



// finalizeLoop persists the turn and emits the terminal `complete`
// event. The aborted_* paths already returned above with their own
// error event; everything else (natural / max_turns / repeated_tool /
// tool_failure / token_diminishing) reaches here and emits a single
// terminal `complete` whose Metadata carries exit_reason.
func (o *DefaultOrchestrator) finalizeLoop(
	ctx context.Context,
	req TurnRequest,
	st *runLoopState,
	out chan<- *contracts.EngineEvent,
	start time.Time,
) {
	_, persistSpan := o.startSpan(ctx, telemetry.OpD2_S2_Context_Memory_Snapshot_Save, tracer.SpanKindInternal,
		tracer.Attribute{Key: "session_id", Value: req.SessionID},
		tracer.Attribute{Key: "context.caller", Value: "d7"},
		tracer.Attribute{Key: "context.runtime_path", Value: string(obsruntime.PathD7Turn)},
		tracer.Attribute{Key: string(metadataKeyExitReason), Value: string(st.exitReason)},
		tracer.Attribute{Key: "context.turn_count", Value: fmt.Sprintf("%d", st.turnCount)},
		tracer.Attribute{Key: "context.usage_prompt_tokens", Value: fmt.Sprintf("%d", st.totalUsage.PromptTokens)},
		tracer.Attribute{Key: "context.usage_completion_tokens", Value: fmt.Sprintf("%d", st.totalUsage.CompletionTokens)},
		tracer.Attribute{Key: "context.usage_total_tokens", Value: fmt.Sprintf("%d", st.totalUsage.TotalTokens)},
		tracer.Attribute{Key: "context.llm_model", Value: st.model},
		tracer.Attribute{Key: "context.max_context_tokens", Value: fmt.Sprintf("%d", st.maxContextTokens)},
		tracer.Attribute{Key: "context.final_text_len", Value: fmt.Sprintf("%d", len(st.finalText))},
	)
	resolvedFinal := resolveFinalText(st.finalText, st.lastThinkingTail.String(), st.exitReason, req.MaxTurns)
	// resolvedSummary is the brief conclusion that IM adapters render on the
	// standalone "任务总结" card. Distinct from resolvedFinal (the full
	// multi-turn transcript) so the card doesn't dump 75K chars of in-flight
	// tool-loop output that the user already saw via streaming text chunks.
	// We apply the same MaxTurns notice as resolvedFinal so the brief and
	// full paths agree on loop-bound signalling.
	resolvedSummary := resolveFinalText(st.lastTurnText, st.lastThinkingTail.String(), st.exitReason, req.MaxTurns)
	if err := st.persister.PersistTurn(ctx, PersistRequest{
		SessionID: req.SessionID,
		Messages:  st.messages,
		TurnCount: st.turnCount,
		Usage:     st.totalUsage,
		FinalText: resolvedFinal,
	}); err != nil {
		slog.Warn("orchestrator: persist turn failed; emitting complete with in-memory state only",
			"session_id", req.SessionID,
			"turn_count", st.turnCount,
			"exit_reason", string(st.exitReason),
			"error", err)
	}
	endSpan(persistSpan)
	o.emitComplete(out, req.SessionID, start, st.totalUsage, st.lastPromptTokens, st.model, st.maxContextTokens,
		resolvedFinal, resolvedSummary, st.exitReason)
}

// resolveFinalText promotes the most recent accumulated thinking into
// finalText when finalText is blank, so the IM adapter's conclusion card
// is never empty. The thinking content is <think>-stripped defensively
// in case the gateway splitter ever leaks a tag boundary.
//
// When the loop terminated on the MaxTurns safety net (exitReason ==
// verify.ExitReasonMaxTurns), a truncation notice is prepended so the user can
// see the loop hit its bound rather than mistaking a quiet final-text
// for a normal end. Unbounded turns (maxTurns ≤ 0) never carry this
// notice regardless of how they exited.
func resolveFinalText(finalText, thinkingTail string, exitReason verify.ExitReason, maxTurns int) string {
	promoted := finalText
	if strings.TrimSpace(promoted) == "" {
		promoted = textutil.StripThinkingTags(thinkingTail)
	}
	promoted = strings.TrimSpace(promoted)
	if exitReason == verify.ExitReasonMaxTurns && maxTurns > 0 {
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
// DM-20260620-003 (AC2 + H1 + H4): variadic code parameter is retained for
// backward compat with out-of-tree callers (the legacy explicit-code path
// was retired in v2.6.0 — DM-20260629-001 — and all in-tree call sites
// pass only 3 args).
//
// DM-20260628-001 (FR-15): the closed-set APIErrorCode is extracted from
// sharederrors.Code(err) so D1 IM adapters receive a controlled enum value
// (rate_limit / authentication_failed / …). When callers pre-sanitize via
// sharederrors.SanitizeForUser, the original err is gone — fall back to
// "unknown" rather than guessing.
func (o *DefaultOrchestrator) emitError(out chan<- *contracts.EngineEvent, sessionID, content string, code ...string) {
	_ = code // legacy variadic retained for out-of-tree backward compat (v2.6.0)
	metadata := map[string]string{"error_code": "unknown"}
	out <- &contracts.EngineEvent{
		Type:      "error",
		Content:   content,
		SessionID: sessionID,
		Metadata:  metadata,
	}
}

// emitErrorWithErr is the V4 variant that carries the original error so the
// closed-set APIErrorCode can be extracted via sharederrors.Code(err).
// Use this in preference to emitError when the error is available.
//
// DM-20260628-001 (FR-13 + FR-15): also handles fallback-trigger observability
// — when APICodeRateLimit/APICodeServerError fires and consecutiveServerErrors
// reaches 2, logs fallback_trigger_candidate. If fallbackModel is empty,
// additionally logs fallback_model_set_but_not_yet_wired.
func (o *DefaultOrchestrator) emitErrorWithErr(out chan<- *contracts.EngineEvent, sessionID, content string, err error, code ...string) {
	var metadata map[string]string
	switch {
	case len(code) > 0 && code[0] != "":
		metadata = map[string]string{"error_code": code[0]}
	case err != nil:
		metadata = map[string]string{"error_code": sharederrors.Code(err).String()}
	default:
		metadata = map[string]string{"error_code": "unknown"}
	}
	if err != nil {
		o.observeFallbackTrigger(err)
	}
	out <- &contracts.EngineEvent{
		Type:      "error",
		Content:   content,
		SessionID: sessionID,
		Metadata:  metadata,
	}
}

// observeFallbackTrigger bumps consecutiveServerErrors on retryable API errors
// and logs fallback_trigger_candidate + fallback_model_set_but_not_yet_wired.
//
// DM-20260628-001 (FR-13, AC3 partial): full retry loop is P0-2 follow-up;
// this only emits observability markers for S4.
func (o *DefaultOrchestrator) observeFallbackTrigger(err error) {
	code := sharederrors.Code(err)
	switch code {
	case sharederrors.APICodeRateLimit, sharederrors.APICodeServerError:
		o.consecutiveServerErrors++
	default:
		// Non-retryable error: reset counter so the next consecutive pair
		// starts fresh from 0.
		o.consecutiveServerErrors = 0
		return
	}
	if o.consecutiveServerErrors < 2 {
		return
	}
	slog.Info("orchestrator: fallback_trigger_candidate",
		"consecutive", o.consecutiveServerErrors,
		"primary_code", code.String())
	if o.fallbackModel == "" {
		slog.Warn("orchestrator: fallback_model_set_but_not_yet_wired",
			"primary_code", code.String(),
			"note", "field reserved; full switch loop is P0-2 follow-up devrix-streaming-fallback")
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
	exitReason verify.ExitReason,
) {
	if model == "" {
		model = o.defaultModel
	}
	if maxContextTokens <= 0 {
		maxContextTokens = o.maxContextTokens
	}
	meta := map[string]string{
		"duration":            fmt.Sprintf("%d", time.Since(start).Milliseconds()),
		"usage":               fmt.Sprintf("%d", usageTokenTotal(usage)),
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
	systemPrompt := i18n.CompressSystemPrompt(i18n.ParseLanguage(o.promptLanguage))
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

