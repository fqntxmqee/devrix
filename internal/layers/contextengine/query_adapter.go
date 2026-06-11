package contextengine

import (
	"context"

	"github.com/devrix/devrix/internal/layers/contextengine/query"
	"github.com/devrix/devrix/internal/shared/types"
)

// llmCaller adapts ILLMGateway to query.LLMCaller.
type llmCaller struct {
	llm ILLMGateway
}

func (a *llmCaller) Call(ctx context.Context, req query.LLMRequest) (<-chan query.LLMChunk, error) {
	tools := make([]ToolSchema, len(req.Tools))
	for i, t := range req.Tools {
		tools[i] = ToolSchema{Name: t.Name, Description: t.Description, Parameters: t.Parameters}
	}
	ch, err := a.llm.ChatStream(ctx, &LLMRequest{
		Model:        req.Model,
		SystemPrompt: req.SystemPrompt,
		Messages:     req.Messages,
		Tools:        tools,
	})
	if err != nil {
		return nil, err
	}
	out := make(chan query.LLMChunk, 8)
	go func() {
		defer close(out)
		for c := range ch {
			chunk := query.LLMChunk{
				Content: c.Content, Thinking: c.Thinking, Done: c.Done,
				Usage: query.TokenUsage{
					PromptTokens: c.Usage.PromptTokens, CompletionTokens: c.Usage.CompletionTokens,
				},
			}
			if len(c.ToolCalls) > 0 {
				chunk.ToolCalls = make([]query.ToolCall, len(c.ToolCalls))
				for i, tc := range c.ToolCalls {
					chunk.ToolCalls[i] = query.ToolCall{ID: tc.ID, Name: tc.Name, Input: tc.Input}
				}
			}
			out <- chunk
		}
	}()
	return out, nil
}

// toolExecutor adapts IToolRunner to query.ToolExecutor.
type toolExecutor struct {
	tools      IToolRunner
	toolsReg   IToolRegistry
}

func (e *toolExecutor) Execute(ctx context.Context, call query.ToolCall) (string, string, error) {
	tc := ToolCall{ID: call.ID, Name: call.Name, Input: call.Input}
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

// permChecker adapts IPermissionGate.
type permChecker struct {
	gate IPermissionGate
	reg  IToolRegistry
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

func toolSchemasToQuery(tools []ToolSchema) []query.ToolSchema {
	out := make([]query.ToolSchema, len(tools))
	for i, t := range tools {
		out[i] = query.ToolSchema{Name: t.Name, Description: t.Description, Parameters: t.Parameters}
	}
	return out
}

func queryUsage(u query.TokenUsage) TokenUsage {
	return TokenUsage{PromptTokens: u.PromptTokens, CompletionTokens: u.CompletionTokens}
}
