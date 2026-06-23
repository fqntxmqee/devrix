package sessionorchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
)

// processAutoClose wraps the path-returned EngineEvent channel and, after the
// channel closes, asynchronously triggers learner.Learn to close the LP-1
// loop in production (Phase 7 PR-7.1, D7-S13-A47-T01/T02).
//
// Verdict synthesis rules (D7-S13-A47-T02):
//
//	last.Type == "complete"   → VerdictPass
//	last.Type == "error"      → VerdictFail (Reason = event.Content)
//	last.Type == "tombstone"  → VerdictIndeterminate (IndeterminateReason = "interrupt")
//	other / nil last event    → nil (no Learn call)
//
// 3-layer fail-safe (D7-S13-A47-T02):
//
//	Layer 1: o.learner == nil           → direct endSpanWhenChannelClosed passthrough (no Learn)
//	Layer 2: empty channel / ctx cancel  → slog.Warn, no Learn call
//	Layer 3: Learn returns error         → slog.Warn, caller unaffected
//
// Skip path (IntentSkip) bypasses processAutoClose entirely: the empty closed
// channel from the skip branch in ProcessMessage (orchestrator.go:373-376)
// flows directly through endSpanWhenChannelClosed. There is no execution
// result to learn from, and no terminal EngineEvent to synthesize a Verdict
// from.
//
// Span lifecycle is preserved: the inner endSpanWhenChannelClosed is the
// canonical span-closing hook. processAutoClose only adds a post-close
// Verdict-synthesis + Learn side effect.
func (o *SessionOrchestrator) processAutoClose(
	ch <-chan *contracts.EngineEvent,
	sessionCtx context.Context,
	sessionSpan tracer.Span,
	sessionID string,
	intent orchtypes.IntentClassification,
) <-chan *contracts.EngineEvent {
	if o == nil || o.learner == nil {
		// Layer 1: nil learner → no Auto-Close, just close span when channel drains.
		return endSpanWhenChannelClosed(ch, sessionSpan)
	}
	out := make(chan *contracts.EngineEvent, 32)
	go func() {
		defer close(out)
		var lastEvent *contracts.EngineEvent
		for ev := range ch {
			lastEvent = ev
			out <- ev
		}
		// Channel closed. Real-span close is delegated to endSpanWhenChannelClosed
		// wrapping `out` below; here we only handle the Auto-Close side effect.
		//
		// Layer 2: empty channel or context cancellation → no Verdict, skip Learn.
		if lastEvent == nil {
			slog.Warn("orchestrator: processAutoClose skipped (empty channel, likely skip path or context cancel)",
				"session_id", sessionID, "intent_kind", intent.Kind)
			return
		}
		if sessionCtx != nil && sessionCtx.Err() != nil {
			slog.Warn("orchestrator: processAutoClose skipped (session context cancelled)",
				"session_id", sessionID, "err", sessionCtx.Err())
			return
		}
		verdict := synthesizeVerdict(lastEvent, sessionID)
		if verdict == nil {
			// Non-terminal last event (text / thinking / tool_call / etc.) — no Verdict to learn.
			return
		}
		req := learn.LearnRequest{
			SessionID: sessionID,
			Verdict:   *verdict,
		}
		// Layer 3: Learn error → log + skip, caller unaffected.
		if _, err := o.learner.Learn(sessionCtx, req); err != nil {
			slog.Warn("orchestrator: processAutoClose learner.Learn failed",
				"session_id", sessionID,
				"verdict_kind", verdict.Kind,
				"verdict_reason", verdict.Reason,
				"err", err)
		}
	}()
	// Wrap out with the real span closer so span lifecycle is identical to v1.0
	// endSpanWhenChannelClosed behavior (End fires when the proxy channel drains).
	return endSpanWhenChannelClosed(out, sessionSpan)
}

// synthesizeVerdict maps the last EngineEvent.Type to a workmodel.Verdict for
// the Learn deposit. Returns nil for non-terminal event types or empty input
// (Phase 7 PR-7.1, D7-S13-A47-T02).
//
// SourceID format: "autoclose:{sessionID}:{unixnano}" — embeds sessionID and a
// high-resolution timestamp so multiple Auto-Close events for the same
// session produce distinct SourceIDs.
func synthesizeVerdict(last *contracts.EngineEvent, sessionID string) *workmodel.Verdict {
	if last == nil {
		return nil
	}
	sourceID := fmt.Sprintf("autoclose:%s:%d", sessionID, time.Now().UnixNano())
	switch last.Type {
	case "complete":
		return &workmodel.Verdict{
			Kind:     types.VerdictPass,
			SourceID: sourceID,
			Reason:   "process complete",
		}
	case "error":
		reason := last.Content
		if reason == "" {
			reason = "process error (no content)"
		}
		return &workmodel.Verdict{
			Kind:     types.VerdictFail,
			SourceID: sourceID,
			Reason:   reason,
		}
	case "tombstone":
		return &workmodel.Verdict{
			Kind:                types.VerdictIndeterminate,
			SourceID:            sourceID,
			Reason:              "tombstone received",
			IndeterminateReason: "interrupt",
		}
	default:
		// Non-terminal event types: text / thinking / tool_call / tool_result /
		// status / permission. No Verdict to synthesize.
		return nil
	}
}
