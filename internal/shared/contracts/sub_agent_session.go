package contracts

import (
	"context"

	"github.com/devrix/devrix/internal/shared/types"
)

type subAgentSessionKey struct{}

// WithSubAgentSession attaches a forked worker SessionContext for nested turns.
// turn_adapter.ExecuteRound prefers this over the main session lookup when present.
func WithSubAgentSession(ctx context.Context, sc *types.SessionContext) context.Context {
	if sc == nil {
		return ctx
	}
	return context.WithValue(ctx, subAgentSessionKey{}, sc)
}

// SubAgentSessionFromContext returns the forked worker session when present.
func SubAgentSessionFromContext(ctx context.Context) (*types.SessionContext, bool) {
	sc, ok := ctx.Value(subAgentSessionKey{}).(*types.SessionContext)
	return sc, ok && sc != nil
}
