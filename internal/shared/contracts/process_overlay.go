package contracts

import "context"

type processOverlayKey struct{}

// ProcessOverlay carries per-Process worker identity for D4 agents.
type ProcessOverlay struct {
	AgentID      string
	ModelTier    string
	IsWorker     bool
	WorkerRole   string
	SystemPrompt string
}

// WithProcessOverlay attaches worker metadata to a Process call.
func WithProcessOverlay(ctx context.Context, ov ProcessOverlay) context.Context {
	return context.WithValue(ctx, processOverlayKey{}, ov)
}

// ProcessOverlayFromContext returns worker metadata when present.
func ProcessOverlayFromContext(ctx context.Context) (ProcessOverlay, bool) {
	ov, ok := ctx.Value(processOverlayKey{}).(ProcessOverlay)
	return ov, ok
}
