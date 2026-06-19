package turn

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// OrchestratorDeps holds the dependencies for the TurnOrchestrator.
type OrchestratorDeps struct {
	LLM              LLMInvoker
	Context          ContextPreparer
	Tools            ToolRoundExecutor
	Persist          SessionPersister
	MaxTurns         int
	DefaultModel     string
	MaxContextTokens int
	ObsBridge        *observability.Bridge
	FocusHint        FocusHintProvider
	ResolveAwait     ResolveAwaiter
}

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
}

// NewOrchestrator creates a DefaultOrchestrator.
func NewOrchestrator(deps OrchestratorDeps) *DefaultOrchestrator {
	if deps.MaxTurns <= 0 {
		deps.MaxTurns = 8
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
	}
}

// RunTurn executes the full prepare→llm→tools→persist loop (D7-S2-A06).
//
// State machine (design.md §3):
//
//	START → PREPARE → ROUTE+LLM → [TOOL_ROUND] → PERSIST → COMPLETE
//	                      ↑           │
//	                      └─ turns<max ┘
func (o *DefaultOrchestrator) RunTurn(ctx context.Context, req TurnRequest) (<-chan *contracts.EngineEvent, error) {
	if req.SessionID == "" {
		return nil, fmt.Errorf("turn: SessionID is required")
	}
	if req.MaxTurns <= 0 {
		req.MaxTurns = o.maxTurns
	}

	ch := make(chan *contracts.EngineEvent, 32)
	go func() {
		defer close(ch)
		ctx, turnSpan := o.startSpan(ctx, telemetry.OpD7_S2_Orchestration_Turn_Run, tracer.SpanKindInternal,
			tracer.Attribute{Key: "session_id", Value: req.SessionID},
			tracer.Attribute{Key: "turn.scope", Value: string(req.Scope)},
			tracer.Attribute{Key: "turn.max_turns", Value: fmt.Sprintf("%d", req.MaxTurns)},
		)
		defer endSpan(turnSpan)
		o.runLoop(ctx, req, ch)
	}()
	return ch, nil
}

// runLoop is the internal state machine: PREPARE → LLM ↔ TOOL_ROUND → PERSIST.
// Cross-domain calls: D7→D2 (prepare/tools/persist), D7→D3 (LLM invoke).
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
				tracer.Attribute{Key: "turn.scope", Value: string(req.Scope)},
			)
			prepared, err = o.context.Prepare(ctx, PrepareRequest{
				SessionID: req.SessionID,
				Message:   req.UserMessage,
				Mode:      req.Mode,
			})
			endSpan(prepSpan)
			if err != nil {
				o.emitError(out, req.SessionID, fmt.Sprintf("prepare failed: %v", err))
				return
			}
			tools = prepared.Tools
		}
		model = req.Model
		if model == "" && prepared.Model != "" {
			model = prepared.Model
		}
		maxContextTokens = prepared.MaxContextTokens
		if req.SkipPersist {
			persister = noopPersister{}
		}
	} else {
		// Step 1: PREPARE — D7 calls D2 for context assembly
		ctx, prepSpan := o.startSpan(ctx, telemetry.OpD2_S2_Context_Process, tracer.SpanKindInternal,
			tracer.Attribute{Key: "session_id", Value: req.SessionID},
			tracer.Attribute{Key: "context.phase", Value: "prepare"},
			tracer.Attribute{Key: "context.caller", Value: "d7"},
		)
		prepared, err = o.context.Prepare(ctx, PrepareRequest{
			SessionID: req.SessionID,
			Message:   req.UserMessage,
			Mode:      req.Mode,
		})
		endSpan(prepSpan)
		if err != nil {
			o.emitError(out, req.SessionID, fmt.Sprintf("prepare failed: %v", err))
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

	// Step 2+3: LLM↔Tool loop
	for turn := 0; turn < req.MaxTurns; turn++ {
		if err := ctx.Err(); err != nil {
			o.emitError(out, req.SessionID, fmt.Sprintf("turn cancelled: %v", err))
			return
		}

		turnCtx, turnSpan := o.startSpan(ctx, telemetry.OpD7_S2_Orchestration_Turn_Iteration, tracer.SpanKindInternal,
			tracer.Attribute{Key: "session_id", Value: req.SessionID},
			tracer.Attribute{Key: "turn.index", Value: fmt.Sprintf("%d", turn)},
		)

		// D7→D3 LLM invoke (D7-S2-A07)
		turnCtx, llmSpan := o.startSpan(turnCtx, telemetry.OpD7_S2_Orchestration_LLM_Invoke, tracer.SpanKindClient,
			tracer.Attribute{Key: "session_id", Value: req.SessionID},
			tracer.Attribute{Key: "turn.index", Value: fmt.Sprintf("%d", turn)},
			tracer.Attribute{Key: "llm.purpose", Value: "turn"},
		)
		var contentBuf strings.Builder
		var toolCalls []llmgateway.ToolCall
		var iterUsage llmgateway.TokenUsage
		var finishReason string

		streamRecoveryAttempts := 0
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
				o.emitError(out, req.SessionID, fmt.Sprintf("llm invoke failed: %v", err))
				return
			}

			contentBuf.Reset()
			toolCalls = nil
			finishReason = ""
			iterUsage = llmgateway.TokenUsage{}
			var partial partialStreamEmit

			for chunk := range chunkCh {
				if chunk.FinishReason != "" {
					finishReason = chunk.FinishReason
				}
				if chunk.Thinking != "" {
					partial.hadThinking = true
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

		finalText = contentBuf.String()
		toolCalls = dedupeToolCalls(toolCalls)

		// No tool calls → final response
		if len(toolCalls) == 0 {
			_, persistSpan := o.startSpan(turnCtx, telemetry.OpD2_S2_Context_Memory_Snapshot_Save, tracer.SpanKindInternal,
				tracer.Attribute{Key: "session_id", Value: req.SessionID},
				tracer.Attribute{Key: "context.caller", Value: "d7"},
			)
			_ = persister.PersistTurn(turnCtx, PersistRequest{
				SessionID: req.SessionID,
				Messages:  messages,
				TurnCount: turn + 1,
				Usage:     totalUsage,
				FinalText: finalText,
			})
			endSpan(persistSpan)
			endSpan(turnSpan)
			o.emitComplete(out, req.SessionID, start, totalUsage, lastPromptTokens, model, maxContextTokens, finalText)
			return
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
			tracer.Attribute{Key: "context.caller", Value: "d7"},
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
			o.emitError(out, req.SessionID, fmt.Sprintf("tool round failed: %v", err))
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

		// Build assistant tool-call message and tool result messages for the next turn.
		messages = append(messages, buildAssistantToolCallMsg(req.SessionID, toolCalls, finalText))
		for _, r := range toolResult.Results {
			messages = append(messages, buildToolResultMsg(req.SessionID, r))
		}

		// NOTE: finalText is intentionally NOT cleared here — it must survive to the
		// emitComplete call after the for-loop exits, so MaxTurns-exceeded emits
		// the last iteration's LLM text (otherwise the IM adapter sees an empty
		// complete event and the user gets no conclusion card).
		endSpan(turnSpan)
	}

	// Max turns reached
	_, persistSpan := o.startSpan(ctx, telemetry.OpD2_S2_Context_Memory_Snapshot_Save, tracer.SpanKindInternal,
		tracer.Attribute{Key: "session_id", Value: req.SessionID},
		tracer.Attribute{Key: "context.caller", Value: "d7"},
	)
	_ = persister.PersistTurn(ctx, PersistRequest{
		SessionID: req.SessionID,
		Messages:  messages,
		TurnCount: req.MaxTurns,
		Usage:     totalUsage,
		FinalText: finalText,
	})
	endSpan(persistSpan)
	o.emitComplete(out, req.SessionID, start, totalUsage, lastPromptTokens, model, maxContextTokens, finalText)
}

func (o *DefaultOrchestrator) emitError(out chan<- *contracts.EngineEvent, sessionID, content string) {
	out <- &contracts.EngineEvent{
		Type:      "error",
		Content:   content,
		SessionID: sessionID,
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
	}
	if model != "" {
		meta["model"] = model
	}
	if pct := contracts.ComputeCtxPct(lastPromptTokens, maxContextTokens); pct > 0 {
		meta["ctx_pct"] = fmt.Sprintf("%d", pct)
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
		if summary := summaryBuilder.String(); summary != "" {
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
