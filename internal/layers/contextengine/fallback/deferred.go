package fallback

import (
	"context"

	"github.com/devrix/devrix/internal/shared/types"
)

// NoOpDeferredInit is the V5a stub deferred initializer.
type NoOpDeferredInit struct{}

// Run records deferred init flags without performing IO.
func (NoOpDeferredInit) Run(_ context.Context, trusted bool, _ *types.Session) (types.DeferredInitResult, error) {
	if !trusted {
		return types.DeferredInitResult{}, nil
	}
	return types.DeferredInitResult{
		PluginInit:   true,
		SkillInit:    true,
		MCPPrefetch:  true,
		SessionHooks: true,
	}, nil
}
