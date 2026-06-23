package plan

// BlastRadius estimates the worst-case blast radius of executing a Plan.
// Used by Plan.Validate() to enforce PP-3 (explosion radius constraint):
// a Plan whose BlastRadius exceeds config limits is rejected with
// ErrPlanBlastRadiusExceeded.
//
// Dimensions (4 axes, per doc 43 §4.4):
//   - FileCount: number of files touched
//   - APICallCount: number of tool invocations
//   - TokenCost: cumulative LLM token consumption estimate
//   - PersistScope: persistence scope (transient/session/permanent)
type BlastRadius struct {
	FileCount    int             `json:"file_count"`
	APICallCount int             `json:"api_call_count"`
	TokenCost    int             `json:"token_cost"`
	PersistScope PersistScope    `json:"persist_scope"`
}

// PersistScope enumerates the persistence lifetime of side effects.
type PersistScope string

const (
	// PersistTransient: no persistence (e.g. read-only probe)
	PersistTransient PersistScope = "transient"
	// PersistSession: side effect scoped to current session
	PersistSession PersistScope = "session"
	// PersistPermanent: side effect persists across sessions
	PersistPermanent PersistScope = "permanent"
)

// Valid reports whether the scope is recognized.
func (p PersistScope) Valid() bool {
	switch p {
	case PersistTransient, PersistSession, PersistPermanent:
		return true
	default:
		return false
	}
}

// Zero reports whether the BlastRadius is the zero value (no blast at all).
func (b BlastRadius) Zero() bool {
	return b.FileCount == 0 && b.APICallCount == 0 && b.TokenCost == 0 && b.PersistScope == ""
}

// FailureCriterion describes a falsifiability predicate (PP-2). The Verifier
// evaluates these against ExecutionEvidence to determine Pass/Fail.
//
// Field is the dotted-path key inside ExecutionEvidence (e.g. "exit_code",
// "diff_hash", "api_status", "duration_ms", "output_match").
//
// Op is one of: "eq", "ne", "gt", "lt", "in", "contains". Whitelist is enforced
// in Validator — see Validate() (PR-B2 scope).
type FailureCriterion struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value any    `json:"value"`
}

// Step is a single execution unit. The Channel executes Steps in order; on
// failure it consults the RetryPolicy (PR-C3) to decide whether to continue.
//
// IdempotencyKey is required for any Step whose ToolInvocation has side
// effects — see PR-C2 AC14.
type Step struct {
	ID              string         `json:"id"`
	Directive       string         `json:"directive"`
	ToolName        string         `json:"tool_name,omitempty"`
	ToolArgs        map[string]any `json:"tool_args,omitempty"`
	IdempotencyKey  string         `json:"idempotency_key,omitempty"`
	EstimatedTokens int            `json:"estimated_tokens,omitempty"`
}