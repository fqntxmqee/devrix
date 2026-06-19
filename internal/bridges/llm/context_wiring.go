package llmbridge

import (
	"log/slog"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/llmgateway/configure"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// ContextLLMStack holds LLM gateway wiring for the context engine.
type ContextLLMStack struct {
	Gateway      llmgateway.ILLMGateway
	RawGateway   llmgateway.IGateway
	TokenCounter contracts.ITokenCounter
	// DefaultModel 是 LLM 网关解析后的全局默认模型名（来自
	// llm_gateway.default_model）。供上层（如 ContextEngine）在
	// SessionContext.Model 为空时回填，用于"任务完成卡片显示模型"等
	// 仅展示用途，不参与路由（路由仍由 LLM 网关执行）。
	DefaultModel string
	// TierResolver resolves model tier aliases to concrete model names.
	TierResolver llmgateway.ITierResolver
}

// WireContextLLM loads and wires the LLM stack.
//
// DSAFT: D3-X-A02-F02 FailFastOnObsNil (v1.1 F4, R3 P0 #8).
// BREAKING change vs v1.0: this function no longer swallows wiring errors
// or falls back to a mock gateway. A nil obs bridge or a config/parse error
// is returned to the caller, which is expected to surface it to the user
// at startup. Use llmgateway.NewFromConfig directly if you need a different
// recovery strategy.
//
// `userOverride` is the user-level LLM gateway config (typically from
// UserConfig.LLMGateway). It is deep-merged on top of the project file
// before validation. nil is fine — it means "no user override" and the
// project file / compiled defaults take effect.
func WireContextLLM(configFile string, userOverride *configure.LLMGatewayFileConfig, obsBridge *observability.Bridge) (ContextLLMStack, error) {
	if obsBridge == nil {
		return ContextLLMStack{}, sharederrors.ErrObservabilityRequired
	}
	var projectFile *configure.LLMGatewayFileConfig
	if configFile != "" {
		cf, err := config.LoadConfigFile(configFile)
		if err != nil {
			return ContextLLMStack{}, err
		}
		projectFile = &cf.LLMGateway
	}
	llmCfg, err := configure.BuildLLMGatewayConfigWithUser(projectFile, userOverride)
	if err != nil {
		return ContextLLMStack{}, err
	}
	wired, err := WireFromConfig(llmCfg, obsBridge)
	if err != nil {
		return ContextLLMStack{}, err
	}
	slog.Info("llm gateway wired", "default_provider", llmCfg.DefaultProvider, "default_model", llmCfg.DefaultModel)
	return ContextLLMStack{
		Gateway:      wired.Bridge,
		RawGateway:   wired.Gateway,
		TokenCounter: wired.TokenCounter,
		DefaultModel: llmCfg.DefaultModel,
		TierResolver: wired.Bridge.(llmgateway.ITierResolver),
	}, nil
}
