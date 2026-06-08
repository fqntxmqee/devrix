package contextengine

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/gateway"
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

	metricsOnce sync.Once
	llmTokens   metrics.Counter
	llmLatency  metrics.Histogram
	llmErrors   metrics.Counter
	toolCalls   metrics.Counter
	toolErrors  metrics.Counter
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

func (e *PEVEngine) initMetrics() {
	e.metricsOnce.Do(func() {
		if e.obsBridge == nil || e.obsBridge.Meter() == nil {
			return
		}
		m := e.obsBridge.Meter()
		e.llmTokens, _ = m.Int64Counter("engine_llm_tokens", metrics.WithLabels(metrics.LabelMap{
			"provider": "mock", "model": "mock",
		}))
		e.llmLatency, _ = m.Float64Histogram("engine_llm_latency",
			metrics.WithHistogramLabels(metrics.LabelMap{"provider": "mock", "model": "mock"}),
			metrics.WithBounds(metrics.LLMHistogramBounds()),
		)
		e.llmErrors, _ = m.Int64Counter("engine_llm_errors", metrics.WithLabels(metrics.LabelMap{
			"provider": "mock", "model": "mock",
		}))
		e.toolCalls, _ = m.Int64Counter("engine_tool_calls", metrics.WithLabels(metrics.LabelMap{
			"tool": "all", "risk_level": "all",
		}))
		e.toolErrors, _ = m.Int64Counter("engine_tool_errors", metrics.WithLabels(metrics.LabelMap{
			"tool": "all", "risk_level": "all",
		}))
	})
}

// VerifyPEV runs the verify phase for tests and tooling.
func (e *PEVEngine) VerifyPEV(ctx context.Context, sc *types.SessionContext, results []ToolResult) types.VerifyResult {
	return e.verify(ctx, sc, results)
}

// PEVRunResult holds PEV output.
type PEVRunResult struct {
	Messages []types.Message
	Usage    TokenUsage
}

// Run executes PEV loop and emits gateway events.
func (e *PEVEngine) Run(
	ctx context.Context,
	sc *types.SessionContext,
	view []types.Message,
	userMessage string,
	emit func(*gateway.EngineEvent),
) (*PEVRunResult, error) {
	e.initMetrics()

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
			emit(&gateway.EngineEvent{
				Type: "complete", SessionID: sc.SessionID,
				Metadata: map[string]string{"usage": "0", "duration": "0"},
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
	emit func(*gateway.EngineEvent),
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
	emit func(*gateway.EngineEvent),
	emitComplete bool,
) (*PEVRunResult, error) {
	start := time.Now()
	maxIter := e.cfg.MaxIterations
	if maxIter <= 0 {
		maxIter = 3
	}

	ctx, runSpan := e.startSpan(ctx, telemetry.OpContextPEVRun, tracer.SpanKindInternal,
		tracer.Attribute{Key: "pev.max_iterations", Value: fmt.Sprintf("%d", maxIter)},
	)
	if runSpan != nil {
		defer runSpan.End()
	}

	toolSchemas, _ := e.toolsReg.ListTools(ctx, sc.WorkDir)
	req := &LLMRequest{
		Model:        sc.Model,
		SystemPrompt: systemPrompt,
		Messages:     view,
		Tools:        toolSchemas,
	}

	var assistantText string
	var toolResults []ToolResult
	var usage TokenUsage
	var synthesisLLMDone bool
	var forcedSynthesis bool

	for iter := 0; iter < maxIter; iter++ {
		sc.PEVState.Phase = types.PEVPhaseExecute
		sc.PEVState.Iteration = iter
		e.observer.EmitPEVIteration(sc.SessionID, iter, types.PEVPhaseExecute)

		// LLM call span + metrics.
		llmStart := time.Now()
		llmAttrs := []tracer.Attribute{
			tracer.Attribute{Key: "pev.iteration", Value: fmt.Sprintf("%d", iter)},
			tracer.Attribute{Key: "llm.model", Value: sc.Model},
		}
		isSynthesisCall := forcedSynthesis
		if forcedSynthesis {
			llmAttrs = append(llmAttrs, tracer.Attribute{Key: "pev.synthesis_llm", Value: "true"})
			req.Tools = nil
			forcedSynthesis = false
		}
		_, llmSpan := e.startSpan(ctx, telemetry.OpContextPEVLLMCall, tracer.SpanKindClient, llmAttrs...)
		chunks, err := e.llm.ChatStream(ctx, req)
		if err != nil {
			errDetail := errors.FormatLLMError(err)
			if llmSpan != nil {
				llmSpan.RecordError(err)
				llmSpan.SetAttributes(
					tracer.Attribute{Key: "llm.status", Value: "error"},
					tracer.Attribute{Key: "llm.error.detail", Value: truncateSpanAttr(errDetail, 500)},
				)
				llmSpan.End()
			}
			e.recordLLMError()
			slog.Warn("pev: llm call failed",
				"sessionID", sc.SessionID,
				"iteration", iter,
				"synthesis", isSynthesisCall,
				"error", err,
				"cause", errDetail,
			)
			if summary := summarizeSuccessfulToolResults(toolResults); summary != "" && (isSynthesisCall || iter > 0) {
				if isSynthesisCall && runSpan != nil {
					runSpan.SetAttributes(
						tracer.Attribute{Key: "pev.degraded", Value: "true"},
						tracer.Attribute{Key: "pev.fallback", Value: "tool_results"},
					)
				}
				emit(&gateway.EngineEvent{
					Type: "text", Content: summary, SessionID: sc.SessionID,
					Metadata: map[string]string{"is_complete": "false", "source": "tool_fallback"},
				})
				assistantText += summary
				break
			}
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
				return nil, ctx.Err()
			default:
			}
			if chunk.Thinking != "" {
				emit(&gateway.EngineEvent{Type: "thinking", Content: chunk.Thinking, SessionID: sc.SessionID})
			}
			if chunk.Content != "" {
				thinking, content := tagSplitter.Push(chunk.Content)
				if thinking != "" {
					emit(&gateway.EngineEvent{Type: "thinking", Content: thinking, SessionID: sc.SessionID})
				}
				if content != "" {
					assistantText += content
					emit(&gateway.EngineEvent{
						Type: "text", Content: content, SessionID: sc.SessionID,
						Metadata: map[string]string{"is_complete": "false"},
					})
				}
			}
			if len(chunk.ToolCalls) > 0 {
				pendingTools = append(pendingTools, chunk.ToolCalls...)
			}
			if chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0 {
				usage = chunk.Usage
			}
		}
		if thinking, content := tagSplitter.Flush(); thinking != "" || content != "" {
			if thinking != "" {
				emit(&gateway.EngineEvent{Type: "thinking", Content: thinking, SessionID: sc.SessionID})
			}
			if content != "" {
				assistantText += content
				emit(&gateway.EngineEvent{
					Type: "text", Content: content, SessionID: sc.SessionID,
					Metadata: map[string]string{"is_complete": "false"},
				})
			}
		}
		if llmSpan != nil {
			llmSpan.SetAttributes(telemetry.LLMUsageAttrs(
				usage.PromptTokens,
				usage.CompletionTokens,
				time.Since(llmStart).Milliseconds(),
			)...)
			llmSpan.End()
		}
		e.recordLLMCall(usage, time.Since(llmStart))

		if len(pendingTools) == 0 {
			break
		}

		req.Messages = append(req.Messages, buildAssistantToolCallsMessage(sc.SessionID, pendingTools))
		callIDs := toolCallIDs(pendingTools)

		for i, tc := range pendingTools {
			risk := tc.RiskLevel
			if risk == "" {
				risk = e.toolsReg.RiskLevel(tc.Name)
			}
			emit(&gateway.EngineEvent{
				Type: "tool_call", ToolName: tc.Name, ToolInput: tc.Input, SessionID: sc.SessionID,
				Metadata: map[string]string{"tool_name": tc.Name, "input": tc.Input, "risk_level": string(risk)},
			})

			toolStart := time.Now()
			// Tool execution span.
			_, toolSpan := e.startSpan(ctx, telemetry.OpContextPEVToolExecute, tracer.SpanKindInternal,
				tracer.Attribute{Key: "tool.name", Value: tc.Name},
				tracer.Attribute{Key: "tool.risk_level", Value: string(risk)},
				tracer.Attribute{Key: "tool.input", Value: truncateSpanAttr(tc.Input, 500)},
			)

			// Permission check span.
			_, permSpan := e.startSpan(ctx, telemetry.OpContextPEVPermissionCheck, tracer.SpanKindInternal,
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
				e.recordToolError()
				emit(pevErrorEvent(sc.SessionID, errors.NewContextPermissionDeniedError(tc.Name), false))
				return &PEVRunResult{Usage: usage}, errors.NewContextPermissionDeniedError(tc.Name)
			}
			if permSpan != nil {
				permSpan.SetAttributes(tracer.Attribute{Key: "permission.result", Value: "approved"})
				permSpan.End()
			}

			toolCtx := WithToolWorkDir(ctx, sc.WorkDir)
			result, err := e.tools.Execute(toolCtx, tc)
			if err != nil {
				result = &ToolResult{Error: err.Error()}
				if toolSpan != nil {
					toolSpan.RecordError(err)
				}
				e.recordToolError()
			} else {
				e.recordToolCall()
			}
			if toolSpan != nil {
				out := ""
				if result != nil {
					out = result.Output
					if result.Error != "" {
						out = result.Error
					}
				}
				toolSpan.SetAttributes(
					tracer.Attribute{Key: "tool.output", Value: truncateSpanAttr(out, 500)},
					tracer.Attribute{Key: "tool.duration_ms", Value: fmt.Sprintf("%d", time.Since(toolStart).Milliseconds())},
				)
				toolSpan.End()
			}
			toolResults = append(toolResults, *result)

			content := result.Output
			if result.Error != "" {
				content = result.Error
			}
			emit(&gateway.EngineEvent{
				Type: "tool_result", Content: content, ToolName: tc.Name, SessionID: sc.SessionID,
				Metadata: map[string]string{"tool_name": tc.Name, "error": result.Error},
			})

			sc.PEVState.LastToolCalls = append(sc.PEVState.LastToolCalls, types.ToolCallRecord{
				ToolName: tc.Name, Input: tc.Input, Output: result.Output, RiskLevel: risk, Error: result.Error,
			})

			toolMsg := buildToolResultMessage(sc.SessionID, callIDs[i], content)
			req.Messages = append(req.Messages, toolMsg)
		}

		sc.PEVState.Phase = types.PEVPhaseVerify
		e.observer.EmitPEVPhase(sc.SessionID, types.PEVPhaseVerify, iter)
		_, verifySpan := e.startSpan(ctx, telemetry.OpContextPEVVerify, tracer.SpanKindInternal,
			tracer.Attribute{Key: "verify.mode", Value: e.cfg.VerifyMode},
		)
		vr := e.verify(ctx, sc, toolResults)
		sc.PEVState.VerifyResult = vr
		if verifySpan != nil {
			verifySpan.SetAttributes(
				tracer.Attribute{Key: "verify.passed", Value: fmt.Sprintf("%t", vr.Passed)},
				tracer.Attribute{Key: "verify.deviation", Value: fmt.Sprintf("%.2f", vr.Deviation)},
			)
			verifySpan.End()
		}
		if vr.Passed {
			if strings.TrimSpace(assistantText) == "" && hasSuccessfulToolOutput(toolResults) && !synthesisLLMDone {
				synthesisLLMDone = true
				forcedSynthesis = true
				continue
			}
			break
		}
		if iter == maxIter-1 {
			emit(pevErrorEvent(sc.SessionID, errors.NewPEVMaxIterationsError(), true))
			return &PEVRunResult{Usage: usage}, errors.NewPEVMaxIterationsError()
		}
	}

	if len(toolResults) == 0 && !sc.PEVState.VerifyResult.Passed {
		sc.PEVState.VerifyResult = types.VerifyResult{Passed: true}
	}

	assistantText = textutil.StripThinkingTags(assistantText)
	if assistantText == "" {
		if summary := summarizeSuccessfulToolResults(toolResults); summary != "" {
			assistantText = summary
			emit(&gateway.EngineEvent{
				Type: "text", Content: summary, SessionID: sc.SessionID,
				Metadata: map[string]string{"is_complete": "false", "source": "tool_fallback"},
			})
		}
	}

	duration := time.Since(start).Milliseconds()
	if emitComplete {
		emit(&gateway.EngineEvent{
			Type: "complete", SessionID: sc.SessionID,
			Metadata: map[string]string{
				"usage": fmt.Sprintf("%d", usage.PromptTokens+usage.CompletionTokens), "duration": fmt.Sprintf("%d", duration),
			},
		})
	}

	if runSpan != nil {
		runSpan.SetAttributes(
			tracer.Attribute{Key: "pev.duration_ms", Value: fmt.Sprintf("%d", duration)},
			tracer.Attribute{Key: "pev.total_tokens", Value: fmt.Sprintf("%d", usage.PromptTokens+usage.CompletionTokens)},
		)
	}

	if emitComplete {
		sc.PEVState.Phase = types.PEVPhaseDone
	}
	var msgs []types.Message
	if assistantText != "" {
		msgs = append(msgs, types.Message{Role: types.MessageRoleAssistant, Content: assistantText})
	}
	return &PEVRunResult{Usage: usage, Messages: msgs}, nil
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

func (e *PEVEngine) recordLLMCall(usage TokenUsage, latency time.Duration) {
	if e.llmTokens != nil {
		e.llmTokens.Add(int64(usage.PromptTokens + usage.CompletionTokens))
	}
	if e.llmLatency != nil {
		e.llmLatency.Observe(latency.Seconds())
	}
}

func (e *PEVEngine) recordLLMError() {
	if e.llmErrors != nil {
		e.llmErrors.Inc()
	}
}

func (e *PEVEngine) recordToolCall() {
	if e.toolCalls != nil {
		e.toolCalls.Inc()
	}
}

func (e *PEVEngine) recordToolError() {
	if e.toolErrors != nil {
		e.toolErrors.Inc()
	}
}

func pevErrorEvent(sessionID string, err *errors.SentinelError, recoverable bool) *gateway.EngineEvent {
	rec := "false"
	if recoverable {
		rec = "true"
	}
	return &gateway.EngineEvent{
		Type: "error", Content: err.Error(), SessionID: sessionID,
		Metadata: map[string]string{"code": err.Code, "recoverable": rec},
	}
}

func truncateSpanAttr(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func hasSuccessfulToolOutput(results []ToolResult) bool {
	for _, r := range results {
		if r.Error == "" && strings.TrimSpace(r.Output) != "" {
			return true
		}
	}
	return false
}

func summarizeSuccessfulToolResults(results []ToolResult) string {
	var parts []string
	for _, r := range results {
		if r.Error != "" || strings.TrimSpace(r.Output) == "" {
			continue
		}
		parts = append(parts, strings.TrimSpace(r.Output))
	}
	return strings.Join(parts, "\n")
}

