package turn

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// OrchestratorDeps holds the dependencies for the TurnOrchestrator.
type OrchestratorDeps struct {
	LLM       LLMInvoker
	Context   ContextPreparer
	Tools     ToolRoundExecutor
	Persist   SessionPersister
	MaxTurns  int
	ObsBridge *observability.Bridge
}

// DefaultOrchestrator implements TurnOrchestrator with the canonical
// prepare→llm→tools→persist state machine (design.md §3).
type DefaultOrchestrator struct {
	llm       LLMInvoker
	context   ContextPreparer
	tools     ToolRoundExecutor
	persist   SessionPersister
	maxTurns  int
	obsBridge *observability.Bridge
}

// NewOrchestrator creates a DefaultOrchestrator.
func NewOrchestrator(deps OrchestratorDeps) *DefaultOrchestrator {
	if deps.MaxTurns <= 0 {
		deps.MaxTurns = 8
	}
	return &DefaultOrchestrator{
		llm:       deps.LLM,
		context:   deps.Context,
		tools:     deps.Tools,
		persist:   deps.Persist,
		maxTurns:  deps.MaxTurns,
		obsBridge: deps.ObsBridge,
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
	// Step 1: PREPARE — D7 calls D2 for context assembly
	ctx, prepSpan := o.startSpan(ctx, telemetry.OpD2_S2_Context_Process, tracer.SpanKindInternal,
		tracer.Attribute{Key: "session_id", Value: req.SessionID},
		tracer.Attribute{Key: "context.phase", Value: "prepare"},
		tracer.Attribute{Key: "context.caller", Value: "d7"},
	)
	prepared, err := o.context.Prepare(ctx, PrepareRequest{
		SessionID: req.SessionID,
		Message:   req.UserMessage,
	})
	endSpan(prepSpan)
	if err != nil {
		o.emitError(out, req.SessionID, fmt.Sprintf("prepare failed: %v", err))
		return
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

	// Messages accumulate across turns. Start with prepared context + user message.
	messages := make([]types.Message, 0, len(prepared.Messages)+1)
	messages = append(messages, prepared.Messages...)
	messages = append(messages, req.UserMessage)

	tools := prepared.Tools
	var totalUsage llmgateway.TokenUsage
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
		chunkCh, err := o.llm.InvokeStream(turnCtx, LLMInvokeRequest{
			SessionID:    req.SessionID,
			SystemPrompt: mergeSystemPrompt(prepared.SystemPrompt, req.SystemPrompt),
			Messages:     messages,
			Tools:        tools,
		})
		if err != nil {
			endSpanWithError(llmSpan, err)
			endSpan(turnSpan)
			o.emitError(out, req.SessionID, fmt.Sprintf("llm invoke failed: %v", err))
			return
		}

		// Process stream chunks
		var contentBuf strings.Builder
		var toolCalls []llmgateway.ToolCall

		for chunk := range chunkCh {
			if chunk.Thinking != "" {
				out <- &contracts.EngineEvent{
					Type:      "thinking",
					Content:   chunk.Thinking,
					SessionID: req.SessionID,
				}
			}
			if chunk.Content != "" {
				contentBuf.WriteString(chunk.Content)
				out <- &contracts.EngineEvent{
					Type:      "text",
					Content:   chunk.Content,
					SessionID: req.SessionID,
				}
			}
			if len(chunk.ToolCalls) > 0 {
				toolCalls = append(toolCalls, chunk.ToolCalls...)
			}
			if chunk.Done {
				totalUsage.PromptTokens += chunk.Usage.PromptTokens
				totalUsage.CompletionTokens += chunk.Usage.CompletionTokens
			}
		}
		endSpan(llmSpan)

		finalText = contentBuf.String()

		// No tool calls → final response
		if len(toolCalls) == 0 {
			_, persistSpan := o.startSpan(turnCtx, telemetry.OpD2_S2_Context_Memory_Snapshot_Save, tracer.SpanKindInternal,
				tracer.Attribute{Key: "session_id", Value: req.SessionID},
				tracer.Attribute{Key: "context.caller", Value: "d7"},
			)
			_ = o.persist.PersistTurn(turnCtx, PersistRequest{
				SessionID:  req.SessionID,
				Messages:   messages,
				TurnCount:  turn + 1,
				Usage:      totalUsage,
				FinalText:  finalText,
			})
			endSpan(persistSpan)
			endSpan(turnSpan)
			out <- &contracts.EngineEvent{
				Type:      "complete",
				SessionID: req.SessionID,
			}
			return
		}

		// Emit tool call events
		for _, tc := range toolCalls {
			out <- &contracts.EngineEvent{
				Type:      "tool_call",
				ToolName:  tc.Name,
				ToolInput: tc.Input,
				SessionID: req.SessionID,
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
			out <- &contracts.EngineEvent{
				Type:      "tool_result",
				Content:   content,
				SessionID: req.SessionID,
			}
		}

		// Build assistant tool-call message and tool result messages for the next turn.
		messages = append(messages, buildAssistantToolCallMsg(req.SessionID, toolCalls, finalText))
		for _, r := range toolResult.Results {
			messages = append(messages, buildToolResultMsg(req.SessionID, r))
		}

		finalText = ""
		endSpan(turnSpan)
	}

	// Max turns reached
	_, persistSpan := o.startSpan(ctx, telemetry.OpD2_S2_Context_Memory_Snapshot_Save, tracer.SpanKindInternal,
		tracer.Attribute{Key: "session_id", Value: req.SessionID},
		tracer.Attribute{Key: "context.caller", Value: "d7"},
	)
	_ = o.persist.PersistTurn(ctx, PersistRequest{
		SessionID:  req.SessionID,
		Messages:   messages,
		TurnCount:  req.MaxTurns,
		Usage:      totalUsage,
		FinalText:  finalText,
	})
	endSpan(persistSpan)
	out <- &contracts.EngineEvent{
		Type:      "complete",
		SessionID: req.SessionID,
	}
}

func (o *DefaultOrchestrator) emitError(out chan<- *contracts.EngineEvent, sessionID, content string) {
	out <- &contracts.EngineEvent{
		Type:      "error",
		Content:   content,
		SessionID: sessionID,
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
	Summary      string
	Degradation  CompressDegradation
	TruncatedTo  int // number of messages kept (only for Truncation)
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
