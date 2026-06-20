package turn

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	derrors "github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// SubTurnConfig DM-20260620-001-B Phase B (AC6 + AC9) — sub-agent
// 上下文隔离配置, 与 config.SubagentConfig 对齐, 供 SubTurnRunner 决策
// 默认 mode 与递归深度上限。
type SubTurnConfig struct {
	// DefaultMode 是 SubTurnRequest.Mode 为空时的回落值;
	// 取值 "brief" / "fork" / "full"。空 = "brief"。
	DefaultMode string
	// LegacyMode 显式覆盖; 缺省走 DefaultMode; 后续 minor release 移除。
	LegacyMode string
	// MaxDepth 子 agent 递归深度上限; Depth >= MaxDepth 时拒绝。
	// <=0 = 3。
	MaxDepth int
	// MaxContextTokens DM-20260620-002 (AC1) — sub-agent runLoop budget.
	// Wired into TurnRequest.MaxContextTokens when the inbound
	// SubTurnRequest does not set one, so nested runTokenAudit /
	// proactive fold / budgetTracker fire normally. <=0 = unset, lets
	// the orchestrator fall back to o.maxContextTokens (Phase A wiring).
	MaxContextTokens int
}

// ResolvedMode returns the effective default mode (LegacyMode overrides
// DefaultMode; both empty → "brief").
func (c SubTurnConfig) ResolvedMode() string {
	if c.LegacyMode != "" {
		return c.LegacyMode
	}
	if c.DefaultMode == "" {
		return "brief"
	}
	return c.DefaultMode
}

// ResolvedMaxDepth returns the effective max depth (<=0 → 3).
func (c SubTurnConfig) ResolvedMaxDepth() int {
	if c.MaxDepth <= 0 {
		return 3
	}
	return c.MaxDepth
}

// SubTurnRunner implements contracts.SubTurnExecutor via DefaultOrchestrator.RunTurn.
type SubTurnRunner struct {
	Orch TurnOrchestrator
	Cfg  SubTurnConfig
}

// NewSubTurnRunner constructs a SubTurnRunner with the given orchestrator
// and default config (back-compat: empty config → brief + MaxDepth=3).
func NewSubTurnRunner(orch TurnOrchestrator, cfg SubTurnConfig) *SubTurnRunner {
	return &SubTurnRunner{Orch: orch, Cfg: cfg}
}

var _ contracts.SubTurnExecutor = (*SubTurnRunner)(nil)

// RunSubTurn executes a nested turn synchronously by draining the event channel.
//
// Phase B (AC6 + AC9) — applies 3-mode dispatch + depth check:
//
//	mode=brief → PreloadedMessages=nil (子 agent 全新 history, 节省 token)
//	mode=fork  → PreloadedMessages=BuildForkedMessages (cache-friendly prefix)
//	mode=full  → PreloadedMessages=messagesWithoutLastUser (旧行为, 向后兼容)
//
// depth >= MaxDepth 返回 ErrSubagentDepthExceeded 并提示改 brief。
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

	mode, err := r.resolveMode(req.Mode)
	if err != nil {
		return nil, err
	}

	maxDepth := r.Cfg.ResolvedMaxDepth()
	if req.Depth >= maxDepth {
		return nil, derrors.NewSubagentDepthExceededError(req.Depth, maxDepth)
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

	preloaded, lastUser := r.applyMode(mode, req.Messages)

	// DM-20260620-002 (AC1) — propagate explicit budget. SubTurnRequest
	// wins when set, otherwise we fall back to Cfg so the nested runLoop
	// gets a non-zero maxContextTokens (otherwise runTokenAudit /
	// proactive fold are no-ops).
	maxCtx := req.MaxContextTokens
	if maxCtx <= 0 {
		maxCtx = r.Cfg.MaxContextTokens
	}

	ch, err := r.Orch.RunTurn(runCtx, TurnRequest{
		SessionID:         req.SessionID,
		UserMessage:       lastUser,
		SystemPrompt:      req.SystemPrompt,
		MaxTurns:          req.MaxTurns,
		Scope:             scope,
		PreloadedMessages: preloaded,
		OverrideTools:     mapToolSchemas(req.Tools),
		SkipPersist:       true,
		Model:             modelFromChild(req.ChildContext),
		MaxContextTokens:  maxCtx,
	})
	if err != nil {
		return nil, err
	}

	return collectSubTurnResult(ch, req.SessionID, emit)
}

// resolveMode returns the effective mode for a request (req.Mode > Cfg default)
// and rejects unknown values.
func (r *SubTurnRunner) resolveMode(reqMode contracts.SubAgentMode) (contracts.SubAgentMode, error) {
	if reqMode == "" {
		return contracts.SubAgentMode(r.Cfg.ResolvedMode()), nil
	}
	switch reqMode {
	case contracts.SubAgentModeBrief, contracts.SubAgentModeFork, contracts.SubAgentModeFull:
		return reqMode, nil
	default:
		return "", derrors.NewSubagentInvalidModeError(string(reqMode))
	}
}

// applyMode returns (PreloadedMessages, UserMessage) for the given mode.
//
//	brief → (nil, lastUser)
//	fork  → (BuildForkedMessages(parentMsgs, directive), directiveUser)
//	full  → (messagesWithoutLastUser(msgs), lastUser)
func (r *SubTurnRunner) applyMode(mode contracts.SubAgentMode, msgs []types.Message) ([]types.Message, types.Message) {
	lastUser := lastUserMessage(msgs)
	switch mode {
	case contracts.SubAgentModeBrief:
		return nil, lastUser
	case contracts.SubAgentModeFork:
		directive := lastUser.Content
		parent := messagesWithoutLastUser(msgs)
		forked := conversation.BuildForkedMessages(directive, parent)
		return forked, lastUser
	default: // full
		return messagesWithoutLastUser(msgs), lastUser
	}
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
