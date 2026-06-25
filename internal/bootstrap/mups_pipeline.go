package bootstrap

import (
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/persist"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/config"
)

// MUPSPipelinesDeps wires the production S6 MUPS Pipeline
// (TurnToolExecutor + SubTurnRunner + PreparedTurnAdapter).
//
// D7-S2-A06+A07: turnOrch + subTurn feed the S2 SessionOrchestrator
// via the D7-S2-A06 Prepare wrapper (ctxPrep) and the legacy
// PreparedTurnAdapter for the context engine integration.
type MUPSPipelinesDeps struct {
	CtxAdapter       *contextEngineAdapter
	OrchPath         *sessionorchestrator.OrchestratePath
	LLMInvoker       sessionorchestrator.LLMInvoker
	DefaultModel     string
	LoopFirst        bool
	ObsBridge        *observability.Bridge
	PlanMode         *workmodel.PlanMode
	SubagentCfg      config.SubagentConfig
	MaxContextTokens int
	FocusHint        *workmodel.FocusHintProvider
	ResolveAwait     *workmodel.ResolveAwaiter
	ToolResultStore  *persist.ToolResultStore
}

// WireMUPSPipeline wires TurnToolExecutor + SubTurnRunner for S6 (MUPS Pipeline).
// Returns (toolExec, turnOrch, subTurn).
//
// D7-S2-A06+A07: turnOrch (D7-S2-A06) + subTurn (D7-S2-A06+A07 SubTurnRunner).
// toolExec is returned so the caller can pass it to NewSessionOrchestrator
// (via WithTurnToolExecutor). ctxPrep is built internally and used as
// turnOrch's Context; the caller does not need to retain it.
func WireMUPSPipeline(deps MUPSPipelinesDeps) (*sessionorchestrator.TurnToolExecutor, *sessionorchestrator.DefaultOrchestrator, *sessionorchestrator.SubTurnRunner) {
	toolExec := sessionorchestrator.NewTurnToolExecutor(deps.CtxAdapter, deps.OrchPath, deps.PlanMode, deps.LoopFirst)
	if deps.ObsBridge != nil {
		toolExec.SetTurnToolMetrics(sessionorchestrator.NewTurnToolMetrics(deps.ObsBridge.Meter()))
	}

	ctxPrep := &sessionorchestrator.TurnPrepareWrapper{Inner: deps.CtxAdapter, LoopFirst: deps.LoopFirst}

	turnOrch := sessionorchestrator.NewOrchestrator(sessionorchestrator.OrchestratorDeps{
		LLM:              deps.LLMInvoker,
		Context:          ctxPrep,
		Tools:            toolExec,
		Persist:          deps.CtxAdapter,
		// MaxTurns=0 → unbounded. The main conversation loop terminates
		// on natural LLM finish or one of the deterministic exit reasons
		// (repeated_tool / tool_failure / token_diminishing / ctx cancel).
		// Child agents (subqueries, plan/implement, workers) set their own
		// MaxTurns based on expected workload.
		MaxTurns:         0,
		DefaultModel:     deps.DefaultModel,
		MaxContextTokens: deps.MaxContextTokens,
		ObsBridge:        deps.ObsBridge,
		FocusHint:        deps.FocusHint,
		ResolveAwait:     deps.ResolveAwait,
		// DM-20260620-001 / AC1: oversized tool results (read_file / grep /
		// cat / etc.) are persisted to disk and replaced with a preview
		// marker so they do not blow up the LLM context budget.
		ToolResultStore: deps.ToolResultStore,
	})
	subTurn := sessionorchestrator.NewSubTurnRunner(turnOrch, sessionorchestrator.SubTurnConfig{
		DefaultMode:      deps.SubagentCfg.DefaultMode,
		LegacyMode:       deps.SubagentCfg.LegacyMode,
		MaxDepth:         deps.SubagentCfg.MaxDepth,
		MaxContextTokens: deps.MaxContextTokens,
	})
	return toolExec, turnOrch, subTurn
}
