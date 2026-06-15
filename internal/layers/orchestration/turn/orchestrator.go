package turn

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// OrchestratorDeps holds the dependencies for the TurnOrchestrator.
type OrchestratorDeps struct {
	LLM      LLMInvoker
	Context  ContextPreparer
	Tools    ToolRoundExecutor
	Persist  SessionPersister
	MaxTurns int
}

// DefaultOrchestrator implements TurnOrchestrator with the canonical
// prepare→llm→tools→persist state machine (design.md §3).
type DefaultOrchestrator struct {
	llm      LLMInvoker
	context  ContextPreparer
	tools    ToolRoundExecutor
	persist  SessionPersister
	maxTurns int
}

// NewOrchestrator creates a DefaultOrchestrator.
func NewOrchestrator(deps OrchestratorDeps) *DefaultOrchestrator {
	if deps.MaxTurns <= 0 {
		deps.MaxTurns = 8
	}
	return &DefaultOrchestrator{
		llm:      deps.LLM,
		context:  deps.Context,
		tools:    deps.Tools,
		persist:  deps.Persist,
		maxTurns: deps.MaxTurns,
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
		o.runLoop(ctx, req, ch)
	}()
	return ch, nil
}

// runLoop is the internal state machine: PREPARE → LLM ↔ TOOL_ROUND → PERSIST.
func (o *DefaultOrchestrator) runLoop(ctx context.Context, req TurnRequest, out chan<- *contracts.EngineEvent) {
	// Step 1: PREPARE — ask D2 for context assembly
	prepared, err := o.context.Prepare(ctx, PrepareRequest{
		SessionID: req.SessionID,
		Message:   req.UserMessage,
	})
	if err != nil {
		o.emitError(out, req.SessionID, fmt.Sprintf("prepare failed: %v", err))
		return
	}

	// D-e: handle CompressHint — D7 calls D3 for summarization.
	if prepared.CompressHint != nil {
		summary, err := o.runCompress(ctx, req, prepared.CompressHint)
		if err != nil {
			o.emitError(out, req.SessionID, fmt.Sprintf("compress failed: %v", err))
			return
		}
		// Replace messages with the summary and re-prepare.
		prepared.Messages = []types.Message{{
			SessionID: req.SessionID,
			Role:      types.MessageRoleSystem,
			Content:   summary,
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

		// Invoke LLM (D7→D3 via GatewayInvoker)
		chunkCh, err := o.llm.InvokeStream(ctx, LLMInvokeRequest{
			SessionID:    req.SessionID,
			SystemPrompt: prepared.SystemPrompt,
			Messages:     messages,
			Tools:        tools,
		})
		if err != nil {
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

		finalText = contentBuf.String()

		// No tool calls → final response
		if len(toolCalls) == 0 {
			_ = o.persist.PersistTurn(ctx, PersistRequest{
				SessionID:  req.SessionID,
				Messages:   messages,
				TurnCount:  turn + 1,
				Usage:      totalUsage,
				FinalText:  finalText,
			})
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

		// Execute tool round (D7→D2)
		toolResult, err := o.tools.ExecuteRound(ctx, ToolRoundRequest{
			SessionID: req.SessionID,
			ToolCalls: toolCalls,
		})
		if err != nil {
			o.emitError(out, req.SessionID, fmt.Sprintf("tool round failed: %v", err))
			return
		}

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
	}

	// Max turns reached
	_ = o.persist.PersistTurn(ctx, PersistRequest{
		SessionID:  req.SessionID,
		Messages:   messages,
		TurnCount:  req.MaxTurns,
		Usage:      totalUsage,
		FinalText:  finalText,
	})
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

// runCompress handles CompressHint by calling D3 for summarization (D-e).
func (o *DefaultOrchestrator) runCompress(ctx context.Context, req TurnRequest, hint *CompressHint) (string, error) {
	if hint == nil || len(hint.MessagesToSummarize) == 0 {
		return "", fmt.Errorf("compress: empty hint")
	}

	// Build a summarization prompt.
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
	if err != nil {
		return "", fmt.Errorf("compress invoke: %w", err)
	}

	var summaryBuilder strings.Builder
	for chunk := range chunkCh {
		if chunk.Content != "" {
			summaryBuilder.WriteString(chunk.Content)
		}
	}
	return summaryBuilder.String(), nil
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
