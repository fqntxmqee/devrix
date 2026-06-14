package route

import (
	"sort"
	"strings"

	sharedconfig "github.com/devrix/devrix/internal/shared/config"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// Router resolves model names to provider + concrete model.
type Router struct {
	cfg *sharedconfig.LLMGatewayConfig
}

// NewRouter creates a provider router.
func NewRouter(cfg *sharedconfig.LLMGatewayConfig) *Router {
	if cfg == nil {
		cfg = sharedconfig.DefaultLLMGatewayConfig()
	}
	return &Router{cfg: cfg}
}

// ResolveTier resolves a tier alias to a concrete model name.
// Returns the input unchanged if not a known tier.
func (r *Router) ResolveTier(tier string) string {
	if tier == "" || r.cfg.ModelTiers == nil {
		return tier
	}
	if concrete, ok := r.cfg.ModelTiers[tier]; ok {
		return concrete
	}
	return tier
}

// Resolve returns provider and model using routing rules.
func (r *Router) Resolve(model string) (provider string, resolvedModel string, err error) {
	model = strings.TrimSpace(model)

	if model == "" {
		provider = r.cfg.DefaultProvider
		resolvedModel = r.defaultModelFor(provider)
		if resolvedModel == "" {
			return "", "", sharederrors.NewUnsupportedModelError(model)
		}
		// Resolve default model tier alias before returning
		resolvedModel = r.ResolveTier(resolvedModel)
		return provider, resolvedModel, nil
	}

	// Resolve tier alias to concrete model before provider routing
	model = r.ResolveTier(model)

	if provider, ok := r.matchRouting(model); ok {
		if _, exists := r.cfg.Providers[provider]; !exists {
			return "", "", sharederrors.NewUnsupportedProviderError(provider)
		}
		return provider, model, nil
	}

	return "", "", sharederrors.NewUnsupportedModelError(model)
}

func (r *Router) defaultModelFor(provider string) string {
	if p, ok := r.cfg.Providers[provider]; ok && p.DefaultModel != "" {
		return p.DefaultModel
	}
	return r.cfg.DefaultModel
}

func (r *Router) matchRouting(model string) (string, bool) {
	if len(r.cfg.ModelRouting) == 0 {
		return "", false
	}
	patterns := make([]string, 0, len(r.cfg.ModelRouting))
	for pattern := range r.cfg.ModelRouting {
		patterns = append(patterns, pattern)
	}
	sort.Slice(patterns, func(i, j int) bool {
		return len(patterns[i]) > len(patterns[j])
	})
	for _, pattern := range patterns {
		if matchPattern(pattern, model) {
			return r.cfg.ModelRouting[pattern], true
		}
	}
	return "", false
}

func matchPattern(pattern, model string) bool {
	if pattern == "" {
		return false
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(model, prefix)
	}
	return model == pattern
}
