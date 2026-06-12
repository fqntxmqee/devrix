package contextengine

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/pev"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/metrics"
	"github.com/devrix/devrix/internal/layers/observability/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/tracer"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/textutil"
	"github.com/devrix/devrix/internal/shared/types"
)

// PEVEngine runs the PEV Execute→Verify loop.
type PEVEngine struct {
	llm        ILLMGateway
	tools      IToolRunner
	toolsReg   IToolRegistry
	permission IPermissionGate
	observer      IObserver
	pevObserver   IPEVObserver
	verifyRunner  IVerifyCommandRunner
	cfg             *config.PEVConfig
	planCfg         config.PlanConfig
	planEngine      *pev.PlanEngine
	milestoneRunner *pev.MilestoneRunner
	obsBridge       *observability.Bridge

	toolLatencyMu sync.Mutex
	toolLatency   map[string]metrics.Histogram

	queryLoop QueryLoopSupport
}

// NewPEVEngine creates a PEV engine.
func NewPEVEngine(
	llm ILLMGateway,
	tools IToolRunner,
	toolsReg IToolRegistry,
	permission IPermissionGate,
	observer IObserver,
	cfg *config.PEVConfig,
	obsBridge *observability.Bridge,
	verifyRunner IVerifyCommandRunner,
	pevObserver IPEVObserver,
	planner contracts.IMilestonePlanner,
	planCfg config.PlanConfig,
) *PEVEngine {
	if cfg == nil {
		cfg = &config.PEVConfig{MaxIterations: 3, VerifyMode: config.VerifyModeBasic}
	}
	if observer == nil {
		observer = NoOpObserver{}
	}
	if pevObserver == nil {
		pevObserver = NoOpPEVObserver{}
	}
	e := &PEVEngine{
		llm:          llm,
		tools:        tools,
		toolsReg:     toolsReg,
		permission:   permission,
		observer:     observer,
		pevObserver:  pevObserver,
		verifyRunner: verifyRunner,
		cfg:          cfg,
		planCfg:      planCfg,
		obsBridge:    obsBridge,
	}
	if planner != nil && llm != nil {
		e.planEngine = pev.NewPlanEngine(newPlanLLMAdapter(llm), planner, planCfg)
		e.milestoneRunner = pev.NewMilestoneRunner(
			planner,
			planCfg,
			e.executeMilestone,
			pevObserver.EmitMilestoneProgress,
		)
	}
	return e
}


// VerifyPEV runs the verify phase for tests and tooling.
func (e *PEVEngine) VerifyPEV(ctx context.Context, sc *types.SessionContext, results []ToolResult) types.VerifyResult {
	return e.verify(ctx, sc, results)
}

// PEVRunResult holds PEV output.
// 方案 2: 增加 ToolCallHistory 字段，用于同步到 sc.Messages
type PEVRunResult struct {
	Messages        []types.Message     // 当前轮次的助手消息（最终回复）
	Usage           TokenUsage
	ToolCallHistory []types.ToolCallRecord // 完整的工具调用历史，用于同步到 sc.Messages
}

// Run executes PEV loop and emits gateway events.
func (e *PEVEngine) Run(
	ctx context.Context,
	sc *types.SessionContext,
	view []types.Message,
	userMessage string,
	emit func(*contracts.EngineEvent),
) (*PEVRunResult, error) {
	if e.planEngine != nil && e.milestoneRunner != nil && pev.ShouldPlan(e.planCfg, sc.PEVState, userMessage) {
		sc.PEVState.Phase = types.PEVPhasePlan
		e.observer.EmitPEVPhase(sc.SessionID, types.PEVPhasePlan, 0)

		planCtx, planSpan := e.startSpan(ctx, telemetry.OpContextPlanGenerate, tracer.SpanKindInternal)
		planResult, err := e.planEngine.Plan(planCtx, userMessage)
		if planSpan != nil {
			if err != nil {
				planSpan.RecordError(err)
			} else if planResult != nil {
				planSpan.SetAttributes(
					tracer.Attribute{Key: "plan.task_id", Value: planResult.TaskID},
					tracer.Attribute{Key: "plan.milestone_count", Value: fmt.Sprintf("%d", len(planResult.Milestones))},
				)
			}
			planSpan.End()
		}
		if err != nil {
			return nil, err
		}
		if planResult != nil && !planResult.Degraded && len(planResult.Milestones) > 0 {
			e.pevObserver.EmitPlanCompleted(sc.SessionID, len(planResult.Milestones))
			if runErr := e.milestoneRunner.Run(ctx, sc, view, planResult.TaskID, emit); runErr != nil {
				return nil, runErr
			}
			// DM-20260611-008：milestone-only 路径不调 LLM（plan 已先调过），
			// usage="0" 是真实情况；打 llm_called="false" 让 D5 观测层能区分。
			emit(&contracts.EngineEvent{
				Type: "complete", SessionID: sc.SessionID,
				Metadata: map[string]string{
					"usage":      "0",
					"duration":   "0",
					"model":      sc.Model,
					"ctx_pct":    "0",
					"llm_called": "false",
				},
			})
			sc.PEVState.Phase = types.PEVPhaseDone
			return &PEVRunResult{Messages: sc.Messages}, nil
		}
	}

	return e.runExecuteVerifyLoop(ctx, sc, view, sc.SystemPrompt, emit, true)
}

func (e *PEVEngine) executeMilestone(
	ctx context.Context,
	sc *types.SessionContext,
	view []types.Message,
	m *types.Milestone,
	emit func(*contracts.EngineEvent),
) (bool, error) {
	msCtx, msSpan := e.startSpan(ctx, telemetry.OpContextMilestoneRun, tracer.SpanKindInternal,
		tracer.Attribute{Key: "milestone.id", Value: m.ID},
		tracer.Attribute{Key: "plan.task_id", Value: sc.PEVState.ActiveTaskID},
	)
	if msSpan != nil {
		defer msSpan.End()
	}
	prompt := sc.SystemPrompt + milestonePromptSuffix(m)
	_, err := e.runExecuteVerifyLoop(msCtx, sc, view, prompt, emit, false)
	if err != nil {
		return false, err
	}
	return sc.PEVState.VerifyResult.Passed, nil
}

func milestonePromptSuffix(m *types.Milestone) string {
	if m == nil {
		return ""
	}
	return fmt.Sprintf("\n\n## Current Milestone: %s (%s)\n%s", m.Name, m.ID, m.Description)
}

func (e *PEVEngine) runExecuteVerifyLoop(
	ctx context.Context,
	sc *types.SessionContext,
	view []types.Message,
	systemPrompt string,
	emit func(*contracts.EngineEvent),
	emitComplete bool,
) (*PEVRunResult, error) {
	if e.queryLoopEnabled() {
		fmt.Fprintf(os.Stderr, "[DEBUG-PEV] queryLoop path enabled=%v verifyMode=%q\n", e.queryLoopEnabled(), e.cfg.VerifyMode)
		return e.runViaQueryLoop(ctx, sc, view, systemPrompt, emit, emitComplete)
	}
	fmt.Fprintf(os.Stderr, "[DEBUG-PEV] legacy path enabled=%v verifyMode=%q\n", e.queryLoopEnabled(), e.cfg.VerifyMode)
	start := time.Now()
	maxIter := e.cfg.MaxIterations
	if maxIter <= 0 {
		maxIter = 3
	}
	if isSingleRoundVerifyMode(e.cfg.VerifyMode) {
		maxIter = 1
	}

	toolSchemas := resolveVisibleTools(ctx, e.toolsReg, sc)
	toolNames := make([]string, 0, len(toolSchemas))
	for _, t := range toolSchemas {
		toolNames = append(toolNames, t.Name)
	}

	ctx, runSpan := e.startSpan(ctx, telemetry.OpContextPEVRun, tracer.SpanKindInternal,
		tracer.Attribute{Key: "pev.max_iterations", Value: fmt.Sprintf("%d", maxIter)},
		tracer.Attribute{Key: "pev.tools_count", Value: fmt.Sprintf("%d", len(toolSchemas))},
		tracer.Attribute{Key: "pev.tools_names", Value: strings.Join(toolNames, ",")},
	)
	if runSpan != nil {
		defer runSpan.End()
	}

	req := &LLMRequest{
		Model:        sc.Model,
		SystemPrompt: systemPrompt,
		Messages:     view,
		Tools:        toolSchemas,
	}

	var assistantText string
	var toolResults []ToolResult
	var usage TokenUsage
	var allToolCallRecords []types.ToolCallRecord // 方案 2: 收集所有工具调用记录

	for iter := 0; iter < maxIter; iter++ {
		iterCtx, iterSpan := e.startSpan(ctx, telemetry.OpContextPEVIteration, tracer.SpanKindInternal,
			tracer.Attribute{Key: "pev.iteration", Value: fmt.Sprintf("%d", iter)},
			tracer.Attribute{Key: "pev.messages_before", Value: fmt.Sprintf("%d", len(req.Messages))},
			tracer.Attribute{Key: "pev.tools_count", Value: fmt.Sprintf("%d", len(req.Tools))},
		)
		endIterSpan := func() {
			if iterSpan != nil {
				iterSpan.End()
			}
		}

		sc.PEVState.Phase = types.PEVPhaseExecute
		sc.PEVState.Iteration = iter
		e.observer.EmitPEVIteration(sc.SessionID, iter, types.PEVPhaseExecute)

		// LLM call span + metrics.
		llmStart := time.Now()
		llmCtx, llmSpan := e.startSpan(iterCtx, telemetry.OpContextPEVLLMCall, tracer.SpanKindClient,
			tracer.Attribute{Key: "pev.iteration", Value: fmt.Sprintf("%d", iter)},
			tracer.Attribute{Key: "llm.model", Value: sc.Model},
			tracer.Attribute{Key: "llm.tools_names", Value: strings.Join(toolNames, ",")},
			tracer.Attribute{Key: "llm.system_prompt_len", Value: fmt.Sprintf("%d", len(systemPrompt))},
		)
		AddLLMRequestEvent(llmSpan, sc.SessionID, iter, sc.Model, req)
		chunks, err := e.llm.ChatStream(llmCtx, req)
		if err != nil {
			errDetail := errors.FormatLLMError(err)
			if llmSpan != nil {
				AddLLMResponseEvent(llmSpan, sc.SessionID, iter, "", TokenUsage{}, nil, nil)
				llmSpan.RecordError(err)
				llmSpan.SetStatus(tracer.StatusCodeError, errDetail)
				llmSpan.SetAttributes(
					tracer.Attribute{Key: "llm.status", Value: "error"},
					tracer.Attribute{Key: "llm.error.detail", Value: truncateSpanAttr(errDetail, 500)},
					tracer.Attribute{Key: "error.code", Value: errors.ErrorCode(err)},
				)
				llmSpan.End()
			}
			slog.WarnContext(iterCtx, "pev: llm call failed",
				"sessionID", sc.SessionID,
				"iteration", iter,
				"cause", errDetail,
			)
			// After tools ran, never fail the whole user message on a follow-up LLM error:
			// synthesis / tool_fallback can still produce a useful reply.
			if len(toolResults) > 0 || (iter > 0 && hasSuccessfulToolOutput(toolResults)) {
				slog.WarnContext(iterCtx, "pev: llm call failed after tools, degrading to synthesis",
					"sessionID", sc.SessionID,
					"iteration", iter,
					"cause", errDetail,
				)
				endIterSpan()
				break
			}
			endIterSpan()
			return nil, errors.NewLLMUnavailableError(err)
		}

		var pendingTools []ToolCall
		var tagSplitter textutil.ThinkTagSplitter
		for chunk := range chunks {
			select {
			case <-ctx.Done():
				if llmSpan != nil {
					llmSpan.End()
				}
				endIterSpan()
				return nil, ctx.Err()
			default:
			}
			if chunk.Thinking != "" {
				emit(&contracts.EngineEvent{Type: "thinking", Content: chunk.Thinking, SessionID: sc.SessionID})
			}
			if chunk.Content != "" {
				thinking, content := tagSplitter.Push(chunk.Content)
				if thinking != "" {
					emit(&contracts.EngineEvent{Type: "thinking", Content: thinking, SessionID: sc.SessionID})
				}
				if content != "" {
					assistantText += content
					emit(&contracts.EngineEvent{
						Type: "text", Content: content, SessionID: sc.SessionID,
						Metadata: map[string]string{"is_complete": "false"},
					})
				}
			}
			if len(chunk.ToolCalls) > 0 {
				// Stream accumulator emits the full merged set on every delta.
				pendingTools = chunk.ToolCalls
			}
			if chunk.Done {
				usage = chunk.Usage
			}
		}
		if thinking, content := tagSplitter.Flush(); thinking != "" || content != "" {
			if thinking != "" {
				emit(&contracts.EngineEvent{Type: "thinking", Content: thinking, SessionID: sc.SessionID})
			}
			if content != "" {
				assistantText += content
				emit(&contracts.EngineEvent{
					Type: "text", Content: content, SessionID: sc.SessionID,
					Metadata: map[string]string{"is_complete": "false"},
				})
			}
		}
		if llmSpan != nil {
			AddLLMResponseEvent(llmSpan, sc.SessionID, iter, assistantText, usage, pendingTools, nil)
			finishReason := "stop"
			if len(pendingTools) > 0 {
				finishReason = "tool_calls"
			}
			llmSpan.SetAttributes(telemetry.LLMUsageAttrs(
				usage.PromptTokens,
				usage.CompletionTokens,
				time.Since(llmStart).Milliseconds(),
			)...)
			llmSpan.SetAttributes(telemetry.GenAIUsageAttrs(
				sc.Model, sc.SessionID, usage.PromptTokens, usage.CompletionTokens,
				usage.CacheReadTokens, usage.ReasoningTokens, finishReason,
			)...)
			llmSpan.End()
		}

		if len(pendingTools) == 0 {
			endIterSpan()
			break
		}

		pendingTools = dedupeToolCalls(pendingTools)
		for i := range pendingTools {
			pendingTools[i].ID = ensureToolCallID(pendingTools[i], i)
		}
		assistantMsg := buildAssistantToolCallsMessage(sc.SessionID, pendingTools)
		if assistantText != "" {
			assistantMsg.Content = assistantText
		}
		req.Messages = append(req.Messages, assistantMsg)

		for _, tc := range pendingTools {
			risk := tc.RiskLevel
			if risk == "" {
				risk = e.toolsReg.RiskLevel(tc.Name)
			}
			emit(&contracts.EngineEvent{
				Type: "tool_call", ToolName: tc.Name, ToolInput: tc.Input, ToolRisk: risk,
				SessionID: sc.SessionID,
				Metadata: map[string]string{"tool_name": tc.Name, "input": tc.Input, "risk_level": string(risk)},
			})

			// Tool execution span - permission_check is a child.
			toolCtx, toolSpan := e.startSpan(iterCtx, telemetry.OpContextPEVToolExecute, tracer.SpanKindInternal,
				tracer.Attribute{Key: "tool.name", Value: tc.Name},
				tracer.Attribute{Key: "tool.risk_level", Value: string(risk)},
				tracer.Attribute{Key: "tool.input_preview", Value: truncateSpanAttr(tc.Input, 500)},
			)

			// Permission check span.
			_, permSpan := e.startSpan(toolCtx, telemetry.OpContextPEVPermissionCheck, tracer.SpanKindInternal,
				tracer.Attribute{Key: "tool.name", Value: tc.Name},
			)
			if e.permission != nil && !e.permission.Request(ctx, sc.SessionID, tc.Name, tc.Input, risk) {
				if permSpan != nil {
					permSpan.SetAttributes(tracer.Attribute{Key: "permission.result", Value: "denied"})
					permSpan.End()
				}
				if toolSpan != nil {
					toolSpan.RecordError(fmt.Errorf("permission denied"))
					toolSpan.End()
				}
				emit(pevErrorEvent(sc.SessionID, errors.NewContextPermissionDeniedError(tc.Name), false))
				endIterSpan()
				return &PEVRunResult{Usage: usage}, errors.NewContextPermissionDeniedError(tc.Name)
			}
			if permSpan != nil {
				permSpan.SetAttributes(tracer.Attribute{Key: "permission.result", Value: "approved"})
				permSpan.End()
			}

			toolStart := time.Now()
			toolExecCtx := withToolStreamEmitter(
				ToolContextWithGate(ctx, sc, e.permission),
				emit, sc.SessionID, tc.Name,
			)
			result, err := e.tools.Execute(toolExecCtx, tc)
			toolDuration := time.Since(toolStart)
			toolStatus := "ok"
			if err != nil {
				result = &ToolResult{Error: err.Error()}
				toolStatus = "error"
				if toolSpan != nil {
					telemetry.RecordSpanError(toolSpan, err)
				}
			} else {
			}
			e.recordToolLatency(tc.Name, risk, toolStatus, toolDuration)
			if toolSpan != nil {
				toolSpan.SetAttributes(
					tracer.Attribute{Key: "tool.duration_ms", Value: fmt.Sprintf("%d", toolDuration.Milliseconds())},
					tracer.Attribute{Key: "tool.output_preview", Value: truncateSpanAttr(result.Output, 500)},
				)
				toolSpan.End()
			}
			toolResults = append(toolResults, *result)

			content := result.Output
			if result.Error != "" {
				content = result.Error
			}
			emit(&contracts.EngineEvent{
				Type: "tool_result", Content: content, ToolName: tc.Name, SessionID: sc.SessionID,
				Metadata: map[string]string{"tool_name": tc.Name, "error": result.Error},
			})

			callRecord := types.ToolCallRecord{
				CallID:    tc.ID,
				ToolName:  tc.Name,
				Input:     tc.Input,
				Output:    result.Output,
				RiskLevel: risk,
				Error:     result.Error,
			}
			sc.PEVState.LastToolCalls = append(sc.PEVState.LastToolCalls, callRecord)

			// 方案 2: 收集工具调用记录
			allToolCallRecords = append(allToolCallRecords, callRecord)

			req.Messages = append(req.Messages, buildToolResultMessage(sc.SessionID, tc.ID, content))
		}

		sc.PEVState.Phase = types.PEVPhaseVerify
		e.observer.EmitPEVPhase(sc.SessionID, types.PEVPhaseVerify, iter)
		_, verifySpan := e.startSpan(iterCtx, telemetry.OpContextPEVVerify, tracer.SpanKindInternal,
			tracer.Attribute{Key: "verify.mode", Value: e.cfg.VerifyMode},
		)
		vr := e.verify(iterCtx, sc, toolResults)
		sc.PEVState.VerifyResult = vr
		if verifySpan != nil {
			verifySpan.SetAttributes(
				tracer.Attribute{Key: "verify.passed", Value: fmt.Sprintf("%t", vr.Passed)},
				tracer.Attribute{Key: "verify.command_count", Value: fmt.Sprintf("%d", len(vr.Commands))},
			)
			if !vr.Passed {
				verifySpan.SetAttributes(tracer.Attribute{Key: "verify.failure_reason", Value: verifyFailureReason(vr)})
			}
			verifySpan.End()
		}
		// basic/none: one tool round per user message; synthesis handles the final reply.
		if isSingleRoundVerifyMode(e.cfg.VerifyMode) {
			endIterSpan()
			break
		}
		if vr.Passed {
			endIterSpan()
			break
		}
		// Follow-up PEV iterations: avoid re-sending raw tool_calls/tool messages to
		// providers (MiniMax returns 400 on mismatched tool_call_id).
		if len(toolResults) > 0 {
			req.Messages = buildSynthesisMessages(view, assistantText, toolResults, e.isYOLO())
		}
		if iter == maxIter-1 {
			emit(pevErrorEvent(sc.SessionID, errors.NewPEVMaxIterationsError(), true))
			endIterSpan()
			return &PEVRunResult{Usage: usage}, errors.NewPEVMaxIterationsError()
		}
		endIterSpan()
	}

	if len(toolResults) > 0 {
		synthText, _, synthUsage := e.runToolSynthesis(ctx, sc, view, systemPrompt, assistantText, toolResults, emit)
		usage.PromptTokens += synthUsage.PromptTokens
		usage.CompletionTokens += synthUsage.CompletionTokens
		assistantText = synthText
	}

	if len(toolResults) == 0 && !sc.PEVState.VerifyResult.Passed {
		sc.PEVState.VerifyResult = types.VerifyResult{Passed: true}
	}

	assistantText = textutil.StripThinkingTags(assistantText)

	duration := time.Since(start).Milliseconds()
	ctxPct := contracts.ComputeCtxPct(usage.PromptTokens, sc.TokenBudget.MaxContextTokens)
	if emitComplete {
		emit(&contracts.EngineEvent{
			Type: "complete", SessionID: sc.SessionID,
			Metadata: map[string]string{
				"usage":      fmt.Sprintf("%d", usage.PromptTokens+usage.CompletionTokens),
				"duration":   fmt.Sprintf("%d", duration),
				"model":      sc.Model,
				"ctx_pct":    fmt.Sprintf("%d", ctxPct),
				"llm_called": "true",
			},
		})
	}

	if runSpan != nil {
		runSpan.SetAttributes(
			tracer.Attribute{Key: "pev.duration_ms", Value: fmt.Sprintf("%d", duration)},
			tracer.Attribute{Key: "pev.total_tokens", Value: fmt.Sprintf("%d", usage.PromptTokens+usage.CompletionTokens)},
			tracer.Attribute{Key: "pev.prompt_tokens", Value: fmt.Sprintf("%d", usage.PromptTokens)},
			tracer.Attribute{Key: "pev.completion_tokens", Value: fmt.Sprintf("%d", usage.CompletionTokens)},
			tracer.Attribute{Key: "pev.ctx_pct", Value: fmt.Sprintf("%d", ctxPct)},
			tracer.Attribute{Key: "pev.max_context_tokens", Value: fmt.Sprintf("%d", sc.TokenBudget.MaxContextTokens)},
			tracer.Attribute{Key: "pev.llm_called", Value: "true"},
		)
	}

	if emitComplete {
		sc.PEVState.Phase = types.PEVPhaseDone
	}
	var msgs []types.Message
	if assistantText != "" {
		msgs = append(msgs, types.Message{Role: types.MessageRoleAssistant, Content: assistantText})
	}
	// 方案 2: 返回完整的工具调用历史
	return &PEVRunResult{Usage: usage, Messages: msgs, ToolCallHistory: allToolCallRecords}, nil
}

// runToolSynthesis calls the LLM to summarize successful tool output.
// On LLM failure it emits tool_fallback text from raw tool output.
// In YOLO mode tools are kept in the request so the LLM may continue calling them.
// When toolCalls are returned the caller should execute them in YOLO mode.
func (e *PEVEngine) runToolSynthesis(
	ctx context.Context,
	sc *types.SessionContext,
	view []types.Message,
	systemPrompt string,
	preamble string,
	toolResults []ToolResult,
	emit func(*contracts.EngineEvent),
) (string, []ToolCall, TokenUsage) {
	yolo := e.isYOLO()
	var tools []ToolSchema
	if yolo {
		tools = resolveVisibleTools(ctx, e.toolsReg, sc)
		tools = FilterToolsByPermissionMode(sc.PermissionMode, tools, sc.PlanFilePath)
		tools = FilterToolsForAgentRole(sc, tools)
	}
	synthReq := &LLMRequest{
		Model:        sc.Model,
		SystemPrompt: systemPrompt,
		Messages:     buildSynthesisMessages(view, preamble, toolResults, yolo),
		Tools:        tools, // kept in YOLO mode so LLM may continue tool calls
	}

	llmStart := time.Now()
	llmCtx, llmSpan := e.startSpan(ctx, telemetry.OpContextPEVSynthesis, tracer.SpanKindClient,
		tracer.Attribute{Key: "pev.synthesis", Value: "true"},
		tracer.Attribute{Key: "llm.model", Value: sc.Model},
	)
	AddLLMRequestEvent(llmSpan, sc.SessionID, -1, sc.Model, synthReq)
	chunks, err := e.llm.ChatStream(llmCtx, synthReq)
	if err != nil {
		errDetail := errors.FormatLLMError(err)
		if llmSpan != nil {
			llmSpan.RecordError(err)
			llmSpan.SetStatus(tracer.StatusCodeError, errDetail)
			llmSpan.SetAttributes(
				tracer.Attribute{Key: "llm.status", Value: "error"},
				tracer.Attribute{Key: "llm.error.detail", Value: truncateSpanAttr(errDetail, 500)},
				tracer.Attribute{Key: "pev.synthesis_source", Value: "tool_fallback"},
				tracer.Attribute{Key: "error.code", Value: errors.ErrorCode(err)},
			)
			llmSpan.End()
		}
		slog.Warn("pev: synthesis llm failed", "sessionID", sc.SessionID, "cause", errDetail)
		fallback := summarizeSuccessfulToolResults(toolResults)
		emit(&contracts.EngineEvent{
			Type: "text", Content: fallback, SessionID: sc.SessionID,
			Metadata: map[string]string{"source": "tool_fallback", "is_complete": "false"},
		})
					return fallback, nil, TokenUsage{}
	}

	var synthText string
	var toolCalls []ToolCall
	var usage TokenUsage
	var tagSplitter textutil.ThinkTagSplitter
	for chunk := range chunks {
		select {
		case <-ctx.Done():
			if llmSpan != nil {
				llmSpan.End()
			}
			fallback := summarizeSuccessfulToolResults(toolResults)
			emit(&contracts.EngineEvent{
				Type: "text", Content: fallback, SessionID: sc.SessionID,
				Metadata: map[string]string{"source": "tool_fallback", "is_complete": "false"},
			})
			return fallback, nil, usage
		default:
		}
		if chunk.Content != "" {
			_, content := tagSplitter.Push(chunk.Content)
			if content != "" {
				synthText += content
				emit(&contracts.EngineEvent{
					Type: "text", Content: content, SessionID: sc.SessionID,
					Metadata: map[string]string{"is_complete": "false"},
				})
			}
		}
		if len(chunk.ToolCalls) > 0 {
			toolCalls = chunk.ToolCalls
		}
		if chunk.Done {
			usage = chunk.Usage
		}
	}
	if _, content := tagSplitter.Flush(); content != "" {
		synthText += content
		emit(&contracts.EngineEvent{
			Type: "text", Content: content, SessionID: sc.SessionID,
			Metadata: map[string]string{"is_complete": "false"},
		})
	}
	if llmSpan != nil {
		AddLLMResponseEvent(llmSpan, sc.SessionID, -1, synthText, usage, nil, toolResults)
		llmSpan.SetAttributes(telemetry.LLMUsageAttrs(
			usage.PromptTokens,
			usage.CompletionTokens,
			time.Since(llmStart).Milliseconds(),
		)...)
		llmSpan.SetAttributes(telemetry.GenAIUsageAttrs(
			sc.Model, sc.SessionID, usage.PromptTokens, usage.CompletionTokens,
			usage.CacheReadTokens, usage.ReasoningTokens, "stop",
		)...)
		llmSpan.SetAttributes(tracer.Attribute{Key: "pev.synthesis_source", Value: "llm"})
		llmSpan.End()
	}

	synthText = textutil.StripThinkingTags(synthText)
	if strings.TrimSpace(synthText) == "" {
		fallback := summarizeSuccessfulToolResults(toolResults)
		emit(&contracts.EngineEvent{
			Type: "text", Content: fallback, SessionID: sc.SessionID,
			Metadata: map[string]string{"source": "tool_fallback", "is_complete": "false"},
		})
		return fallback, nil, usage
	}
	return synthText, toolCalls, usage
}

func (e *PEVEngine) startSpan(ctx context.Context, operation string, kind tracer.SpanKind, attrs ...tracer.Attribute) (context.Context, tracer.Span) {
	if e.obsBridge == nil || e.obsBridge.Tracer() == nil {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(kind),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(operation, attrs...)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return e.obsBridge.Tracer().Start(ctx, operation, opts...)
}

func (e *PEVEngine) recordToolLatency(toolName string, risk types.RiskLevel, status string, duration time.Duration) {
	if h := e.toolLatencyHistogram(toolName, risk, status); h != nil {
		h.Observe(duration.Seconds())
	}
}

func (e *PEVEngine) toolLatencyHistogram(toolName string, risk types.RiskLevel, status string) metrics.Histogram {
	if e.obsBridge == nil || e.obsBridge.Meter() == nil {
		return nil
	}
	key := toolName + "|" + string(risk) + "|" + status
	e.toolLatencyMu.Lock()
	defer e.toolLatencyMu.Unlock()
	if e.toolLatency == nil {
		e.toolLatency = make(map[string]metrics.Histogram)
	}
	if h, ok := e.toolLatency[key]; ok {
		return h
	}
	tb := observability.NewToolBridgeFromBridge(e.obsBridge)
	lm, err := tb.InitLatencyMetrics(toolName, string(risk), status)
	if err != nil || lm == nil || lm.Latency == nil {
		return nil
	}
	e.toolLatency[key] = lm.Latency
	return lm.Latency
}

// yoloChecker is an optional interface that IPermissionGate can implement
// to report whether YOLO auto-approval mode is active.
type yoloChecker interface {
	IsYOLOMode() bool
}

// isYOLO reports whether the permission gate has YOLO mode enabled.
func (e *PEVEngine) isYOLO() bool {
	yc, ok := e.permission.(yoloChecker)
	return ok && yc.IsYOLOMode()
}

func withToolStreamEmitter(
	ctx context.Context,
	emit func(*contracts.EngineEvent),
	sessionID, toolName string,
) context.Context {
	if emit == nil {
		return ctx
	}
	return WithToolStreamEmitter(ctx, func(ev ToolStreamEvent) {
		switch ev.Type {
		case "thinking":
			emit(&contracts.EngineEvent{
				Type: "thinking", Content: ev.Content, SessionID: sessionID,
				Metadata: map[string]string{"source": "agent_tool", "tool_name": toolName, "agent": ev.ToolName},
			})
		case "text":
			emit(&contracts.EngineEvent{
				Type: "text", Content: ev.Content, SessionID: sessionID,
				Metadata: map[string]string{"is_complete": "false", "source": "agent_tool", "tool_name": toolName, "agent": ev.ToolName},
			})
		case "tool_use":
			emit(&contracts.EngineEvent{
				Type: "info", Content: ev.Content, SessionID: sessionID,
				Metadata: map[string]string{"source": "agent_tool", "tool_name": toolName, "agent": ev.ToolName},
			})
		}
	})
}

func pevErrorEvent(sessionID string, err *errors.SentinelError, recoverable bool) *contracts.EngineEvent {
	rec := "false"
	if recoverable {
		rec = "true"
	}
	return &contracts.EngineEvent{
		Type: "error", Content: err.Error(), SessionID: sessionID,
		Metadata: map[string]string{"code": err.Code, "recoverable": rec},
	}
}

func resolveVisibleTools(ctx context.Context, reg IToolRegistry, sc *types.SessionContext) []ToolSchema {
	if sc != nil && sc.Harness != nil && len(sc.Harness.Report.VisibleToolList) > 0 {
		return visibleToolsToSchemas(sc.Harness)
	}
	workDir := ""
	if sc != nil {
		workDir = sc.WorkDir
	}
	tools, _ := reg.ListTools(ctx, workDir)
	return tools
}

func verifyFailureReason(vr types.VerifyResult) string {
	if vr.Passed {
		return ""
	}
	if vr.Deviation > 0 {
		return fmt.Sprintf("verify_failed:deviation=%.2f", vr.Deviation)
	}
	return "verify_failed:no_successful_tool_output"
}

