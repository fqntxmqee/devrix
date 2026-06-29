package sessionorchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/audit"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/persist"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/token"
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/executionflow/verify"
	"github.com/devrix/devrix/internal/layers/orchestration/hardening"
	obsruntime "github.com/devrix/devrix/internal/layers/observability/configure/runtime"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/contracts"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

// prepareContext runs the D2 Prepare call (with nested vs top-level
// branching), applies focus hint / resolve-await enrichment, and runs
// CompressHint summarization. Returns the loop state ready for the
// LLM↔tool loop. Emits an error event and returns a non-nil error on
// prepare failure (caller should return immediately).
//
// Split out from turn_orchestrator.go in DM-20260629-001 PR-3 (turn-fn-split
// second batch) along with runLLMStream + executeToolRound + the message
// builders and token-audit helpers; mapped to D7-S2-A07 (SessionTurnLoop
// prepare phase) in t-registry.
func (o *DefaultOrchestrator) prepareContext(ctx context.Context, req TurnRequest, out chan<- *contracts.EngineEvent) (*runLoopState, error) {
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
				return nil, err
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
			return nil, err
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

	return &runLoopState{
		systemPrompt:         systemPrompt,
		messages:             messages,
		tools:                tools,
		model:                model,
		maxContextTokens:     maxContextTokens,
		persister:            persister,
		nested:               nested,
		exitReason:           verify.ExitReasonNatural,
		recentToolSignatures: make([]string, 0, repeatedToolLookback),
		userContextPrepend:   prepared.UserContextPrepend,
	}, nil
}

// runLLMStream invokes the LLM, processes the streaming chunks, and
// runs the max-output-token recovery loop (up to
// maxOutputTokenRecoveryAttempts retries). Emits `thinking` / `text`
// events to the UI. Returns the turn's accumulated text, tool calls,
// and final usage. On invoke failure, emits an error event and returns
// a non-nil error.
//
// st.lastThinkingTail is updated with the most recent non-empty turn's
// thinking content (post-strip), used as the finalText fallback in
// finalizeLoop.
//
// Mapped to D7-S2-A08 (LLM Invoke + ReAct) in t-registry.
func (o *DefaultOrchestrator) runLLMStream(
	turnCtx context.Context,
	req TurnRequest,
	st *runLoopState,
	out chan<- *contracts.EngineEvent,
) (text string, toolCalls []llmgateway.ToolCall, usage llmgateway.TokenUsage, err error) {
	_, llmSpan := o.startSpan(turnCtx, telemetry.OpD7_S2_Orchestration_LLM_Invoke, tracer.SpanKindClient,
		tracer.Attribute{Key: "session_id", Value: req.SessionID},
		tracer.Attribute{Key: "turn.index", Value: fmt.Sprintf("%d", st.turnCount)},
		tracer.Attribute{Key: "llm.purpose", Value: "turn"},
		tracer.Attribute{Key: "llm.model", Value: st.model},
		tracer.Attribute{Key: "llm.pre_message_count", Value: fmt.Sprintf("%d", len(st.messages))},
		tracer.Attribute{Key: "llm.pre_tool_count", Value: fmt.Sprintf("%d", len(st.tools))},
		tracer.Attribute{Key: "llm.max_context_tokens", Value: fmt.Sprintf("%d", st.maxContextTokens)},
		tracer.Attribute{Key: "llm.stream_recovery_attempts", Value: "0"},
	)

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

	var contentBuf strings.Builder
	// turnThinking accumulates all thinking chunks emitted in this turn.
	// After the inner stream loop, the most recent non-empty turn's
	// turnThinking.String() is preserved as lastThinkingTail for the
	// finalText fallback below (see emitComplete call sites).
	var turnThinking strings.Builder
	var finishReason string
	var iterUsage llmgateway.TokenUsage

streamRecoveryLoop:
	for {
		apiMessages := messagesForLLMInvoke(st.messages, st.userContextPrepend)
		chunkCh, invokeErr := o.invokeStreamWithRecovery(turnCtx, req, LLMInvokeRequest{
			SessionID:    req.SessionID,
			SystemPrompt: st.systemPrompt,
			Messages:     apiMessages,
			Tools:        st.tools,
		})
		if invokeErr != nil {
			endSpanWithError(llmSpan, invokeErr)
			o.emitError(out, req.SessionID,
				sharederrors.SanitizeForUser(fmt.Errorf("llm invoke failed: %w", invokeErr)),
				sharederrors.ErrorCode(invokeErr),
			)
			return "", nil, llmgateway.TokenUsage{}, invokeErr
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

		if !hardening.NeedsMaxOutputTokenRecovery(finishReason) {
			break streamRecoveryLoop
		}
		if streamRecoveryAttempts >= maxOutputTokenRecoveryAttempts {
			break streamRecoveryLoop
		}
		emitStreamRecoveryTombstones(out, req.SessionID, partial)
		st.messages = append(st.messages, types.Message{
			SessionID: req.SessionID,
			Role:      types.MessageRoleUser,
			Content:   hardening.MaxOutputTokensRecoveryMessage,
		})
		streamRecoveryAttempts++
	}
	usage = iterUsage
	endSpan(llmSpan)

	// Preserve this turn's accumulated thinking for the finalizeLoop
	// fallback. Stash as lastThinkingTail so the most recent non-empty
	// thinking is always the fallback source.
	if t := strings.TrimSpace(turnThinking.String()); t != "" {
		st.lastThinkingTail.Reset()
		st.lastThinkingTail.WriteString(t)
	}
	text = contentBuf.String()
	return text, toolCalls, usage, nil
}

// executeToolRound emits the per-tool `tool_call` events, runs the
// tool round via the D2 executor, and emits `tool_result` events. On
// executor failure, emits an error event and returns a non-nil error.
//
// Mapped to D7-S2-A08 (Tool Round) in t-registry.
func (o *DefaultOrchestrator) executeToolRound(
	turnCtx context.Context,
	req TurnRequest,
	st *runLoopState,
	toolCalls []llmgateway.ToolCall,
	out chan<- *contracts.EngineEvent,
) (ToolRoundResult, error) {
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
		tracer.Attribute{Key: "turn.index", Value: fmt.Sprintf("%d", st.turnCount)},
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
		o.emitError(out, req.SessionID,
			sharederrors.SanitizeForUser(fmt.Errorf("tool round failed: %w", err)),
			sharederrors.ErrorCode(err),
		)
		return ToolRoundResult{}, err
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
	return toolResult, nil
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
// and surfaces verify.ExitReasonRepeatedTool on the final complete event.
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