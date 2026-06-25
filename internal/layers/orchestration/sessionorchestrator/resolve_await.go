package sessionorchestrator

import "context"

// ResolveAwaiter blocks at RunTurn start until focus running children complete.
type ResolveAwaiter interface {
	AwaitRunningChildren(ctx context.Context, sessionID string) string
}
