package configure

// MergeLLMGatewayFileConfig deep-merges override into base.
//
// Semantics:
//   - Scalar/string/time fields: override wins when non-zero.
//   - Map fields (ModelTiers, ModelRouting): key-level merge (override keys
//     replace base keys; base keys not in override are preserved).
//   - Provider entries (Providers[name]): same scalar rules applied per
//     provider; user can override individual fields (e.g. default_model) without
//     having to redeclare type/base_url/timeout/retry.
//
// Returns base unchanged when override is nil; returns a clone of override when
// base is nil.
//
// DSAFT: D3-S6-A02 (user-level LLM gateway override, v2.x).
func MergeLLMGatewayFileConfig(base, override *LLMGatewayFileConfig) *LLMGatewayFileConfig {
	if override == nil {
		return base
	}
	if base == nil {
		out := *override
		// Deep-clone maps so the caller's override isn't aliased.
		out.ModelTiers = cloneStringMap(override.ModelTiers)
		out.ModelRouting = cloneStringMap(override.ModelRouting)
		out.Providers = cloneProviders(override.Providers)
		return &out
	}

	out := *base
	out.ModelTiers = cloneStringMap(base.ModelTiers)
	out.ModelRouting = cloneStringMap(base.ModelRouting)
	out.Providers = cloneProviders(base.Providers)

	if override.DefaultProvider != "" {
		out.DefaultProvider = override.DefaultProvider
	}
	if override.DefaultModel != "" {
		out.DefaultModel = override.DefaultModel
	}
	if override.DefaultTier != "" {
		out.DefaultTier = override.DefaultTier
	}
	out.ModelTiers = mergeStringMap(out.ModelTiers, override.ModelTiers)
	out.ModelRouting = mergeStringMap(out.ModelRouting, override.ModelRouting)

	// Circuit breaker: only override non-zero fields (so partial overrides work).
	cb := out.CircuitBreaker
	if override.CircuitBreaker.FailureThreshold != 0 {
		cb.FailureThreshold = override.CircuitBreaker.FailureThreshold
	}
	if override.CircuitBreaker.SuccessThreshold != 0 {
		cb.SuccessThreshold = override.CircuitBreaker.SuccessThreshold
	}
	if override.CircuitBreaker.OpenDuration != 0 {
		cb.OpenDuration = override.CircuitBreaker.OpenDuration
	}
	if override.CircuitBreaker.HalfOpenMaxProbes != 0 {
		cb.HalfOpenMaxProbes = override.CircuitBreaker.HalfOpenMaxProbes
	}
	if override.CircuitBreaker.Scope != "" {
		cb.Scope = override.CircuitBreaker.Scope
	}
	out.CircuitBreaker = cb

	// Providers: per-key merge.
	for name, ovr := range override.Providers {
		merged := out.Providers[name]
		if merged.Type == "" {
			merged.Type = ovr.Type
		}
		if ovr.Type != "" {
			merged.Type = ovr.Type
		}
		if ovr.BaseURL != "" {
			merged.BaseURL = ovr.BaseURL
		}
		if ovr.APIKeyEnv != "" {
			merged.APIKeyEnv = ovr.APIKeyEnv
		}
		if ovr.DefaultModel != "" {
			merged.DefaultModel = ovr.DefaultModel
		}
		if ovr.FallbackModel != "" {
			merged.FallbackModel = ovr.FallbackModel
		}
		if ovr.Timeout != 0 {
			merged.Timeout = ovr.Timeout
		}
		if ovr.MaxTokens != 0 {
			merged.MaxTokens = ovr.MaxTokens
		}
		if ovr.Temperature != 0 {
			merged.Temperature = ovr.Temperature
		}
		if ovr.Retry.MaxAttempts != 0 {
			merged.Retry.MaxAttempts = ovr.Retry.MaxAttempts
		}
		if ovr.Retry.InitialDelay != 0 {
			merged.Retry.InitialDelay = ovr.Retry.InitialDelay
		}
		if ovr.Retry.MaxDelay != 0 {
			merged.Retry.MaxDelay = ovr.Retry.MaxDelay
		}
		if ovr.Retry.Backoff != 0 {
			merged.Retry.Backoff = ovr.Retry.Backoff
		}
		if len(ovr.Headers) > 0 {
			merged.Headers = mergeStringMap(merged.Headers, ovr.Headers)
		}
		out.Providers[name] = merged
	}
	return &out
}

func cloneStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func cloneProviders(m map[string]LLMProviderConfig) map[string]LLMProviderConfig {
	if m == nil {
		return nil
	}
	out := make(map[string]LLMProviderConfig, len(m))
	for k, v := range m {
		clone := v
		clone.Headers = cloneStringMap(v.Headers)
		out[k] = clone
	}
	return out
}

func mergeStringMap(base, override map[string]string) map[string]string {
	if len(override) == 0 {
		return base
	}
	if base == nil {
		base = make(map[string]string, len(override))
	}
	for k, v := range override {
		base[k] = v
	}
	return base
}