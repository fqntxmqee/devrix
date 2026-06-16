package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/orchestration/turn"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// contextEngineAdapter implements turn.ContextPreparer, turn.ToolRoundExecutor,
// and turn.SessionPersister by delegating to the context engine internals.
//
// DM-020 D-c+d+e: temporary adapter that bridges orchestration interfaces to
// the existing context engine. Token counter (D-e) enables CompressHint generation.
type contextEngineAdapter struct {
	gw       *capture.CommunicationGateway
	engine   contracts.IEngine
	tools    contextengine.IToolRunner
	toolsReg contextengine.IToolRegistry
	perm     contracts.IPermissionGate
	counter  contracts.ITokenCounter
}

func newContextEngineAdapter(gw *capture.CommunicationGateway, engine contracts.IEngine, counter contracts.ITokenCounter) *contextEngineAdapter {
	a := &contextEngineAdapter{gw: gw, engine: engine, counter: counter}
	if ce, ok := engine.(*contextengine.ContextEngine); ok {
		a.tools = ce.ToolRunner()
		a.toolsReg = ce.ToolRegistry()
		a.perm = ce.PermissionGate()
	}
	return a
}

// compressThreshold is the per-message token budget above which a CompressHint is generated.
const compressThreshold = 4000

// Prepare implements turn.ContextPreparer.
// D-e: checks token budget and returns CompressHint when exceeded.
func (a *contextEngineAdapter) Prepare(ctx context.Context, req turn.PrepareRequest) (turn.PreparedContext, error) {
	session, err := a.gw.GetSession(req.SessionID)
	if err != nil {
		session = types.NewSession(req.SessionID, "d7", "")
	}

	var toolSchemas []turn.ToolSchema
	if a.toolsReg != nil {
		schemas, err := a.toolsReg.ListTools(ctx, session.WorkDir)
		if err == nil {
			for _, s := range schemas {
				params := parseToolParams(s.Parameters)
				toolSchemas = append(toolSchemas, turn.ToolSchema{
					Name:        s.Name,
					Description: s.Description,
					Parameters:  params,
				})
			}
		}
	}

	result := turn.PreparedContext{
		Messages: []types.Message{req.Message},
		Tools:    toolSchemas,
	}
	if ce, ok := a.engine.(*contextengine.ContextEngine); ok {
		if sc, ok := ce.SessionContext(req.SessionID); ok && sc != nil {
			result.Model = sc.Model
			result.MaxContextTokens = sc.TokenBudget.MaxContextTokens
		}
	}
	if session != nil && session.Model != "" {
		result.Model = session.Model
	}

	// D-e: check token budget and generate CompressHint if needed.
	if a.counter != nil && len(result.Messages) > 0 {
		tokens := a.counter.CountMessages(result.Messages)
		if tokens > compressThreshold {
			result.CompressHint = &turn.CompressHint{
				MessagesToSummarize: result.Messages,
				TargetTokenBudget:   compressThreshold / 2,
			}
		}
	}

	return result, nil
}

func parseToolParams(raw string) map[string]any {
	if raw == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return map[string]any{"raw": raw}
	}
	return m
}

// ExecuteRound implements turn.ToolRoundExecutor.
func (a *contextEngineAdapter) ExecuteRound(ctx context.Context, req turn.ToolRoundRequest) (turn.ToolRoundResult, error) {
	if a.tools == nil {
		return turn.ToolRoundResult{}, fmt.Errorf("turn adapter: tool runner not available")
	}

	results := make([]turn.ToolResult, len(req.ToolCalls))
	for i, tc := range req.ToolCalls {
		result, err := a.tools.Execute(ctx, contextengine.ToolCall{
			ID:    tc.ID,
			Name:  tc.Name,
			Input: tc.Input,
		})
		if err != nil {
			results[i] = turn.ToolResult{
				ToolCallID: tc.ID,
				Error:      err.Error(),
			}
		} else {
			results[i] = turn.ToolResult{
				ToolCallID: tc.ID,
				Output:     result.Output,
				Error:      result.Error,
			}
		}
	}
	return turn.ToolRoundResult{Results: results}, nil
}

// PersistTurn implements turn.SessionPersister.
func (a *contextEngineAdapter) PersistTurn(ctx context.Context, req turn.PersistRequest) error {
	if ce, ok := a.engine.(*contextengine.ContextEngine); ok {
		_, err := ce.ExportSessionSnapshot(req.SessionID)
		return err
	}
	return nil
}
