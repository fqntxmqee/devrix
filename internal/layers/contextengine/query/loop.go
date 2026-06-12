package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/attachments"
	"github.com/devrix/devrix/internal/layers/contextengine/conversation"
	"github.com/devrix/devrix/internal/layers/contextengine/queue"
	"github.com/devrix/devrix/internal/layers/contextengine/usercontext"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// Loop runs the Claude Code-aligned query loop (while tool_use continue).
type Loop struct {
	LLM          LLMCaller
	Tools        ToolExecutor
	Permission   PermissionChecker
	Compress     CompressFunc
	Attachments  *attachments.Registry
	UserContext  *usercontext.Provider
	Hooks           LoopHooks
	PrependUC       func(msgs []types.Message, uc map[string]string) []types.Message
	WrapToolContext func(ctx context.Context, sc *types.SessionContext) context.Context
	// WrapToolStreamContext wraps the tool execution context with a stream emitter
	// so agent tools (call_claude-code etc.) can stream mid-execution events.
	WrapToolStreamContext func(ctx context.Context, emit EmitFunc, sessionID, toolName string) context.Context
	SessionQueue    *queue.SessionQueue
	StreamingTools  bool
}

// Run executes the loop until no tool calls, max turns, cancel, or hook stop.
func (l *Loop) Run(
	ctx context.Context,
	sc *types.SessionContext,
	params Params,
	emit EmitFunc,
) (*Result, error) {
	if l.LLM == nil {
		return nil, fmt.Errorf("query loop: LLM is nil")
	}
	prepend := l.PrependUC
	if prepend == nil && l.UserContext != nil {
		prepend = usercontext.PrependForAPI
	}

	messages := append([]types.Message(nil), params.Messages...)
	messages = conversation.StripSystem(messages)

	var (
		assistantText     string
		usage             TokenUsage
		allToolRecords    []types.ToolCallRecord
		toolRoundResults  []ToolRoundResult
	)

	maxTurns := params.MaxTurns
	turn := 0

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if maxTurns > 0 && turn >= maxTurns {
			break
		}

		if l.Compress != nil {
			compressed, err := l.Compress(ctx, messages)
			if err != nil {
				return nil, err
			}
			messages = compressed
		}

		if l.Attachments != nil && sc != nil {
			payloads := l.Attachments.Collect(ctx, sc, messages, turn)
			messages = append(messages, attachments.Render(payloads)...)
		}

		if l.SessionQueue != nil && sc != nil {
			mainThread := sc.AgentID == ""
			drained := l.SessionQueue.Drain(sc.SessionID, sc.AgentID, mainThread)
			messages = append(messages, queue.RenderNotifications(sc.SessionID, drained)...)
		}

		uc := params.UserContext
		if l.UserContext != nil && sc != nil {
			uc = l.UserContext.Get(ctx, sc)
			if sc.AgentID != "" {
				uc = usercontext.OmitClaudeMd(uc)
			}
		}

		apiMessages := messages
		if prepend != nil && len(uc) > 0 {
			apiMessages = prepend(messages, uc)
		}

		chunks, err := l.LLM.Call(ctx, LLMRequest{
			Model:        sc.Model,
			SystemPrompt: params.SystemPrompt,
			Messages:     apiMessages,
			Tools:        params.Tools,
		})
		if err != nil {
			if len(toolRoundResults) > 0 {
				break
			}
			return nil, err
		}

		var pending []ToolCall
		assistantText = ""
		var iterUsage TokenUsage
		for chunk := range chunks {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if chunk.Thinking != "" && emit != nil {
				emitThinking(emit, sc.SessionID, chunk.Thinking)
			}
			if chunk.Content != "" {
				assistantText += chunk.Content
				if emit != nil {
					emitText(emit, sc.SessionID, chunk.Content, false)
				}
			}
			if len(chunk.ToolCalls) > 0 {
				pending = chunk.ToolCalls
			}
			// Usage 可能出现在 finish_reason 帧或独立 usage 帧，亦可能由
			// [DONE] 哨兵帧带回（参见 sse_parser 的 lastUsage 处理）。
			// 用"最后一次非零值覆盖"语义，避免被多次累加。
			if chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0 {
				iterUsage = TokenUsage{
					PromptTokens:     chunk.Usage.PromptTokens,
					CompletionTokens: chunk.Usage.CompletionTokens,
				}
			}
		}
		usage.PromptTokens += iterUsage.PromptTokens
		usage.CompletionTokens += iterUsage.CompletionTokens

		if len(pending) == 0 {
			if l.Hooks.BeforeComplete != nil {
				stop, err := l.Hooks.BeforeComplete(ctx, sc)
				if err != nil {
					return nil, err
				}
				if stop {
					break
				}
			}
			break
		}

		refs := make([]conversation.ToolCallRef, len(pending))
		for i, tc := range pending {
			refs[i] = conversation.ToolCallRef{ID: tc.ID, Name: tc.Name, Input: tc.Input}
		}
		refs = conversation.DedupeToolCalls(refs)

		messages = append(messages, conversation.BuildAssistantToolCallsMessage(sc.SessionID, assistantText, refs))

		toolRoundResults = toolRoundResults[:0]
		if l.StreamingTools && len(refs) > 1 {
			exec := &StreamingToolExecutor{
				Tools:           l.Tools,
				Permission:      l.Permission,
				WrapToolContext: l.WrapToolContext,
				Emit:            emit,
				WrapToolStreamEmitter: l.WrapToolStreamContext,
			}
			batchRefs := make([]BatchToolRef, len(refs))
			for i, ref := range refs {
				batchRefs[i] = BatchToolRef{ID: ref.ID, Name: ref.Name, Input: ref.Input}
			}
			batch := exec.ExecuteBatch(ctx, sc, batchRefs)
			for i, res := range batch {
				ref := refs[i]
				content := res.Output
				if res.Error != "" {
					content = res.Error
				}
				if emit != nil {
					emitToolCall(emit, sc, ref)
					emitToolResult(emit, sc.SessionID, ref.Name, content, res.Error)
				}
				messages = append(messages, conversation.BuildToolResultMessage(sc.SessionID, ref.ID, content))
				allToolRecords = append(allToolRecords, types.ToolCallRecord{
					CallID: ref.ID, ToolName: ref.Name, Input: ref.Input, Output: res.Output, Error: res.Error,
				})
				toolRoundResults = append(toolRoundResults, ToolRoundResult{Name: ref.Name, Output: res.Output, Error: res.Error})
			}
		} else {
			for _, ref := range refs {
				if l.Permission != nil && !l.Permission.Request(ctx, sc.SessionID, ref.Name, ref.Input) {
					return nil, fmt.Errorf("permission denied for tool %s", ref.Name)
				}
				if emit != nil {
					emitToolCall(emit, sc, ref)
				}
				out, errMsg, execErr := "", "", error(nil)
				if l.Tools != nil {
					toolCtx := ctx
					if l.WrapToolContext != nil {
						toolCtx = l.WrapToolContext(ctx, sc)
					}
					if emit != nil && l.WrapToolStreamContext != nil {
						toolCtx = l.WrapToolStreamContext(toolCtx, emit, sc.SessionID, ref.Name)
					}
					out, errMsg, execErr = l.Tools.Execute(toolCtx, ToolCall{ID: ref.ID, Name: ref.Name, Input: ref.Input})
				}
				if execErr != nil && errMsg == "" {
					errMsg = execErr.Error()
				}
				content := out
				if errMsg != "" {
					content = errMsg
				}
				if emit != nil {
					emitToolResult(emit, sc.SessionID, ref.Name, content, errMsg)
				}
				messages = append(messages, conversation.BuildToolResultMessage(sc.SessionID, ref.ID, content))
				rec := types.ToolCallRecord{
					CallID: ref.ID, ToolName: ref.Name, Input: ref.Input,
					Output: out, Error: errMsg,
				}
				allToolRecords = append(allToolRecords, rec)
				toolRoundResults = append(toolRoundResults, ToolRoundResult{Name: ref.Name, Output: out, Error: errMsg})
			}
		}

		if l.Hooks.AfterToolRound != nil {
			stop, err := l.Hooks.AfterToolRound(ctx, sc, toolRoundResults)
			if err != nil {
				return nil, err
			}
			if stop {
				break
			}
		}

		assistantText = ""
		turn++
		if sc != nil {
			sc.QueryDepth++
		}
	}

	assistantText = strings.TrimSpace(assistantText)
	var outMsgs []types.Message
	if assistantText != "" {
		outMsgs = append(outMsgs, types.Message{Role: types.MessageRoleAssistant, Content: assistantText, SessionID: sc.SessionID})
	}

	return &Result{
		Messages:        outMsgs,
		AssistantText:   assistantText,
		Usage:           usage,
		TurnCount:       turn,
		ToolCallHistory: allToolRecords,
	}, nil
}

func emitThinking(emit EmitFunc, sessionID, content string) {
	emit(&contracts.EngineEvent{Type: "thinking", Content: content, SessionID: sessionID})
}

func emitText(emit EmitFunc, sessionID, content string, complete bool) {
	meta := map[string]string{"is_complete": "false"}
	if complete {
		meta["is_complete"] = "true"
	}
	emit(&contracts.EngineEvent{Type: "text", Content: content, SessionID: sessionID, Metadata: meta})
}

func emitToolCall(emit EmitFunc, sc *types.SessionContext, ref conversation.ToolCallRef) {
	emit(&contracts.EngineEvent{
		Type: "tool_call", ToolName: ref.Name, ToolInput: ref.Input, SessionID: sc.SessionID,
		Metadata: map[string]string{"tool_name": ref.Name, "input": ref.Input},
	})
}

func emitToolResult(emit EmitFunc, sessionID, name, content, errMsg string) {
	emit(&contracts.EngineEvent{
		Type: "tool_result", Content: content, ToolName: name, SessionID: sessionID,
		Metadata: map[string]string{"tool_name": name, "error": errMsg},
	})
}