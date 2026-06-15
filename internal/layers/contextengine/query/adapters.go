package query

import (
	"context"
	"fmt"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/policy/toolrunner"
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// NewLLMCaller adapts llmgateway.ILLMGateway to LLMCaller.
//
// DSAFT: D2-S10-A01-F01 (WireQueryLoop)
//
// Deprecated: Use turn.QueryLLMCaller via EngineDeps.QueryLLMCaller (DM-020).
// Production wiring must inject the D7-supplied adapter; this local adapter
// remains only as a fallback when neither EngineDeps.QueryLLMCaller nor
// EngineDeps.LLM is supplied.
func NewLLMCaller(llm llmgateway.ILLMGateway) LLMCaller {
	return &llmCaller{llm: llm}
}

// NewToolExecutor adapts toolrunner.IToolRunner to ToolExecutor.
func NewToolExecutor(tools toolrunner.IToolRunner, toolsReg toolrunner.IToolRegistry, obsBridge *observability.Bridge) ToolExecutor {
	return &toolExecutor{tools: tools, toolsReg: toolsReg, obsBridge: obsBridge}
}

// NewPermChecker adapts contracts.IPermissionGate to PermissionChecker.
func NewPermChecker(gate contracts.IPermissionGate, reg toolrunner.IToolRegistry, obsBridge *observability.Bridge) PermissionChecker {
	return &permChecker{gate: gate, reg: reg, obsBridge: obsBridge}
}

// ToolSchemasFromRunner converts toolrunner schemas to query-local schemas.
func ToolSchemasFromRunner(tools []toolrunner.ToolSchema) []ToolSchema {
	out := make([]ToolSchema, len(tools))
	for i, t := range tools {
		out[i] = ToolSchema{Name: t.Name, Description: t.Description, Parameters: t.Parameters}
	}
	return out
}

type llmCaller struct {
	llm llmgateway.ILLMGateway
}

func (a *llmCaller) Call(ctx context.Context, req LLMRequest) (<-chan LLMChunk, error) {
	tools := make([]llmgateway.ToolSchema, len(req.Tools))
	for i, t := range req.Tools {
		tools[i] = llmgateway.ToolSchema{Name: t.Name, Description: t.Description, Parameters: t.Parameters}
	}
	ch, err := a.llm.ChatStream(ctx, &llmgateway.Request{
		Model:        req.Model,
		SystemPrompt: req.SystemPrompt,
		Messages:     req.Messages,
		Tools:        tools,
		Stream:       true,
	})
	if err != nil {
		return nil, err
	}
	out := make(chan LLMChunk, 8)
	go func() {
		defer close(out)
		for c := range ch {
			chunk := LLMChunk{
				Content: c.Content, Thinking: c.Thinking, Done: c.Done,
				Usage: TokenUsage{
					PromptTokens: c.Usage.PromptTokens, CompletionTokens: c.Usage.CompletionTokens,
				},
			}
			if len(c.ToolCalls) > 0 {
				chunk.ToolCalls = make([]ToolCall, len(c.ToolCalls))
				for i, tc := range c.ToolCalls {
					chunk.ToolCalls[i] = ToolCall{ID: tc.ID, Name: tc.Name, Input: tc.Input}
				}
			}
			out <- chunk
		}
	}()
	return out, nil
}

type toolExecutor struct {
	tools     toolrunner.IToolRunner
	toolsReg  toolrunner.IToolRegistry
	obsBridge *observability.Bridge
}

func (e *toolExecutor) Execute(ctx context.Context, call ToolCall) (string, string, error) {
	start := time.Now()
	tc := toolrunner.ToolCall{ID: call.ID, Name: call.Name, Input: call.Input}
	if e.toolsReg != nil {
		tc.RiskLevel = e.toolsReg.RiskLevel(call.Name)
	}

	// [trace] tool.execute.single
	ctx, toolSpan := startToolSpan(ctx, e.obsBridge, telemetry.OpD2_S5_Tool_Execute_Single,
		tracer.Attribute{Key: "tool.name", Value: call.Name},
		tracer.Attribute{Key: "tool.input", Value: truncateStr(call.Input, 500)},
		tracer.Attribute{Key: "tool.risk_level", Value: string(tc.RiskLevel)},
	)

	res, err := e.tools.Execute(ctx, tc)

	if toolSpan != nil {
		toolSpan.SetAttributes(
			tracer.Attribute{Key: "tool.duration_ms", Value: fmt.Sprintf("%d", time.Since(start).Milliseconds())},
		)
		if res != nil {
			toolSpan.SetAttributes(
				tracer.Attribute{Key: "tool.output_size", Value: fmt.Sprintf("%d", len(res.Output))},
			)
			if res.Error != "" {
				toolSpan.SetAttributes(tracer.Attribute{Key: "tool.error", Value: truncateStr(res.Error, 200)})
			}
		}
		if err != nil {
			toolSpan.SetAttributes(tracer.Attribute{Key: "tool.error", Value: truncateStr(err.Error(), 200)})
		}
		toolSpan.End()
	}
	if err != nil {
		return "", err.Error(), err
	}
	if res == nil {
		return "", "", nil
	}
	return res.Output, res.Error, nil
}

// startToolSpan creates a child span for tool execution when observability is configured.
func startToolSpan(ctx context.Context, obsBridge *observability.Bridge, operation string, attrs ...tracer.Attribute) (context.Context, tracer.Span) {
	if obsBridge == nil || !obsBridge.IsEnabled() {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(operation, attrs...)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return obsBridge.Tracer().Start(ctx, operation, opts...)
}

// truncateStr truncates a string to maxLen bytes, appending "..." if truncated.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

type permChecker struct {
	gate      contracts.IPermissionGate
	reg       toolrunner.IToolRegistry
	obsBridge *observability.Bridge
}

func (p *permChecker) Request(ctx context.Context, sessionID, toolName, input string) bool {
	_, span := startToolSpan(ctx, p.obsBridge, telemetry.OpD2_S5_Tool_Execute_Permission,
		tracer.Attribute{Key: "tool.name", Value: toolName},
		tracer.Attribute{Key: "tool.session_id", Value: sessionID},
	)
	if span != nil {
		defer span.End()
	}

	if p.gate == nil {
		return true
	}
	risk := types.RiskLevelLow
	if p.reg != nil {
		risk = p.reg.RiskLevel(toolName)
	}
	allowed := p.gate.Request(ctx, sessionID, toolName, input, risk)
	if span != nil {
		span.SetAttributes(
			tracer.Attribute{Key: "tool.risk_level", Value: string(risk)},
			tracer.Attribute{Key: "tool.permitted", Value: fmt.Sprintf("%t", allowed)},
		)
	}
	return allowed
}
