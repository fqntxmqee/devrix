package sessionorchestrator

import "context"

// FocusHintProvider injects WorkTree focus context at turn start (v1.5 resolve hook).
type FocusHintProvider interface {
	FocusHint(ctx context.Context, sessionID string) string
}
