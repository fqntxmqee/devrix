package sessionorchestrator

import (
	"context"

	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
)

// effectiveUserID resolves the user identifier for adaptive thresholds.
// Priority: ProcessRequest.UserID → Metadata["user_id"] → baggage user.id.
func effectiveUserID(ctx context.Context, req orchtypes.ProcessRequest) string {
	if req.UserID != "" {
		return req.UserID
	}
	if req.Metadata != nil {
		if u := req.Metadata["user_id"]; u != "" {
			return u
		}
	}
	if u, ok := tracer.DefaultBaggageManager.Get(ctx, "user.id"); ok && u != "" {
		return u
	}
	return ""
}
