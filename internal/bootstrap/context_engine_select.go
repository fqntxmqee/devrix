package bootstrap

import (
	"log/slog"
	"strings"

	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine/kernel"
	"github.com/devrix/devrix/internal/layers/multiagent/external"
	"github.com/devrix/devrix/internal/layers/multiagent/provision/freefork"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// SelectContextEngine builds the gateway-facing context engine for the given mode name.
//
// DM-20260617-005: maCfg is forwarded to NewContextEngine so the main engine
// registers delegate_* tools when multi-agent delegate is enabled. Previously
// only the per-agent builder path received maCfg, leaving the leader LLM in
// the main engine without delegate_*/free_fork/query_diagnostics.
//
// DM-20260617-008 W5: forker is forwarded to NewContextEngine so the main
// engine's free_fork surface uses the explicit freefork.Forker. Pass nil when
// multi-agent free_fork is disabled.
func SelectContextEngine(
	name string,
	permMgr *capture.PermissionManager,
	ctxCfg *config.ContextEngineConfig,
	toolCfg *config.ToolConfig,
	maCfg *config.MultiAgentConfig,
	obsBridge *observability.Bridge,
	llmStack llmbridge.ContextLLMStack,
	agentToolReg *external.Registry,
	forker freefork.Forker,
) contracts.IEngine {
	engine := strings.ToLower(strings.TrimSpace(name))
	switch engine {
	case "four_flow", "fourflow", "four-flow":
		slog.Warn("four_flow engine was removed; using context engine with real LLM")
		fallthrough
	case "", "context", "ctx":
		return NewContextEngine(llmStack, permMgr, ctxCfg, toolCfg, maCfg, obsBridge, agentToolReg, forker)
	default:
		slog.Warn("unknown context engine; using context engine", "requested", engine)
		return NewContextEngine(llmStack, permMgr, ctxCfg, toolCfg, maCfg, obsBridge, agentToolReg, forker)
	}
}

// ContextEngineKind returns a short label for logs.
func ContextEngineKind(engine contracts.IEngine) string {
	switch engine.(type) {
	case *kernel.ContextEngine:
		return "context"
	default:
		return "unknown"
	}
}
