package turn

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// SubTurnRunner implements contracts.SubTurnExecutor via DefaultOrchestrator.RunTurn.
type SubTurnRunner struct {
	Orch TurnOrchestrator
}

// NewSubTurnRunner constructs a SubTurnRunner.
func NewSubTurnRunner(orch TurnOrchestrator) *SubTurnRunner {
	return &SubTurnRunner{Orch: orch}
}

var _ contracts.SubTurnExecutor = (*SubTurnRunner)(nil)

// RunSubTurn executes a nested turn synchronously by draining the event channel.
func (r *SubTurnRunner) RunSubTurn(ctx context.Context, req contracts.SubTurnRequest) (*contracts.SubTurnResult, error) {
	if r == nil || r.Orch == nil {
		return nil, fmt.Errorf("subturn: orchestrator is nil")
	}
	if req.SessionID == "" {
		return nil, fmt.Errorf("subturn: SessionID is required")
	}
	if len(req.Messages) == 0 {
		return nil, fmt.Errorf("subturn: at least one message is required")
	}

	scope := mapSubTurnScope(req.Scope)
	runCtx := ctx
	if req.ChildContext != nil {
		runCtx = contracts.WithSubAgentSession(ctx, req.ChildContext)
	}

	emit := req.Emit
	if req.FlowReporter != nil {
		emit = req.FlowReporter.WrapEmit(ctx, req.FlowParams, emit)
	}

	ch, err := r.Orch.RunTurn(runCtx, TurnRequest{
		SessionID:         req.SessionID,
		UserMessage:       lastUserMessage(req.Messages),
		SystemPrompt:      req.SystemPrompt,
		MaxTurns:          req.MaxTurns,
		Scope:             scope,
		PreloadedMessages: messagesWithoutLastUser(req.Messages),
		OverrideTools:     mapToolSchemas(req.Tools),
		SkipPersist:       true,
		Model:             modelFromChild(req.ChildContext),
	})
	if err != nil {
		return nil, err
	}

	return collectSubTurnResult(ch, req.SessionID, emit)
}

// PreparedTurnAdapter implements contracts.PreparedTurnRunner for engine.Process.
type PreparedTurnAdapter struct {
	Orch TurnOrchestrator
}

// NewPreparedTurnAdapter constructs a PreparedTurnAdapter.
func NewPreparedTurnAdapter(orch TurnOrchestrator) *PreparedTurnAdapter {
	return &PreparedTurnAdapter{Orch: orch}
}

var _ contracts.PreparedTurnRunner = (*PreparedTurnAdapter)(nil)

// RunPreparedTurn runs a main-scope turn with pre-assembled context from D2 Process.
func (a *PreparedTurnAdapter) RunPreparedTurn(ctx context.Context, req contracts.PreparedTurnRequest) (*contracts.PreparedTurnResult, error) {
	if a == nil || a.Orch == nil {
		return nil, fmt.Errorf("prepared turn: orchestrator is nil")
	}
	if req.SessionID == "" {
		return nil, fmt.Errorf("prepared turn: SessionID is required")
	}
	if len(req.Messages) == 0 {
		return nil, fmt.Errorf("prepared turn: at least one message is required")
	}

	ch, err := a.Orch.RunTurn(ctx, TurnRequest{
		SessionID:         req.SessionID,
		UserMessage:       lastUserMessage(req.Messages),
		SystemPrompt:      req.SystemPrompt,
		MaxTurns:          req.MaxTurns,
		Scope:             TurnScopeMain,
		PreloadedMessages: messagesWithoutLastUser(req.Messages),
		OverrideTools:     mapToolSchemas(req.Tools),
	})
	if err != nil {
		return nil, err
	}

	sub, err := collectSubTurnResult(ch, req.SessionID, req.Emit)
	if err != nil {
		return nil, err
	}
	return &contracts.PreparedTurnResult{
		AssistantText:   sub.AssistantText,
		Usage:           sub.Usage,
		TurnCount:       sub.TurnCount,
		ToolCallHistory: sub.ToolCallHistory,
	}, nil
}

func mapSubTurnScope(scope contracts.SubTurnScope) TurnScope {
	switch scope {
	case contracts.SubTurnScopeBackground:
		return TurnScopeBackground
	case contracts.SubTurnScopeWaveWorker:
		return TurnScopeWaveWorker
	default:
		return TurnScopeSubQuery
	}
}

func mapToolSchemas(in []contracts.ToolSchema) []ToolSchema {
	if len(in) == 0 {
		return nil
	}
	out := make([]ToolSchema, len(in))
	for i, t := range in {
		out[i] = ToolSchema{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  parseToolParamsMap(t.Parameters),
		}
	}
	return out
}

func parseToolParamsMap(raw string) map[string]any {
	if raw == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return map[string]any{"raw": raw}
	}
	return m
}

func lastUserMessage(msgs []types.Message) types.Message {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == types.MessageRoleUser {
			return msgs[i]
		}
	}
	return msgs[len(msgs)-1]
}

func messagesWithoutLastUser(msgs []types.Message) []types.Message {
	if len(msgs) == 0 {
		return nil
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == types.MessageRoleUser {
			if i == 0 {
				return nil
			}
			out := make([]types.Message, i)
			copy(out, msgs[:i])
			return out
		}
	}
	out := make([]types.Message, len(msgs)-1)
	copy(out, msgs[:len(msgs)-1])
	return out
}

func modelFromChild(sc *types.SessionContext) string {
	if sc == nil {
		return ""
	}
	return sc.Model
}

func collectSubTurnResult(ch <-chan *contracts.EngineEvent, sessionID string, emit contracts.EngineEmitFunc) (*contracts.SubTurnResult, error) {
	var (
		assistantText strings.Builder
		turnCount     int
		usage         contracts.TokenUsage
		toolHistory   []types.ToolCallRecord
		outMsgs       []types.Message
	)
	for ev := range ch {
		if ev == nil {
			continue
		}
		if emit != nil && ev.Type != "complete" {
			emit(ev)
		}
		switch ev.Type {
		case "text":
			if ev.Metadata["is_complete"] != "false" {
				assistantText.WriteString(ev.Content)
			}
		case "tool_call":
			toolHistory = append(toolHistory, types.ToolCallRecord{
				CallID:   ev.Metadata["tool_call_id"],
				ToolName: ev.ToolName,
				Input:    ev.ToolInput,
			})
			turnCount++
		case "error":
			return nil, fmt.Errorf("subturn: %s", ev.Content)
		case "complete":
			if strings.TrimSpace(ev.Content) != "" {
				assistantText.Reset()
				assistantText.WriteString(ev.Content)
			}
			if ev.Metadata != nil {
				if u := ev.Metadata["usage"]; u != "" {
					var total int
					if _, err := fmt.Sscanf(u, "%d", &total); err == nil && total > 0 {
						usage.CompletionTokens = total
					}
				}
			}
			text := strings.TrimSpace(assistantText.String())
			if text != "" {
				outMsgs = []types.Message{{
					Role:      types.MessageRoleAssistant,
					Content:   text,
					SessionID: sessionID,
				}}
			}
			return &contracts.SubTurnResult{
				AssistantText:   text,
				Messages:        outMsgs,
				TurnCount:       turnCount,
				ToolCallHistory: toolHistory,
				Usage:           usage,
			}, nil
		}
	}
	return nil, fmt.Errorf("subturn: turn ended without complete event")
}

type noopPersister struct{}

func (noopPersister) PersistTurn(_ context.Context, _ PersistRequest) error { return nil }

func isNestedScope(scope TurnScope) bool {
	switch scope {
	case TurnScopeSubQuery, TurnScopeBackground, TurnScopeWaveWorker:
		return true
	default:
		return false
	}
}
