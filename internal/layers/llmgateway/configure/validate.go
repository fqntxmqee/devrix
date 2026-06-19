package configure

import (
	"errors"
	"fmt"
	"os"
)

// ErrLLMConfigMissing signals that LLM gateway config cannot start because
// required fields (default_model, default_provider) are absent after merging
// project defaults with the user-level override.
//
// This is the contract enforcement point of "no hardcoded model defaults":
// if the user never declares a model anywhere, devrix refuses to start with a
// clear pointer to where to set it.
var ErrLLMConfigMissing = errors.New("llm_gateway config missing required fields")

// ValidateLLMGatewayConfig checks that the resolved config has the minimum
// required fields to start the LLM gateway. Returns ErrLLMConfigMissing wrapped
// with a human-readable hint that names the user's config file path and the
// relevant env vars.
func ValidateLLMGatewayConfig(cfg *LLMGatewayConfig) error {
	if cfg == nil {
		return wrapMissing("llm_gateway config is nil after load", nil)
	}
	missing := []string{}
	if cfg.DefaultModel == "" {
		missing = append(missing, "default_model")
	}
	if cfg.DefaultProvider == "" {
		missing = append(missing, "default_provider")
	}
	if cfg.DefaultProvider != "" {
		p, ok := cfg.Providers[cfg.DefaultProvider]
		if !ok {
			return fmt.Errorf("llm_gateway.default_provider %q has no matching providers entry (available: %v)",
				cfg.DefaultProvider, providerKeys(cfg.Providers))
		}
		if p.DefaultModel == "" {
			missing = append(missing, fmt.Sprintf("providers.%s.default_model", cfg.DefaultProvider))
		}
	}
	if len(missing) > 0 {
		return wrapMissing(fmt.Sprintf("missing fields: %v", missing), missing)
	}
	return nil
}

func wrapMissing(msg string, fields []string) error {
	home, _ := os.UserHomeDir()
	userCfg := "~/.devrix/config.yaml"
	if home != "" {
		userCfg = home + "/.devrix/config.yaml"
	}
	hint := fmt.Sprintf(
		"set them in %s under llm_gateway: (e.g. `default_model: <your-model>`), "+
			"or via env vars DEVRIX_LLM_DEFAULT_MODEL / DEVRIX_LLM_DEFAULT_PROVIDER",
		userCfg,
	)
	if fields != nil {
		hint = fmt.Sprintf("missing %d field(s): %v — %s", len(fields), fields, hint)
	}
	return fmt.Errorf("%w: %s. %s", ErrLLMConfigMissing, msg, hint)
}

func providerKeys(m map[string]LLMProviderRuntimeConfig) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}