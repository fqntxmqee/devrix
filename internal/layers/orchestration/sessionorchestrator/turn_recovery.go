package sessionorchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/executionflow/verify"
	obsruntime "github.com/devrix/devrix/internal/layers/observability/configure/runtime"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/contracts"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/textutil"
	"github.com/devrix/devrix/internal/shared/types"
)

// finalizeLoop persists the turn and emits the terminal `complete`
// event. The aborted_* paths already returned above with their own
// error event; everything else (natural / max_turns / repeated_tool /
// tool_failure / token_diminishing) reaches here and emits a single
// terminal `complete` whose Metadata carries exit_reason.
//
// Split out from turn_orchestrator.go in DM-20260629-001 PR-3
// (turn-fn-split second batch, recovery half) along with resolveFinalText,
// emitError{,WithErr}, observeFallbackTrigger, emitComplete, runCompress,
// CompressDegradation, and compressResult; mapped to D7-S2-A09
// (SessionTurnLoop finalize phase) in t-registry.
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
	// DM-20260630-011 (devrix-session-conclusion-completeness) AC1:
	// classify the resolved summary BEFORE emit so D7 propagates
	// summary_quality to D1 IM adapters. Emit always (even on valid kind)
	// so dashboards can filter by "interesting" via != valid.
	summaryQuality := EmitLastTextQuality(ctx, req.SessionID, resolvedSummary, string(st.exitReason))
	// DM-20260630-011 follow-up: also classify resolvedFinal (the fallback
	// Content that D1 EmitComplete uses when summary_quality is too_short /
	// inconclusive). When the LLM ends mid-tool-call with a transitional
	// phrase like "Now let me look at..." (sess_1782826968112_7000 — 82 chars),
	// BOTH summary and final classify as too_short, so the fallback chain
	// would forward the junk to Feishu. Emitting final_quality lets D1 detect
	// this and emit a clear "task incomplete" message instead.
	finalQuality := ClassifyLastTextQuality(resolvedFinal)
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
		resolvedFinal, resolvedSummary, summaryQuality.Kind, finalQuality.Kind, st.exitReason)
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

// emitComplete emits the terminal `complete` event with usage stats,
// timing, model, ctx_pct, and exit_reason. summary (when non-empty) is
// surfaced on meta["summary"] for IM "任务总结" cards; finalText is the
// full multi-turn transcript on Content.
//
// summaryQuality (DM-20260630-011) drives meta["summary_quality"] so D1
// IM adapters can render explicit "任务结论不完整" UI when the LLM last
// text was too_short / inconclusive, instead of showing the raw artifact.
//
// usageTokenTotal helper (defined in turn_invoke.go) is reused across the
// package.
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
	summaryQuality SummaryQualityKind,
	finalQuality SummaryQualityKind,
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
	// DM-20260630-011 AC1: surface summary_quality so D1 EmitComplete can
	// decide whether to fall back to Content / stats (when too_short /
	// inconclusive) or just keep the summary (when valid / thin). Always
	// emitted (even when valid) so dashboards / D6 Evolution have a stable
	// Jaeger filter dimension across sessions.
	meta["summary_quality"] = string(summaryQuality)
	// DM-20260630-011 follow-up: surface final_quality so D1 EmitComplete can
	// detect when both summary AND fallback Content are bad (LLM ended
	// mid-tool-call with transitional text), and emit a clear "task
	// incomplete" message instead of forwarding the junk text.
	meta["final_quality"] = string(finalQuality)
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