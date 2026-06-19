package mockctx

import (
	"context"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// StaticPreparedTurnRunner is a test double for contracts.PreparedTurnRunner.
type StaticPreparedTurnRunner struct {
	Response string
	Err      error
	ToolCall *contracts.ToolCall
}

// RunPreparedTurn implements contracts.PreparedTurnRunner.
func (m *StaticPreparedTurnRunner) RunPreparedTurn(ctx context.Context, req contracts.PreparedTurnRequest) (*contracts.PreparedTurnResult, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	text := m.Response
	if text == "" {
		text = "I can help you with that."
	}
	if req.Emit != nil {
		if m.ToolCall != nil {
			req.Emit(&contracts.EngineEvent{
				Type:      "tool_call",
				SessionID: req.SessionID,
				ToolName:  m.ToolCall.Name,
				ToolInput: m.ToolCall.Input,
			})
		} else {
			req.Emit(&contracts.EngineEvent{
				Type:      "text",
				Content:   text,
				SessionID: req.SessionID,
			})
		}
		req.Emit(&contracts.EngineEvent{
			Type:      "complete",
			Content:   text,
			SessionID: req.SessionID,
			Metadata:  map[string]string{"usage": "15"},
		})
	}
	var history []types.ToolCallRecord
	if m.ToolCall != nil {
		history = []types.ToolCallRecord{{
			CallID:   m.ToolCall.ID,
			ToolName: m.ToolCall.Name,
			Input:    m.ToolCall.Input,
		}}
	}
	return &contracts.PreparedTurnResult{
		AssistantText:   text,
		Usage:           contracts.TokenUsage{PromptTokens: 10, CompletionTokens: 5},
		ToolCallHistory: history,
	}, nil
}

// PreparedTurnRunnerWithTools returns a runner that emits a bash tool call.
func PreparedTurnRunnerWithTools() contracts.PreparedTurnRunner {
	return &StaticPreparedTurnRunner{
		ToolCall: &contracts.ToolCall{ID: "tc1", Name: "bash", Input: "ls"},
	}
}

// PreparedTurnRunnerFromCaller adapts a legacy StaticLLMCaller-style response.
func PreparedTurnRunnerFromCaller(caller *StaticLLMCaller) contracts.PreparedTurnRunner {
	if caller == nil {
		return &StaticPreparedTurnRunner{}
	}
	return &StaticPreparedTurnRunner{
		Response: caller.Response,
		Err:      caller.Err,
		ToolCall: caller.ToolCall,
	}
}
