package sessionorchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/devrix/devrix/internal/shared/contracts"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// durationOrDefault returns time.Millisecond * d, or a sane default if d <= 0.
func durationOrDefault(d int) time.Duration {
	if d <= 0 {
		return 50 * time.Millisecond
	}
	return time.Duration(d) * time.Millisecond
}

func emit(ctx context.Context, sink EventPublisher, out chan<- *contracts.EngineEvent, ev *contracts.EngineEvent) {
	_ = sink
	select {
	case out <- ev:
	case <-ctx.Done():
	}
}

func emitError(ctx context.Context, sink EventPublisher, out chan<- *contracts.EngineEvent, sessionID, label string, err error) {
	// DM-20260628-001 (FR-15): error_code metadata carries the closed-set
	// APIErrorCode (AC7). Falls back to "unknown" for bare errors.
	apiCode := sharederrors.Code(err).String()
	emit(ctx, sink, out, &contracts.EngineEvent{
		Type:      "error",
		Content:   fmt.Sprintf("%s: %s", label, sharederrors.SanitizeForUser(err)),
		SessionID: sessionID,
		Metadata:  map[string]string{"error_code": apiCode},
	})
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
