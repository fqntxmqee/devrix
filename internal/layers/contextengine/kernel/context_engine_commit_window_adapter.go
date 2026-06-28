// CommitWindow adapter: D2-S17-A04 CommitWindow — implements
// persist.CommitWindowRunner using the facade's compression pipeline and
// memory manager.
//
// DSAFT: D2-S17-A04 (CommitWindow)
package kernel

import (
	"context"

	"github.com/devrix/devrix/internal/layers/contextengine/persist"
	"github.com/devrix/devrix/internal/shared/types"
)

// commitWindowAdapter is the facade's implementation of persist.CommitWindowRunner.
// It reuses the existing compression pipeline factory + memory store methods
// (SetActiveMessages, TrimMessages).
type commitWindowAdapter struct {
	engine *ContextEngine
}

// RunCommitWindow trims the active window when over MaxMessages or
// token-budget threshold.
func (a *commitWindowAdapter) RunCommitWindow(ctx context.Context, sc *types.SessionContext) (types.CompressionReport, error) {
	return persist.RunCommitWindow(ctx, persist.CommitWindowDeps{
		Store:    a.engine.memory,
		Pipeline: a.engine.compressionPipeline(sc.SessionID),
		// Cap active window at the larger of MaxMessages and a sensible
		// minimum (50) — matches facade/engine_persist.go::commitActiveWindow.
		MaxMessages: a.engine.cfg.Compression.MaxMessages,
		ShouldCompress: func(msgs []types.Message, budget types.TokenBudget) bool {
			return a.engine.shouldCompress(msgs, budget)
		},
	}, sc)
}

// newCommitWindowAdapter constructs the facade's CommitWindowRunner adapter.
func newCommitWindowAdapter(e *ContextEngine) *commitWindowAdapter {
	return &commitWindowAdapter{engine: e}
}

// Compile-time guard: commitWindowAdapter must satisfy persist.CommitWindowRunner.
var _ persist.CommitWindowRunner = (*commitWindowAdapter)(nil)
