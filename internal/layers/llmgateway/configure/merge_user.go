package configure

// MergeUserOverrideFile is the 3-way user-override entry point.
//
// Merge order (highest priority wins):
//  1. user — from internal/shared/config.UserConfig.LLMGateway
//  2. file — from project config (devrix.yaml llm_gateway section)
//  3. defaults — implicit (applied later via BuildLLMGatewayConfig)
//
// The function returns a fully-merged LLMGatewayFileConfig ready to be
// fed into BuildLLMGatewayConfig for default reconciliation. This is
// the canonical 3-way user-override path; callers should NOT stitch
// configs themselves.
//
// DSAFT: D3-S6-A02 (user-level LLM gateway override, v2.x).
//
// DM-20260629-003 PR-3 (#1 god-fn-split pt2): extracted from
// shared_config.go::BuildLLMGatewayConfigWithUser so the merge
// semantic lives next to its sibling MergeLLMGatewayFileConfig in
// merge.go, and the orchestrator (BuildLLMGatewayConfigWithUser)
// stays a thin shim.
func MergeUserOverrideFile(file, user *LLMGatewayFileConfig) *LLMGatewayFileConfig {
	// user wins over file at the file-config layer; defaults are
	// reconciled later by BuildLLMGatewayConfig.
	return MergeLLMGatewayFileConfig(file, user)
}

// ApplyUserOverridesToResolved applies a user-override LLMGatewayFileConfig
// directly onto an already-resolved LLMGatewayConfig (defaults+file merged).
//
// This is the lower-level helper for callers that already hold a resolved
// LLMGatewayConfig (e.g. compiled defaults from cache) and only need to
// re-apply user overrides without rebuilding the whole config tree.
//
// Semantics:
//   - Scalar/string/time fields: override wins when non-zero.
//   - Provider entries: per-key scalar merge (user can override
//     individual fields like default_model without redeclaring
//     type/base_url/timeout/retry).
//
// DSAFT: D3-S6-A02.
//
// DM-20260629-003 PR-3: extracted from merge.go for separation of concerns.
// merge.go keeps the file-level 2-way merge (defaults + project config);
// merge_user.go owns the user-override layer (highest priority).
func ApplyUserOverridesToResolved(base *LLMGatewayConfig, user *LLMGatewayFileConfig) *LLMGatewayConfig {
	if base == nil {
		base = DefaultLLMGatewayConfig()
	}
	if user == nil {
		return base
	}

	if user.DefaultProvider != "" {
		base.DefaultProvider = user.DefaultProvider
	}
	if user.DefaultModel != "" {
		base.DefaultModel = user.DefaultModel
	}
	if user.DefaultTier != "" {
		base.DefaultTier = user.DefaultTier
	}
	if len(user.ModelTiers) > 0 {
		base.ModelTiers = mergeStringMap(base.ModelTiers, user.ModelTiers)
	}
	if len(user.ModelRouting) > 0 {
		base.ModelRouting = mergeStringMap(base.ModelRouting, user.ModelRouting)
	}

	// Circuit breaker: only override non-zero fields (so partial overrides work).
	cb := base.CircuitBreaker
	if user.CircuitBreaker.FailureThreshold != 0 {
		cb.FailureThreshold = user.CircuitBreaker.FailureThreshold
	}
	if user.CircuitBreaker.SuccessThreshold != 0 {
		cb.SuccessThreshold = user.CircuitBreaker.SuccessThreshold
	}
	if user.CircuitBreaker.OpenDuration != 0 {
		cb.OpenDuration = user.CircuitBreaker.OpenDuration
	}
	if user.CircuitBreaker.HalfOpenMaxProbes != 0 {
		cb.HalfOpenMaxProbes = user.CircuitBreaker.HalfOpenMaxProbes
	}
	if user.CircuitBreaker.Scope != "" {
		cb.Scope = user.CircuitBreaker.Scope
	}
	base.CircuitBreaker = cb

	// Providers: per-key merge.
	for name, ovr := range user.Providers {
		merged := base.Providers[name]
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
		base.Providers[name] = merged
	}
	return base
}