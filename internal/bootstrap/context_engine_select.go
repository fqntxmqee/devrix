package bootstrap

import (
	"log/slog"
	"strings"

	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/layers/communication/milestone"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/multiagent/tool"
	"github.com/devrix/devrix/internal/shared/config"
)

// SelectContextEngine builds the gateway-facing context engine for the given mode name.
func SelectContextEngine(
	name string,
	permMgr *gateway.PermissionManager,
	ctxCfg *config.ContextEngineConfig,
	toolCfg *config.ToolConfig,
	obsBridge *observability.Bridge,
	llmStack llmbridge.ContextLLMStack,
	milestoneSvc milestone.IMilestoneService,
	agentToolReg *tool.Registry,
) gateway.IContextEngine {
	engine := strings.ToLower(strings.TrimSpace(name))
	switch engine {
	case "stub", "echo":
		return gateway.NewStubContextEngine()
	case "four_flow", "fourflow", "four-flow":
		slog.Warn("four_flow engine was removed; using context engine with real LLM")
		fallthrough
	case "", "context", "ctx":
		return NewContextEngine(llmStack, permMgr, ctxCfg, toolCfg, obsBridge, milestoneSvc, agentToolReg)
	default:
		slog.Warn("unknown context engine; using context engine", "requested", engine)
		return NewContextEngine(llmStack, permMgr, ctxCfg, toolCfg, obsBridge, milestoneSvc, agentToolReg)
	}
}

// ContextEngineKind returns a short label for logs.
func ContextEngineKind(engine gateway.IContextEngine) string {
	switch engine.(type) {
	case *gateway.StubContextEngine:
		return "stub"
	case *contextengine.ContextEngine:
		return "context"
	default:
		return "unknown"
	}
}
