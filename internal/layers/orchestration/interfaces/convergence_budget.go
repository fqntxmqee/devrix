package interfaces

// ConvergenceBudget helpers — the struct itself is declared in task_spec.go
// (PR-A landed it there as part of TaskSpec.ConvergenceBudget). The
// helpers here are pure functions that operate on the existing
// ConvergenceBudget type, keeping the Pure types invariant (IV-1) and
// matching the same file-co-location pattern as fallback_policy.go.

// NewConvergenceBudget constructs a ConvergenceBudget with the supplied
// policy. The token / step / time fields default to zero, which means
// "use the upstream-provided budget". Callers that want to override
// (e.g. a tighter ceiling) should use the With* builders below.
func NewConvergenceBudget(policy FallbackPolicy) ConvergenceBudget {
	return ConvergenceBudget{Policy: policy}
}

// WithMaxDepth returns a shallow copy of b with MaxDepth replaced.
func (b ConvergenceBudget) WithMaxDepth(n int) ConvergenceBudget {
	c := b
	c.MaxDepth = n
	return c
}

// WithMaxSteps returns a shallow copy of b with MaxSteps replaced.
func (b ConvergenceBudget) WithMaxSteps(n int) ConvergenceBudget {
	c := b
	c.MaxSteps = n
	return c
}

// WithMaxTokens returns a shallow copy of b with MaxTokens replaced.
func (b ConvergenceBudget) WithMaxTokens(n int) ConvergenceBudget {
	c := b
	c.MaxTokens = n
	return c
}

// WithPolicy returns a shallow copy of b with Policy replaced. Used when
// the bootstrap wants to overwrite the spec's policy with the Feature
// Flag override.
func (b ConvergenceBudget) WithPolicy(p FallbackPolicy) ConvergenceBudget {
	c := b
	c.Policy = p
	return c
}

// Validate enforces the invariants on a ConvergenceBudget. The check is
// cheap and idempotent; callers can call it any time they receive a budget
// from an untrusted source (e.g. an external queue).
//
//   - MaxDepth >= 0  (0 means "uncapped")
//   - MaxSteps >= 0  (0 means "uncapped")
//   - MaxTokens >= 0 (0 means "uncapped")
//   - Policy is one of the 3 recognized values
func (b ConvergenceBudget) Validate() error {
	if b.MaxDepth < 0 {
		return NewResourceInvalidError()
	}
	if b.MaxSteps < 0 {
		return NewResourceInvalidError()
	}
	if b.MaxTokens < 0 {
		return NewResourceInvalidError()
	}
	if !b.Policy.Valid() {
		return NewResourceInvalidError()
	}
	return nil
}

// RemainingBelowReserve reports whether the supplied tokensUsed has
// crossed below the supplied reserve. The spec is treated as
// "approaching exhaustion" once the remaining tokens (tokensBudget -
// tokensUsed) drop to or below reserve.
//
// This helper is intentionally separate from ConvergenceBudget because
// the spec's budget fields are advisory limits (the actual enforcement
// lives in ContextBudget Phase B). The guard consults this method with
// the live values reported by Resource.TokensUsed / TokensBudget.
func RemainingBelowReserve(tokensUsed, tokensBudget, reserve int) bool {
	if tokensBudget <= 0 {
		return false // no budget declared → never triggers
	}
	remaining := tokensBudget - tokensUsed
	if remaining < 0 {
		remaining = 0
	}
	return remaining <= reserve
}

// ToFields decomposes a ConvergenceBudget into the 3-tuple that the
// telemetry pipeline emits as span attributes. Kept as a tiny helper so
// callers don't have to reach into the struct fields by name.
func (b ConvergenceBudget) ToFields() (maxDepth, maxSteps, maxTokens int) {
	return b.MaxDepth, b.MaxSteps, b.MaxTokens
}
