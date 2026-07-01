package kernel

import (
	"log/slog"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/compression"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	"github.com/devrix/devrix/internal/shared/types"
)

// sessionAutocompactSink is the production CompressionEventSink that
// performs the autocompact writeback (RH-D2-03 / RH-D2-04, DM-20260630-013).
//
// Without this sink, async autocompact summarization produced a summary
// message that was emitted to observability but never written back into the
// in-memory SessionContext — leaving the original [compressing ...] pending
// placeholder visible to subsequent turns.
//
// The sink replaces the first pending autocompact placeholder in the
// session's Messages with the real summary message, marking metadata
// status="complete". A pending placeholder is identified by
// metadata["compressed_by"]=="autocompact" && metadata["status"]=="pending".
//
// Concurrency: SetActiveMessages holds messagesMu, so the in-place
// replacement is safe against concurrent reads from Prepare/turn loops.
//
// DSAFT: D2-S15-A80 (Autocompact Writeback, RH-D2-03/04).
type sessionAutocompactSink struct {
	memory *memory.Manager
	mu     sync.Mutex
	// lastReplaced tracks (sessionID,asyncToken) so that stale observers
	// from previous summarizations cannot rewrite history.
	lastReplaced map[string]string
	now          func() time.Time
}

// NewSessionAutocompactSink wires the production writeback sink to a memory
// manager. When memory is nil the sink is a no-op (defensive — async
// autocompact is opt-in).
func NewSessionAutocompactSink(mem *memory.Manager) compression.CompressionEventSink {
	return &sessionAutocompactSink{
		memory:       mem,
		lastReplaced: make(map[string]string),
		now:          time.Now,
	}
}

// EmitCompressionStep records pipeline step transitions. The sink only
// performs writeback; observability for steps is handled by the bridge
// layer (TracingStepObserver). Kept as a no-op so the interface stays
// satisfied when this sink is the only observer wired in.
func (s *sessionAutocompactSink) EmitCompressionStep(sessionID, step string, before, after int) {
	if s == nil || s.memory == nil {
		return
	}
	slog.Debug("contextengine.autocompact.writeback.step",
		"session_id", sessionID, "step", step,
		"before", before, "after", after)
}

// EmitAutocompact records the start of an autocompact pass. Not used by
// writeback logic but exposed for symmetry / future metrics.
func (s *sessionAutocompactSink) EmitAutocompact(sessionID string, meta AutocompactMeta) {
	if s == nil || s.memory == nil {
		return
	}
	slog.Debug("contextengine.autocompact.writeback.start",
		"session_id", sessionID, "degraded", meta.Degraded, "model", meta.Model)
}

// EmitAutocompactComplete replaces the pending placeholder in the session's
// message window with the actual summary produced by the async summarizer.
// On success the placeholder's metadata status flips from "pending" to
// "complete". Missing placeholders are not an error (e.g. when a newer
// placeholder has already been installed) — the writeback is best-effort
// with explicit logging so we can detect regressions.
func (s *sessionAutocompactSink) EmitAutocompactComplete(sessionID string, summary types.Message, asyncToken string) {
	if s == nil || s.memory == nil || sessionID == "" {
		return
	}
	sc, ok := s.memory.Get(sessionID)
	if !ok || sc == nil {
		slog.Warn("contextengine.autocompact.writeback.no_session",
			"session_id", sessionID, "async_token", asyncToken)
		return
	}

	s.mu.Lock()
	previous, seen := s.lastReplaced[sessionID]
	s.mu.Unlock()
	if seen && previous == asyncToken {
		// Same token already applied — observer re-fire, no-op.
		return
	}

	replaced := s.memory.ReplaceAutocompactPlaceholder(sc, summary, asyncToken)
	if !replaced {
		slog.Warn("contextengine.autocompact.writeback.no_placeholder",
			"session_id", sessionID, "async_token", asyncToken)
		return
	}

	s.mu.Lock()
	s.lastReplaced[sessionID] = asyncToken
	s.mu.Unlock()
	slog.Info("contextengine.autocompact.writeback.ok",
		"session_id", sessionID, "async_token", asyncToken,
		"summary_bytes", len(summary.Content))
}

func (s *sessionAutocompactSink) EmitAutocompactFailed(sessionID string, restored []types.Message, asyncToken string) {
	if s == nil || s.memory == nil || sessionID == "" {
		return
	}
	sc, ok := s.memory.Get(sessionID)
	if !ok || sc == nil {
		slog.Warn("contextengine.autocompact.restore.no_session",
			"session_id", sessionID, "async_token", asyncToken)
		return
	}
	if !s.memory.RestoreAutocompactPlaceholder(sc, restored, asyncToken) {
		slog.Warn("contextengine.autocompact.restore.no_placeholder",
			"session_id", sessionID, "async_token", asyncToken)
		return
	}
	slog.Info("contextengine.autocompact.restore.ok",
		"session_id", sessionID, "async_token", asyncToken, "restored_messages", len(restored))
}
