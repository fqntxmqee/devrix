package contextengine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/layers/contextengine/conversation"
	"github.com/devrix/devrix/internal/layers/contextengine/query"
	"github.com/devrix/devrix/internal/layers/observability/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/tracer"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/textutil"
	"github.com/devrix/devrix/internal/shared/types"
)

func (e *PEVEngine) runViaQueryLoop(
	ctx context.Context,
	sc *types.SessionContext,
	view []types.Message,
	systemPrompt string,
	emit func(*gateway.EngineEvent),
	emitComplete bool,
) (*PEVRunResult, error) {
	start := time.Now()
	// DM-20260611-008：query loop 路径补充 runSpan，让 D5 观测层能定位
	// token 链路（与 legacy PEV 路径的 pev.run 对齐）。
	ctx, queryRunSpan := e.startSpan(ctx, telemetry.OpContextPEVRun, tracer.SpanKindInternal,
		tracer.Attribute{Key: "pev.path", Value: "query_loop"},
	)
	if queryRunSpan != nil {
		defer queryRunSpan.End()
	}
	toolSchemas := resolveVisibleTools(ctx, e.toolsReg, sc)
	toolSchemas = FilterToolsByPermissionMode(sc.PermissionMode, toolSchemas, sc.PlanFilePath)
	toolSchemas = FilterToolsForAgentRole(sc, toolSchemas)

	maxTurns := e.queryLoop.MaxTurns
	if maxTurns == 0 && e.cfg.MaxIterations > 0 {
		// Legacy fallback when max_turns unset but PEV max_iterations configured.
		maxTurns = e.cfg.MaxIterations
	}

	loop := &query.Loop{
		LLM:             &llmCaller{llm: e.llm},
		Tools:           &toolExecutor{tools: e.tools, toolsReg: e.toolsReg},
		Permission:      &permChecker{gate: e.permission, reg: e.toolsReg},
		Attachments:     e.queryLoop.Attachments,
		UserContext:     e.queryLoop.UserContext,
		WrapToolContext: func(ctx context.Context, sc *types.SessionContext) context.Context {
			return ToolContextWithGate(ctx, sc, e.permission)
		},
		WrapToolStreamContext: func(ctx context.Context, emit query.EmitFunc, sessionID, toolName string) context.Context {
			return withToolStreamEmitter(ctx, emit, sessionID, toolName)
		},
		SessionQueue:    e.queryLoop.SessionQueue,
		StreamingTools:  e.queryLoop.StreamingTools,
		Hooks: query.LoopHooks{
			AfterToolRound: func(ctx context.Context, sc *types.SessionContext, results []query.ToolRoundResult) (bool, error) {
				if isSingleRoundVerifyMode(e.cfg.VerifyMode) {
					return false, nil
				}
				var toolResults []ToolResult
				for _, r := range results {
					toolResults = append(toolResults, ToolResult{Output: r.Output, Error: r.Error})
				}
				sc.PEVState.Phase = types.PEVPhaseVerify
				vr := e.verify(ctx, sc, toolResults)
				sc.PEVState.VerifyResult = vr
				return vr.Passed, nil
			},
		},
	}
	if e.queryLoop.Compress && e.queryLoop.CompressFn != nil {
		loop.Compress = e.queryLoop.CompressFn(sc.SessionID)
	}

	messages := conversation.StripSystem(view)
	res, err := loop.Run(ctx, sc, query.Params{
		SystemPrompt: systemPrompt,
		Messages:     messages,
		Tools:        toolSchemasToQuery(toolSchemas),
		MaxTurns:     maxTurns,
	}, emit)
	if err != nil {
		return nil, err
	}

	assistantText := res.AssistantText
	if assistantText == "" && len(res.ToolCallHistory) > 0 {
		var toolResults []ToolResult
		for _, rec := range res.ToolCallHistory {
			toolResults = append(toolResults, ToolResult{Output: rec.Output, Error: rec.Error})
		}
		synth, synthToolCalls, synthUsage := e.runToolSynthesis(ctx, sc, view, systemPrompt, "", toolResults, emit)
		assistantText = synth
		res.Usage.PromptTokens += synthUsage.PromptTokens
		res.Usage.CompletionTokens += synthUsage.CompletionTokens

		// YOLO continuation: execute tool calls returned from synthesis
		if e.isYOLO() && len(synthToolCalls) > 0 {
			var contResults []ToolResult
			for _, tc := range synthToolCalls {
				risk := types.RiskLevelLow
				if e.toolsReg != nil {
					risk = e.toolsReg.RiskLevel(tc.Name)
				}
				if e.permission != nil && !e.permission.Request(ctx, sc.SessionID, tc.Name, tc.Input, risk) {
					continue
				}
				emit(&gateway.EngineEvent{
					Type: "tool_call", ToolName: tc.Name, ToolInput: tc.Input, SessionID: sc.SessionID,
					Metadata: map[string]string{"tool_name": tc.Name, "input": tc.Input},
				})
				toolCtx := withToolStreamEmitter(ctx, emit, sc.SessionID, tc.Name)
				result, err := e.tools.Execute(toolCtx, tc)
				var output, errMsg string
				if err != nil {
					errMsg = err.Error()
				} else if result != nil {
					output = result.Output
					errMsg = result.Error
				}
				content := output
				if errMsg != "" {
					content = errMsg
				}
				emit(&gateway.EngineEvent{
					Type: "tool_result", Content: content, ToolName: tc.Name, SessionID: sc.SessionID,
					Metadata: map[string]string{"tool_name": tc.Name, "error": errMsg},
				})
				contResults = append(contResults, ToolResult{Output: output, Error: errMsg})
				res.ToolCallHistory = append(res.ToolCallHistory, types.ToolCallRecord{
					CallID: tc.ID, ToolName: tc.Name, Input: tc.Input, Output: output, Error: errMsg,
				})
			}

			// Final LLM call to summarize continuation tool results
			if len(contResults) > 0 {
				contMsgs := buildSynthesisMessages(view, assistantText, contResults, false)
				finalReq := &LLMRequest{
					Model:        sc.Model,
					SystemPrompt: systemPrompt,
					Messages:     contMsgs,
				}
				finalChunks, finalErr := e.llm.ChatStream(ctx, finalReq)
				if finalErr == nil {
					var finalText string
					var finalUsage TokenUsage
					var ftSplitter textutil.ThinkTagSplitter
					for chunk := range finalChunks {
						if chunk.Content != "" {
							_, content := ftSplitter.Push(chunk.Content)
							if content != "" {
								finalText += content
								emit(&gateway.EngineEvent{
									Type: "text", Content: content, SessionID: sc.SessionID,
									Metadata: map[string]string{"is_complete": "false"},
								})
							}
						}
						if chunk.Done {
							finalUsage = chunk.Usage
						}
					}
					if _, content := ftSplitter.Flush(); content != "" {
						finalText += content
						emit(&gateway.EngineEvent{
							Type: "text", Content: content, SessionID: sc.SessionID,
							Metadata: map[string]string{"is_complete": "false"},
						})
					}
					finalText = textutil.StripThinkingTags(finalText)
					if strings.TrimSpace(finalText) != "" {
						assistantText = finalText
						res.Usage.PromptTokens += finalUsage.PromptTokens
						res.Usage.CompletionTokens += finalUsage.CompletionTokens
					}
				}
			}
		}
	}

	assistantText = textutil.StripThinkingTags(assistantText)
	if emitComplete {
		duration := time.Since(start).Milliseconds()
		ctxPct := gateway.ComputeCtxPct(res.Usage.PromptTokens, sc.TokenBudget.MaxContextTokens)
		emit(&gateway.EngineEvent{
			Type: "complete", SessionID: sc.SessionID,
			Metadata: map[string]string{
				"usage":      fmt.Sprintf("%d", res.Usage.PromptTokens+res.Usage.CompletionTokens),
				"duration":   fmt.Sprintf("%d", duration),
				"model":      sc.Model,
				"ctx_pct":    fmt.Sprintf("%d", ctxPct),
				"llm_called": "true",
			},
		})
		if queryRunSpan != nil {
			queryRunSpan.SetAttributes(
				tracer.Attribute{Key: "pev.duration_ms", Value: fmt.Sprintf("%d", duration)},
				tracer.Attribute{Key: "pev.total_tokens", Value: fmt.Sprintf("%d", res.Usage.PromptTokens+res.Usage.CompletionTokens)},
				tracer.Attribute{Key: "pev.prompt_tokens", Value: fmt.Sprintf("%d", res.Usage.PromptTokens)},
				tracer.Attribute{Key: "pev.completion_tokens", Value: fmt.Sprintf("%d", res.Usage.CompletionTokens)},
				tracer.Attribute{Key: "pev.ctx_pct", Value: fmt.Sprintf("%d", ctxPct)},
				tracer.Attribute{Key: "pev.max_context_tokens", Value: fmt.Sprintf("%d", sc.TokenBudget.MaxContextTokens)},
				tracer.Attribute{Key: "pev.llm_called", Value: "true"},
			)
		}
		sc.PEVState.Phase = types.PEVPhaseDone
	}

	var msgs []types.Message
	if assistantText != "" {
		msgs = append(msgs, types.Message{Role: types.MessageRoleAssistant, Content: assistantText, SessionID: sc.SessionID})
	}
	return &PEVRunResult{
		Messages:        msgs,
		Usage:           queryUsage(res.Usage),
		ToolCallHistory: res.ToolCallHistory,
	}, nil
}

// InitSessionPermission applies default permission mode and plan file path.
func InitSessionPermission(sc *types.SessionContext, cfg config.ContextPermissionConfig) {
	if sc == nil {
		return
	}
	if sc.PermissionMode == "" {
		sc.PermissionMode = types.PermissionMode(cfg.DefaultMode)
		if sc.PermissionMode == "" {
			sc.PermissionMode = types.PermissionDefault
		}
	}
	if sc.PlanFilePath == "" && cfg.Plan.PlanFileDir != "" {
		sc.PlanFilePath = fmt.Sprintf("%s/%s.md", cfg.Plan.PlanFileDir, sc.SessionID)
	}
}
