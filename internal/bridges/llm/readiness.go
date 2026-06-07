package llmbridge

import (
	"log/slog"

	llmconfig "github.com/devrix/devrix/internal/layers/llmgateway/config"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/shared/config"
)

// LogLLMReadiness logs whether the default provider API key is present.
func LogLLMReadiness(configFile string) {
	llmCfg, err := config.LoadLLMGatewayConfig(configFile)
	if err != nil {
		slog.Warn("llm gateway config unavailable", "error", err)
		return
	}
	providerName := llmCfg.DefaultProvider
	p, ok := llmCfg.Providers[providerName]
	if !ok {
		slog.Warn("llm default provider not configured", "provider", providerName)
		return
	}
	if key, hasKey := llmconfig.APIKey(p); hasKey {
		slog.Info("llm provider ready",
			"provider", providerName,
			"model", p.DefaultModel,
			"key_env", p.APIKeyEnv,
			"key_prefix", maskKeyPrefix(key),
		)
		return
	}
	slog.Warn("llm api key missing — calls will fail until env is set",
		"provider", providerName,
		"env", p.APIKeyEnv,
	)
}

// IsMockGateway reports whether the stack fell back to the in-process mock.
func IsMockGateway(stack ContextLLMStack) bool {
	_, ok := stack.Gateway.(*mockctx.LLMGateway)
	return ok
}

func maskKeyPrefix(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "..." + key[len(key)-4:]
}
