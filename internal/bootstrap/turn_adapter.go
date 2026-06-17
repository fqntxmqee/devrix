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
//
// TOOL-SURFACE-1 W9: ExecuteRound routes tool calls through the engine's
// surface list when available (TOOL-SURFACE-1-A03 dispatch path) and
// falls back to the legacy IToolRunner when the engine was built without
// surfaces (phase-1 back-compat).
type contextEngineAdapter struct {
	gw       *capture.CommunicationGateway
	engine   contracts.IEngine
	tools    contextengine.IToolRunner
	toolsReg contextengine.IToolRegistry
	perm     contracts.IPermissionGate
	counter  contracts.ITokenCounter
	surfaces []contracts.ToolSurface
}

func newContextEngineAdapter(gw *capture.CommunicationGateway, engine contracts.IEngine, counter contracts.ITokenCounter) *contextEngineAdapter {
	a := &contextEngineAdapter{gw: gw, engine: engine, counter: counter}
	if ce, ok := engine.(*contextengine.ContextEngine); ok {
		a.tools = ce.ToolRunner()
		a.toolsReg = ce.ToolRegistry()
		a.perm = ce.PermissionGate()
		// TOOL-SURFACE-1 (W9): engine may have a non-nil surface list
		// (built by bootstrap.BuildSurfaces in W8). When present, the
		// surface dispatch path is the primary one; the legacy tools
		// field is still kept for any caller that needs the IToolRunner
		// view (e.g. Prepare's ListTools call).
		if ce.HasSurfaces() {
			a.surfaces = ce.Surfaces()
		}
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
		Tools: toolSchemas,
	}
	if prov, ok := a.engine.(sessionContextProvider); ok {
		if sc, ok := prov.SessionContext(req.SessionID); ok && sc != nil {
			result.Messages = copySessionMessages(sc.Messages)
			result.Model = sc.Model
			result.MaxContextTokens = sc.TokenBudget.MaxContextTokens
		}
	}
	if session != nil && session.Model != "" {
		result.Model = session.Model
	}

	// D-e: check token budget and generate CompressHint when exceeded.
	// Count history + current turn; TurnOrchestrator appends req.Message separately.
	if a.counter != nil {
		toCount := append([]types.Message(nil), result.Messages...)
		if req.Message.Content != "" || req.Message.Role != "" {
			toCount = append(toCount, req.Message)
		}
		if len(toCount) > 0 {
			tokens := a.counter.CountMessages(toCount)
			if tokens > compressThreshold {
				result.CompressHint = &turn.CompressHint{
					MessagesToSummarize: result.Messages,
					TargetTokenBudget:   compressThreshold / 2,
				}
			}
		}
	}

	return result, nil
}

// sessionContextProvider is implemented by *contextengine.ContextEngine.
type sessionContextProvider interface {
	SessionContext(sessionID string) (*types.SessionContext, bool)
}

func copySessionMessages(msgs []types.Message) []types.Message {
	if len(msgs) == 0 {
		return nil
	}
	out := make([]types.Message, len(msgs))
	copy(out, msgs)
	return out
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
//
// DM-20260617-004 (devrix-d7-tool-ctx-inject): D7 path doesn't go through
// D2 queryloop's WrapToolContext hook, so the live SessionContext (and its
// SessionID/WorkDir) never reaches permission-aware tool runners
// (delegate_status, task_output, task_list_background). Without it, those
// tools return "session context unavailable" / "session_id unavailable".
// Mirror D2's ToolContextWithGate here so D7→D2 tool dispatch behaves the
// same as the legacy D2 path.
//
// DM-20260617-006 (devrix-tool-pipeline-permission): close the D2→D7 拆面
// gap on tool permission. D2 legacy path (query/executor.go:50) already
// gates via permChecker.Request; the D7 turn adapter used to skip this
// check, leaving all tools auto-approved in plan_mode and outside YOLO.
// Now call IPermissionGate.Request with the looked-up risk before
// a.tools.Execute, and propagate the risk into contextengine.ToolCall so
// downstream runners can read the policy classification.
//
// TOOL-SURFACE-1 W9: when the engine has a non-nil surface list, findSurface
// dispatches each tool call to the matching surface.Execute. The legacy
// IToolRunner path is still used when surfaces are absent (phase-1
// back-compat) or for a tool name that no surface claims.
func (a *contextEngineAdapter) ExecuteRound(ctx context.Context, req turn.ToolRoundRequest) (turn.ToolRoundResult, error) {
	if a.tools == nil && len(a.surfaces) == 0 {
		return turn.ToolRoundResult{}, fmt.Errorf("turn adapter: tool runner not available")
	}

	toolCtx := ctx
	if req.SessionID != "" {
		if prov, ok := a.engine.(sessionContextProvider); ok {
			if sc, ok := prov.SessionContext(req.SessionID); ok && sc != nil {
				toolCtx = contextengine.ToolContextWithGate(toolCtx, sc, a.perm)
			}
		}
	}

	results := make([]turn.ToolResult, len(req.ToolCalls))
	for i, tc := range req.ToolCalls {
		// DM-20260617-006: gate via IPermissionGate (suggestion 3) and
		// propagate risk into the D2 ToolCall (suggestion 4 partial). When
		// a.perm is nil we leave the gate open — adapter is shared with
		// tests/mocks that don't wire permission state.
		risk := a.riskForTool(tc.Name)
		if a.perm != nil && !a.perm.Request(toolCtx, req.SessionID, tc.Name, tc.Input, risk) {
			results[i] = turn.ToolResult{
				ToolCallID: tc.ID,
				Error:      "permission denied",
			}
			continue
		}
		// TOOL-SURFACE-1 (W9): prefer surface dispatch when available.
		surf, ok := a.findSurface(tc.Name)
		if ok {
			res, err := surf.Execute(toolCtx, tc.Name, tc.Input, "")
			if err != nil {
				results[i] = turn.ToolResult{ToolCallID: tc.ID, Error: err.Error()}
			} else {
				results[i] = turn.ToolResult{ToolCallID: tc.ID, Output: res.Output, Error: res.Error}
			}
			continue
		}
		// Fall back to the legacy IToolRunner path (W11 removes this).
		if a.tools == nil {
			results[i] = turn.ToolResult{
				ToolCallID: tc.ID,
				Error:      fmt.Sprintf("turn adapter: no surface or runner for tool %q", tc.Name),
			}
			continue
		}
		result, err := a.tools.Execute(toolCtx, contextengine.ToolCall{
			ID:        tc.ID,
			Name:      tc.Name,
			Input:     tc.Input,
			RiskLevel: risk,
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

// riskForTool returns the risk classification for toolName. Surfaces are
// consulted first; the legacy IToolRegistry is the fallback. Returns
// LOW when neither source knows the tool (matches the safe default
// used by every surface in this package).
func (a *contextEngineAdapter) riskForTool(toolName string) types.RiskLevel {
	for _, s := range a.surfaces {
		if r := s.RiskLevel(toolName); r != "" {
			return r
		}
	}
	if a.toolsReg != nil {
		return a.toolsReg.RiskLevel(toolName)
	}
	return types.RiskLevelLow
}

// findSurface returns the first surface in the list that claims toolName
// (via RiskLevel != "") and (ok, true) when found. Linear scan — surface
// lists are at most ~7 entries, so O(N) is fine.
func (a *contextEngineAdapter) findSurface(toolName string) (contracts.ToolSurface, bool) {
	for _, s := range a.surfaces {
		if s == nil {
			continue
		}
		// RiskLevel == "" means "I don't know this tool" — skip and
		// try the next surface. Every surface returns a real level for
		// the tools it owns, so a non-empty value is a positive match.
		if r := s.RiskLevel(toolName); r != "" {
			return s, true
		}
	}
	return nil, false
}

// PersistTurn implements turn.SessionPersister.
//
// DM-20260617-003 (devrix-d7-turn-history-persist): commit this turn's
// transcript (user + assistant + tool_calls + tool_results) into D2 memory
// so the next Prepare call can read it back. Previous implementation was a
// stub that discarded req.Messages, causing multi-turn conversation context
// loss under the loop_first routing mode.
func (a *contextEngineAdapter) PersistTurn(ctx context.Context, req turn.PersistRequest) error {
	if ce, ok := a.engine.(*contextengine.ContextEngine); ok {
		if err := ce.AppendAndTrimMessages(req.SessionID, req.Messages); err != nil {
			return fmt.Errorf("turn adapter: persist: %w", err)
		}
	}
	return nil
}
