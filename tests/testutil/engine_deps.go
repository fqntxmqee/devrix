package testutil

import (
	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/layers/orchestration/turn"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// ContextEngineDepsFromStack builds EngineDeps with D7 turn adapters (production-like).
func ContextEngineDepsFromStack(stack llmbridge.ContextLLMStack, ctxCfg *config.ContextEngineConfig) contextengine.EngineDeps {
	if ctxCfg == nil {
		ctxCfg = config.DefaultContextEngineConfig()
	}
	summarizer := turn.NewCompressionSummarizer(turn.CompressionSummarizerDeps{
		Gateway:      stack.RawGateway,
		TierResolver: stack.TierResolver,
		DefaultTier:  stack.DefaultModel,
		Timeout:      ctxCfg.Compression.Autocompact.Timeout,
	})
	return contextengine.EngineDeps{
		Summarizer:   summarizer,
		TokenCounter: stack.TokenCounter,
		TierResolver: stack.TierResolver,
		DefaultModel: stack.DefaultModel,
		Config:       ctxCfg,
	}
}

// MergeEngineDeps overlays non-zero fields from patch onto base.
func MergeEngineDeps(base contextengine.EngineDeps, patch contextengine.EngineDeps) contextengine.EngineDeps {
	if patch.Summarizer != nil {
		base.Summarizer = patch.Summarizer
	}
	if patch.PreparedTurnRunner != nil {
		base.PreparedTurnRunner = patch.PreparedTurnRunner
	}
	if patch.TokenCounter != nil {
		base.TokenCounter = patch.TokenCounter
	}
	if patch.Tools != nil {
		base.Tools = patch.Tools
	}
	if patch.ToolsReg != nil {
		base.ToolsReg = patch.ToolsReg
	}
	if patch.Permission != nil {
		base.Permission = patch.Permission
	}
	if patch.LongTermRecaller != nil {
		base.LongTermRecaller = patch.LongTermRecaller
	}
	if patch.LongTermStore != nil {
		base.LongTermStore = patch.LongTermStore
	}
	if patch.Config != nil {
		base.Config = patch.Config
	}
	if patch.ObsBridge != nil {
		base.ObsBridge = patch.ObsBridge
	}
	if patch.DefaultModel != "" {
		base.DefaultModel = patch.DefaultModel
	}
	if patch.TierResolver != nil {
		base.TierResolver = patch.TierResolver
	}
	if patch.AgentRoleToolFilter != nil {
		base.AgentRoleToolFilter = patch.AgentRoleToolFilter
	}
	if patch.SessionCommandQueue != nil {
		base.SessionCommandQueue = patch.SessionCommandQueue
	}
	return base
}

// EnsureLLMDeps panics if Summarizer or PreparedTurnRunner are missing.
func EnsureLLMDeps(deps contextengine.EngineDeps) contextengine.EngineDeps {
	if deps.Summarizer == nil {
		panic("test EngineDeps missing Summarizer")
	}
	if deps.PreparedTurnRunner == nil {
		deps.PreparedTurnRunner = &mockctx.StaticPreparedTurnRunner{}
	}
	return deps
}

// EngineDepsWithPreparedTurn returns minimal EngineDeps with a static prepared-turn runner.
func EngineDepsWithPreparedTurn(runner contracts.PreparedTurnRunner) contextengine.EngineDeps {
	if runner == nil {
		runner = &mockctx.StaticPreparedTurnRunner{}
	}
	return contextengine.EngineDeps{
		PreparedTurnRunner: runner,
		Summarizer:         &mockctx.StaticSummarizer{},
	}
}

// StaticLLMDeps returns deps with a static prepared-turn runner (legacy name kept for tests).
func StaticLLMDeps(caller contracts.LLMCaller) contextengine.EngineDeps {
	runner := &mockctx.StaticPreparedTurnRunner{}
	if c, ok := caller.(*mockctx.StaticLLMCaller); ok {
		runner = mockctx.PreparedTurnRunnerFromCaller(c).(*mockctx.StaticPreparedTurnRunner)
	}
	return EngineDepsWithPreparedTurn(runner)
}
