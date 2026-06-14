package bootstrap

import (
	"log/slog"
	"strings"

	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/multiagent/tool"
	"github.com/devrix/devrix/internal/shared/config"
)

// SelectContextEngine builds the gateway-facing context engine for the given mode name.
func SelectContextEngine(
	name string,
	permMgr *capture.PermissionManager,
	ctxCfg *config.ContextEngineConfig,
	toolCfg *config.ToolConfig,
	obsBridge *observability.Bridge,
	llmStack llmbridge.ContextLLMStack,
	agentToolReg *tool.Registry,
) capture.IContextEngine {
	engine := strings.ToLower(strings.TrimSpace(name))
	switch engine {
	case "four_flow", "fourflow", "four-flow":
		slog.Warn("four_flow engine was removed; using context engine with real LLM")
		fallthrough
	case "", "context", "ctx":
		return NewContextEngine(llmStack, permMgr, ctxCfg, toolCfg, obsBridge, agentToolReg)
	default:
		slog.Warn("unknown context engine; using context engine", "requested", engine)
		return NewContextEngine(llmStack, permMgr, ctxCfg, toolCfg, obsBridge, agentToolReg)
	}
}

// ContextEngineKind returns a short label for logs.
func ContextEngineKind(engine capture.IContextEngine) string {
	switch engine.(type) {
	case *contextengine.ContextEngine:
		return "context"
	default:
		return "unknown"
	}
}
