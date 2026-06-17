package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/llmgateway"
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
//
// TOOL-SURFACE-1-A02 (DM-20260618-003 devrix-surface-lazy-loading):
// the adapter filters out DeferLoading=true specs from the LLM prompt
// (tool_search is exempt) and consults an optional DeferDecision chain
// (e.g. PlanModeOpenWorldPolicy) for runtime defer signals.
type contextEngineAdapter struct {
	gw           *capture.CommunicationGateway
	engine       contracts.IEngine
	tools        contextengine.IToolRunner
	toolsReg     contextengine.IToolRegistry
	perm         contracts.IPermissionGate
	counter      contracts.ITokenCounter
	surfaces     []contracts.ToolSurface
	deferDecider contracts.DeferDecision
}

func newContextEngineAdapter(gw *capture.CommunicationGateway, engine contracts.IEngine, counter contracts.ITokenCounter) *contextEngineAdapter {
	a := &contextEngineAdapter{
		gw: gw, engine: engine, counter: counter,
		deferDecider: contracts.NeverDefer{},
	}
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
//
// TOOL-SURFACE-1 (W11 phase 2b): tool schema aggregation prefers the
// engine's surface list (the new SoT) and falls back to the legacy
// toolReg only for tools that no surface claims. This lets the LLM
// see free_fork / query_diagnostics / verify_plan_execution / lsp
// without those tools being registered in toolReg.
func (a *contextEngineAdapter) Prepare(ctx context.Context, req turn.PrepareRequest) (turn.PreparedContext, error) {
	var session *types.Session
	if a.gw != nil {
		s, err := a.gw.GetSession(req.SessionID)
		if err == nil {
			session = s
		}
	}
	if session == nil {
		session = types.NewSession(req.SessionID, "d7", "")
	}

	seen := map[string]bool{}
	var toolSchemas []turn.ToolSchema
	for _, s := range a.surfaces {
		for _, sp := range s.Tools(ctx, session.WorkDir, "") {
			if seen[sp.Name] {
				continue
			}
			seen[sp.Name] = true
			// TOOL-SURFACE-1-A02 (DM-20260618-003): defer-load filter.
			// Static DeferLoading=true specs and runtime defer decisions
			// (e.g. PlanModeOpenWorldPolicy) are dropped from the LLM
			// prompt. tool_search itself MUST stay in-pack.
			if sp.DeferLoading && sp.Name != "tool_search" {
				continue
			}
			if a.deferDecider != nil && a.deferDecider.ShouldDefer(ctx, sp) {
				continue
			}
			params := parseToolParams(sp.Parameters)
			toolSchemas = append(toolSchemas, turn.ToolSchema{
				Name:        sp.Name,
				Description: sp.Description,
				Parameters:  params,
			})
		}
	}
	if a.toolsReg != nil {
		schemas, err := a.toolsReg.ListTools(ctx, session.WorkDir)
		if err == nil {
			for _, s := range schemas {
				if seen[s.Name] {
					continue
				}
				seen[s.Name] = true
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
//
// TOOL-SURFACE-1-A01-F06 (DM-20260618-001 devrix-tool-spec-enrichment):
// tool calls marked ConcurrencySafe=true on their ToolSpec run in
// parallel via errgroup; the rest run sequentially. The permission
// gate is consulted INSIDE executeOne so the gate mutex is held only
// for the gate call (the gate is a shared, sequential resource). The
// results slice is pre-allocated and indexed so the order of
// req.ToolCalls is preserved in the output.
//
// TOOL-SURFACE-1-A01-F07 (DM-20260618-002 devrix-surface-permission-extension):
// ExecuteRound runs a 2-phase dispatch:
//
//	Phase 1: for each tool call, consult surface.CheckPermission. If
//	  the surface returns Deny or Ask, the result is set immediately
//	  (PermissionDeniedError / PermissionAskRequiredError) and the
//	  tool is NOT executed. Ask delegates to IPermissionGate.CheckPermission
//	  for the final policy decision (plan-mode OpenWorld denial goes
//	  through this path).
//	Phase 2: parallel / sequential dispatch of the remaining
//	  Allow tools, identical to DM-001 F06.
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

	concSafe := a.concurrencyMap()
	results := make([]turn.ToolResult, len(req.ToolCalls))

	// Phase 1: CheckPermission pre-dispatch (DM-002 F07).
	for i, tc := range req.ToolCalls {
		if r, denied := a.checkPermission(toolCtx, req.SessionID, tc); denied {
			results[i] = r
		}
	}

	// Phase 2: execute the surviving calls in parallel / sequential.
	var parallelIdx []int
	for i, tc := range req.ToolCalls {
		if results[i].Error != "" {
			continue // already denied in Phase 1
		}
		if concSafe[tc.Name] {
			parallelIdx = append(parallelIdx, i)
			continue
		}
		results[i] = a.executeOne(toolCtx, req.SessionID, tc)
	}

	if len(parallelIdx) > 0 {
		var g errgroup.Group
		for _, idx := range parallelIdx {
			idx, tc := idx, req.ToolCalls[idx]
			g.Go(func() error {
				results[idx] = a.executeOne(toolCtx, req.SessionID, tc)
				return nil
			})
		}
		_ = g.Wait()
	}

	return turn.ToolRoundResult{Results: results}, nil
}

// checkPermission runs surface.CheckPermission → IPermissionGate.CheckPermission
// and returns the appropriate ToolResult + denied=true when the tool
// should NOT be executed. Returns (_, false) when the call should
// proceed to Phase 2.
//
// TOOL-SURFACE-1-A01-F07 (DM-20260618-002).
func (a *contextEngineAdapter) checkPermission(toolCtx context.Context, sessionID string, tc llmgateway.ToolCall) (turn.ToolResult, bool) {
	surf, ok := a.findSurface(tc.Name)
	if !ok {
		return turn.ToolResult{}, false
	}
	spec, _ := a.findSpec(toolCtx, tc.Name)
	var specVal contracts.ToolSpec
	if spec != nil {
		specVal = *spec
	}
	decision := surf.CheckPermission(toolCtx, specVal, json.RawMessage(tc.Input))
	if decision == contracts.DecisionAllow {
		return turn.ToolResult{}, false
	}
	if decision == contracts.DecisionAsk && a.perm != nil {
		decision = a.perm.CheckPermission(toolCtx, specVal)
	}
	if decision == contracts.DecisionAllow {
		return turn.ToolResult{}, false
	}
	reason := ""
	switch decision {
	case contracts.DecisionDeny:
		reason = "policy denied"
	case contracts.DecisionAsk:
		reason = "ask required"
	}
	if decision == contracts.DecisionDeny {
		return turn.ToolResult{
			ToolCallID: tc.ID,
			Error: (&contracts.PermissionDeniedError{
				Spec:   specVal,
				Input:  json.RawMessage(tc.Input),
				Reason: reason,
			}).Error(),
		}, true
	}
	return turn.ToolResult{
		ToolCallID: tc.ID,
		Error: (&contracts.PermissionAskRequiredError{
			Spec:   specVal,
			Input:  json.RawMessage(tc.Input),
			Reason: reason,
		}).Error(),
	}, true
}

// findSpec looks up the ToolSpec for a tool name across all surfaces.
// Returns (nil, false) when no surface claims the tool.
func (a *contextEngineAdapter) findSpec(ctx context.Context, name string) (*contracts.ToolSpec, bool) {
	for _, s := range a.surfaces {
		if s == nil {
			continue
		}
		for _, sp := range s.Tools(ctx, "", "") {
			if sp.Name == name {
				return &sp, true
			}
		}
	}
	return nil, false
}

// executeOne runs the full gate → surface → fallback chain for a single
// tool call. Shared by both the sequential and parallel dispatch paths.
func (a *contextEngineAdapter) executeOne(toolCtx context.Context, sessionID string, tc llmgateway.ToolCall) turn.ToolResult {
	// DM-20260617-006: gate via IPermissionGate (suggestion 3) and
	// propagate risk into the D2 ToolCall (suggestion 4 partial). When
	// a.perm is nil we leave the gate open — adapter is shared with
	// tests/mocks that don't wire permission state.
	risk := a.riskForTool(tc.Name)
	if a.perm != nil && !a.perm.Request(toolCtx, sessionID, tc.Name, tc.Input, risk) {
		return turn.ToolResult{ToolCallID: tc.ID, Error: "permission denied"}
	}
	// TOOL-SURFACE-1 (W9): prefer surface dispatch when available.
	if surf, ok := a.findSurface(tc.Name); ok {
		res, err := surf.Execute(toolCtx, tc.Name, tc.Input, "")
		if err != nil {
			return turn.ToolResult{ToolCallID: tc.ID, Error: err.Error()}
		}
		return turn.ToolResult{ToolCallID: tc.ID, Output: res.Output, Error: res.Error}
	}
	// Fall back to the legacy IToolRunner path (W11 removes this).
	if a.tools == nil {
		return turn.ToolResult{
			ToolCallID: tc.ID,
			Error:      fmt.Sprintf("turn adapter: no surface or runner for tool %q", tc.Name),
		}
	}
	result, err := a.tools.Execute(toolCtx, contextengine.ToolCall{
		ID:        tc.ID,
		Name:      tc.Name,
		Input:     tc.Input,
		RiskLevel: risk,
	})
	if err != nil {
		return turn.ToolResult{ToolCallID: tc.ID, Error: err.Error()}
	}
	return turn.ToolResult{ToolCallID: tc.ID, Output: result.Output, Error: result.Error}
}

// concurrencyMap builds a toolName → ConcurrencySafe lookup from the
// surface list. Tools not declared by any surface default to false
// (sequential). Used by ExecuteRound to decide parallel vs sequential
// dispatch.
//
// TOOL-SURFACE-1-A01-F06 (DM-20260618-001 devrix-tool-spec-enrichment).
func (a *contextEngineAdapter) concurrencyMap() map[string]bool {
	m := make(map[string]bool, 32)
	for _, s := range a.surfaces {
		if s == nil {
			continue
		}
		for _, sp := range s.Tools(context.Background(), "", "") {
			m[sp.Name] = sp.ConcurrencySafe
		}
	}
	return m
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
