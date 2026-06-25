package sessionorchestrator

import (
	"context"

	"github.com/devrix/devrix/internal/shared/contracts"
)

type toolStreamKey struct{}

// WithToolEventStream attaches a callback for tools that emit EngineEvents
// during ExecuteRound (e.g. delegate_wave). The Turn loop forwards these to
// the outbound channel.
func WithToolEventStream(ctx context.Context, emit func(*contracts.EngineEvent)) context.Context {
	if emit == nil {
		return ctx
	}
	return context.WithValue(ctx, toolStreamKey{}, emit)
}

// ToolEventStreamFrom returns the stream callback from ctx, if any.
func ToolEventStreamFrom(ctx context.Context) func(*contracts.EngineEvent) {
	if ctx == nil {
		return nil
	}
	if emit, ok := ctx.Value(toolStreamKey{}).(func(*contracts.EngineEvent)); ok {
		return emit
	}
	return nil
}
