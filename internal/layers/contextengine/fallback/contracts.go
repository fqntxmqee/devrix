package fallback

import (
	"context"

	"github.com/devrix/devrix/internal/shared/types"
)

// IHarnessBootstrap orchestrates harness bootstrap stages for a session.
type IHarnessBootstrap interface {
	Run(ctx context.Context, session *types.Session) (*types.HarnessSessionState, error)
}

// IDeferredInit performs trust-gated deferred initialization (V5a stub).
type IDeferredInit interface {
	Run(ctx context.Context, trusted bool, session *types.Session) (types.DeferredInitResult, error)
}

// IToolPoolFilter filters the visible tool set for a session.
type IToolPoolFilter interface {
	Filter(all []ToolDesc) []ToolDesc
}

// IPromptRouter scores tools/commands against the user prompt for advisory hints.
type IPromptRouter interface {
	Route(prompt string, tools []ToolDesc, limit int) types.RoutingHint
}

// IPreflightEvaluator evaluates assembled context before the LLM call.
type IPreflightEvaluator interface {
	Evaluate(
		sc *types.SessionContext,
		userMessage string,
		visibleTools []ToolDesc,
		assembledContext string,
	) types.PreflightResult
}

// StageEmitter emits bootstrap stage info events (optional).
type StageEmitter func(stage types.BootstrapStage, metadata map[string]string)
