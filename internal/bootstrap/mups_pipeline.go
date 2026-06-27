package bootstrap

import (
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/persist"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/config"
)

// MUPSPipelinesDeps wires TurnToolExecutor + SubTurnRunner for sub-agent
// and WorkItemExecutor tool rounds (D7-S2-A06+A07).
type MUPSPipelinesDeps struct {
	CtxAdapter       *contextEngineAdapter
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

// WireMUPSPipeline wires TurnToolExecutor + SubTurnRunner.
// Returns (toolExec, turnOrch, subTurn) for ItemPipeline and sub-agent paths.
func WireMUPSPipeline(deps MUPSPipelinesDeps) (*sessionorchestrator.TurnToolExecutor, *sessionorchestrator.DefaultOrchestrator, *sessionorchestrator.SubTurnRunner) {
	toolExec := sessionorchestrator.NewTurnToolExecutor(deps.CtxAdapter, deps.PlanMode, deps.LoopFirst)
	if deps.ObsBridge != nil {
		toolExec.SetTurnToolMetrics(sessionorchestrator.NewTurnToolMetrics(deps.ObsBridge.Meter()))
	}

	ctxPrep := &sessionorchestrator.TurnPrepareWrapper{Inner: deps.CtxAdapter, LoopFirst: deps.LoopFirst}

	turnOrch := sessionorchestrator.NewOrchestrator(sessionorchestrator.OrchestratorDeps{
		LLM:              deps.LLMInvoker,
		Context:          ctxPrep,
		Tools:            toolExec,
		Persist:          deps.CtxAdapter,
		MaxTurns:         0,
		DefaultModel:     deps.DefaultModel,
		MaxContextTokens: deps.MaxContextTokens,
		ObsBridge:        deps.ObsBridge,
		FocusHint:        deps.FocusHint,
		ResolveAwait:     deps.ResolveAwait,
		ToolResultStore:  deps.ToolResultStore,
	})
	subTurn := sessionorchestrator.NewSubTurnRunner(turnOrch, sessionorchestrator.SubTurnConfig{
		DefaultMode:      deps.SubagentCfg.DefaultMode,
		LegacyMode:       deps.SubagentCfg.LegacyMode,
		MaxDepth:         deps.SubagentCfg.MaxDepth,
		MaxContextTokens: deps.MaxContextTokens,
	})
	return toolExec, turnOrch, subTurn
}
