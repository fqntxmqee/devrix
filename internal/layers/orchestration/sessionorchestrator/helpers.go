package sessionorchestrator

import (
	"context"
	"fmt"
	"log/slog"
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

// emit forwards an EngineEvent to the per-turn `out` channel.
//
// Hotfix 2026-06-28 (DM-20260628-002): the previous implementation panicked
// with `send on closed channel` when a multi-turn session's later
// RunSessionTurnLoop invocation inherited a stale executor.Emit hook from
// an earlier turn whose owning goroutine had already closed its `out`
// channel via `defer close(out)`. Production crash observed on
// sess_1782638991113_5000 (review d2 domain kernel + followup "you didn't
// give review conclusion, what's wrong?").
//
// The channel-closure is a legitimate lifecycle event (the goroutine that
// owns `out` has finished) and dropping late events is the right behaviour
// for a streaming fan-out — a recovering panic + slog.Warn keeps the
// orchestrator alive while preserving an audit trail. Architectural fix in
// item_pipeline.go ensures each Run() picks up the freshest r.Emit so this
// recover path should rarely fire.
func emit(ctx context.Context, sink EventPublisher, out chan<- *contracts.EngineEvent, ev *contracts.EngineEvent) {
	_ = sink
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("orchestrator: emit recovered from closed-channel panic (out channel already closed by owning goroutine)",
				"session_id", ev.SessionID,
				"event_type", ev.Type,
				"recover", fmt.Sprint(r))
		}
	}()
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
