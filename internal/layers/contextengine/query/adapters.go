package query

import (
	"context"

	"github.com/devrix/devrix/internal/layers/contextengine/toolrunner"
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// NewLLMCaller adapts llmgateway.ILLMGateway to LLMCaller.
//
// DSAFT: D2-S10-A01-F01 (WireQueryLoop)
func NewLLMCaller(llm llmgateway.ILLMGateway) LLMCaller {
	return &llmCaller{llm: llm}
}

// NewToolExecutor adapts toolrunner.IToolRunner to ToolExecutor.
func NewToolExecutor(tools toolrunner.IToolRunner, toolsReg toolrunner.IToolRegistry) ToolExecutor {
	return &toolExecutor{tools: tools, toolsReg: toolsReg}
}

// NewPermChecker adapts contracts.IPermissionGate to PermissionChecker.
func NewPermChecker(gate contracts.IPermissionGate, reg toolrunner.IToolRegistry) PermissionChecker {
	return &permChecker{gate: gate, reg: reg}
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
	tools    toolrunner.IToolRunner
	toolsReg toolrunner.IToolRegistry
}

func (e *toolExecutor) Execute(ctx context.Context, call ToolCall) (string, string, error) {
	tc := toolrunner.ToolCall{ID: call.ID, Name: call.Name, Input: call.Input}
	if e.toolsReg != nil {
		tc.RiskLevel = e.toolsReg.RiskLevel(call.Name)
	}
	res, err := e.tools.Execute(ctx, tc)
	if err != nil {
		return "", err.Error(), err
	}
	if res == nil {
		return "", "", nil
	}
	return res.Output, res.Error, nil
}

type permChecker struct {
	gate contracts.IPermissionGate
	reg  toolrunner.IToolRegistry
}

func (p *permChecker) Request(ctx context.Context, sessionID, toolName, input string) bool {
	if p.gate == nil {
		return true
	}
	risk := types.RiskLevelLow
	if p.reg != nil {
		risk = p.reg.RiskLevel(toolName)
	}
	return p.gate.Request(ctx, sessionID, toolName, input, risk)
}
