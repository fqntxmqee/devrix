package mockctx

import (
	"context"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// StaticPreparedTurnRunner is a test double for contracts.PreparedTurnRunner.
//
// Cross-domain fixture: D7-S2-A06 RunTurn is the production implementation;
// D2 (legacy/ + prepare.Orchestrator) consumes via EngineDeps.PreparedTurnRunner.
// Lives in D2/mock/ because D2 tests need a fake and D2 cannot import D7.
type StaticPreparedTurnRunner struct {
	Response string
	Err      error
	ToolName string
	ToolInput string
}

// RunPreparedTurn implements contracts.PreparedTurnRunner.
//
// Emits a complete EngineEvent stream (text/tool_call → complete) when req.Emit
// is non-nil. The streaming text chunks carry is_complete="false" so the
// engine's working-memory aggregator flushes them into sc.Messages on persist.
func (m *StaticPreparedTurnRunner) RunPreparedTurn(ctx context.Context, req contracts.PreparedTurnRequest) (*contracts.PreparedTurnResult, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	text := m.Response
	if text == "" {
		text = "I can help you with that."
	}
	emitTool := m.ToolName != ""
	if req.Emit != nil {
		if emitTool {
			req.Emit(&contracts.EngineEvent{
				Type:      "tool_call",
				SessionID: req.SessionID,
				ToolName:  m.ToolName,
				ToolInput: m.ToolInput,
			})
		} else {
			req.Emit(&contracts.EngineEvent{
				Type:      "text",
				Content:   text,
				SessionID: req.SessionID,
				Metadata:  map[string]string{"is_complete": "false"},
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
	if emitTool {
		history = []types.ToolCallRecord{{
			ToolName: m.ToolName,
			Input:    m.ToolInput,
		}}
	}
	return &contracts.PreparedTurnResult{
		AssistantText:   text,
		Usage:           contracts.TokenUsage{PromptTokens: 10, CompletionTokens: 5},
		ToolCallHistory: history,
	}, nil
}

// PreparedTurnRunnerWithTools returns a runner that emits a bash tool call.
//
// Convenience constructor for tests that want to exercise the tool_call path.
func PreparedTurnRunnerWithTools() contracts.PreparedTurnRunner {
	return &StaticPreparedTurnRunner{
		ToolName:  "bash",
		ToolInput: "ls",
	}
}
