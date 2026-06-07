package contextengine

import (
	"context"
	"fmt"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

// PEVEngine runs the PEV Execute→Verify loop.
type PEVEngine struct {
	llm        ILLMGateway
	tools      IToolRunner
	toolsReg   IToolRegistry
	permission IPermissionGate
	observer   IObserver
	cfg        *config.PEVConfig
}

// NewPEVEngine creates a PEV engine.
func NewPEVEngine(
	llm ILLMGateway,
	tools IToolRunner,
	toolsReg IToolRegistry,
	permission IPermissionGate,
	observer IObserver,
	cfg *config.PEVConfig,
) *PEVEngine {
	if cfg == nil {
		cfg = &config.PEVConfig{MaxIterations: 3, VerifyMode: "basic"}
	}
	if observer == nil {
		observer = NoOpObserver{}
	}
	return &PEVEngine{
		llm:        llm,
		tools:      tools,
		toolsReg:   toolsReg,
		permission: permission,
		observer:   observer,
		cfg:        cfg,
	}
}

// PEVRunResult holds PEV output.
type PEVRunResult struct {
	Messages []types.Message
	Usage    TokenUsage
}

// Run executes PEV loop and emits gateway events.
func (e *PEVEngine) Run(ctx context.Context, sc *types.SessionContext, view []types.Message, emit func(*gateway.EngineEvent)) (*PEVRunResult, error) {
	start := time.Now()
	maxIter := e.cfg.MaxIterations
	if maxIter <= 0 {
		maxIter = 3
	}

	toolSchemas, _ := e.toolsReg.ListTools(ctx, sc.WorkDir)
	req := &LLMRequest{
		Model:        sc.Model,
		SystemPrompt: sc.SystemPrompt,
		Messages:     view,
		Tools:        toolSchemas,
	}

	var assistantText string
	var toolResults []ToolResult
	var usage TokenUsage

	for iter := 0; iter < maxIter; iter++ {
		sc.PEVState.Phase = types.PEVPhaseExecute
		sc.PEVState.Iteration = iter
		e.observer.EmitPEVIteration(sc.SessionID, iter, types.PEVPhaseExecute)

		chunks, err := e.llm.ChatStream(ctx, req)
		if err != nil {
			return nil, errors.NewLLMUnavailableError(err)
		}

		var pendingTools []ToolCall
		for chunk := range chunks {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			if chunk.Thinking != "" {
				emit(&gateway.EngineEvent{Type: "thinking", Content: chunk.Thinking, SessionID: sc.SessionID})
			}
			if chunk.Content != "" {
				assistantText += chunk.Content
				emit(&gateway.EngineEvent{
					Type: "text", Content: chunk.Content, SessionID: sc.SessionID,
					Metadata: map[string]string{"is_complete": "false"},
				})
			}
			if len(chunk.ToolCalls) > 0 {
				pendingTools = append(pendingTools, chunk.ToolCalls...)
			}
			if chunk.Done {
				usage = chunk.Usage
			}
		}

		if len(pendingTools) == 0 {
			break
		}

		for _, tc := range pendingTools {
			risk := tc.RiskLevel
			if risk == "" {
				risk = e.toolsReg.RiskLevel(tc.Name)
			}
			emit(&gateway.EngineEvent{
				Type: "tool_call", ToolName: tc.Name, ToolInput: tc.Input, SessionID: sc.SessionID,
				Metadata: map[string]string{"tool_name": tc.Name, "input": tc.Input, "risk_level": string(risk)},
			})

			if e.permission != nil && !e.permission.Request(ctx, sc.SessionID, tc.Name, tc.Input, risk) {
				emit(pevErrorEvent(sc.SessionID, errors.NewContextPermissionDeniedError(tc.Name), false))
				return &PEVRunResult{Usage: usage}, errors.NewContextPermissionDeniedError(tc.Name)
			}

			result, err := e.tools.Execute(ctx, tc)
			if err != nil {
				result = &ToolResult{Error: err.Error()}
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

			toolMsg := types.NewMessage(fmt.Sprintf("tool_%d", time.Now().UnixNano()), sc.SessionID, types.MessageRoleTool, content)
			req.Messages = append(req.Messages, *toolMsg)
		}

		sc.PEVState.Phase = types.PEVPhaseVerify
		e.observer.EmitPEVPhase(sc.SessionID, types.PEVPhaseVerify, iter)
		vr := verifyPEV(e.cfg.VerifyMode, toolResults)
		sc.PEVState.VerifyResult = vr
		if vr.Passed {
			break
		}
		if iter == maxIter-1 {
			emit(pevErrorEvent(sc.SessionID, errors.NewPEVMaxIterationsError(), true))
			return &PEVRunResult{Usage: usage}, errors.NewPEVMaxIterationsError()
		}
	}

	if assistantText != "" {
		emit(&gateway.EngineEvent{
			Type: "text", Content: assistantText, SessionID: sc.SessionID,
			Metadata: map[string]string{"is_complete": "true"},
		})
	}

	duration := time.Since(start).Milliseconds()
	emit(&gateway.EngineEvent{
		Type: "complete", SessionID: sc.SessionID,
		Metadata: map[string]string{
			"usage": fmt.Sprintf("%d", usage.PromptTokens+usage.CompletionTokens), "duration": fmt.Sprintf("%d", duration),
		},
	})

	sc.PEVState.Phase = types.PEVPhaseDone
	var msgs []types.Message
	if assistantText != "" {
		msgs = append(msgs, types.Message{Role: types.MessageRoleAssistant, Content: assistantText})
	}
	return &PEVRunResult{Usage: usage, Messages: msgs}, nil
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

func verifyPEV(mode string, results []ToolResult) types.VerifyResult {
	if mode == "none" {
		return types.VerifyResult{Passed: true}
	}
	for _, r := range results {
		if r.Error != "" {
			return types.VerifyResult{Passed: false, Deviation: 1}
		}
		if mode == "basic" && r.Output == "" {
			return types.VerifyResult{Passed: false, Deviation: 0.5}
		}
	}
	return types.VerifyResult{Passed: true}
}
