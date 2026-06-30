package sessionorchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// TurnState serializes turns per session_id so that turn N+1 cannot enter
// RunSessionTurnLoop while turn N is still draining its out channel.
//
// Without this, two goroutines for the same session_id race on the
// executor.Emit hook (the bug behind sess_1782638991113_5000 panic of
// 2026-06-28, hotfixed in PR #271). PR #271 was a defensive recover +
// per-Run overwrite; this Change is the architectural fix.
//
// Lifecycle:
//   - BeginTurn(sessionID) reserves a slot, returns TurnInProgressError
//     if a previous turn is still open.
//   - EndTurn(sessionID) closes the slot (idempotent via sync.Once per
//     turnHandle — calling EndTurn twice on the same handle is safe).
//   - WaitTurn(ctx, sessionID) blocks until EndTurn or ctx cancellation.
//
// In-memory only. Session close / cleanup is the orchestrator's
// responsibility (we never auto-delete handles here to keep semantics
// simple; map entries are small and devrix is single-process).
type TurnState struct {
	mu      sync.RWMutex
	handles map[string]*turnHandle
}

// turnHandle is one in-flight turn's bookkeeping.
type turnHandle struct {
	turnNo    int
	startedAt time.Time
	done      chan struct{}
	closeOnce sync.Once // guarantees close(done) happens exactly once
}

// NewTurnState constructs an empty TurnState.
func NewTurnState() *TurnState {
	return &TurnState{handles: make(map[string]*turnHandle)}
}

// BeginTurn reserves a turn slot for sessionID. Returns TurnInProgressError
// if a previous turn is still open (handle.done not yet closed). If the
// previous handle is already closed (stale leftover from a crash), it is
// silently replaced.
func (ts *TurnState) BeginTurn(sessionID string) error {
	if ts == nil {
		return nil // nil receiver = disabled (legacy/test path)
	}
	if sessionID == "" {
		return fmt.Errorf("turn_state: sessionID required")
	}
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if prev, ok := ts.handles[sessionID]; ok {
		select {
		case <-prev.done:
			// stale — previous turn finished but handle was not cleaned.
			// safe to replace; no goroutine is waiting on prev.done now
			// (WaitTurn reads handle under RLock and could miss this
			// replacement, but that path is acceptable — the new handle
			// just won't be observed by that WaitTurn).
		default:
			return TurnInProgressError{
				SessionID:      sessionID,
				SinceStartedAt: prev.startedAt,
				TurnNo:         prev.turnNo,
			}
		}
	}
	ts.handles[sessionID] = &turnHandle{
		turnNo:    ts.nextTurnNoLocked(sessionID),
		startedAt: time.Now(),
		done:      make(chan struct{}),
	}
	return nil
}

// nextTurnNoLocked returns turnNo+1 for the session. Caller must hold ts.mu.
func (ts *TurnState) nextTurnNoLocked(sessionID string) int {
	if prev, ok := ts.handles[sessionID]; ok {
		return prev.turnNo + 1
	}
	return 1
}

// EndTurn closes the handle's done channel and purges the map entry.
// Idempotent — repeated calls are safe (sync.Once guarantees close(done)
// happens exactly once per handle, and the second delete is a no-op).
// No-op if no handle exists for sessionID.
//
// RH-D7-07 (DM-20260630-013): previously the handle was retained in the
// map after close, which meant long-running devrix processes accumulated
// one map entry per completed session — unbounded growth across the
// devrix lifetime. After purge, WaitTurn called for a session that has
// already ended returns nil immediately (per the no-handle branch), which
// matches the documented semantics ("WaitTurn late-arrival returns nil
// when no handle exists").
func (ts *TurnState) EndTurn(sessionID string) {
	if ts == nil || sessionID == "" {
		return
	}
	ts.mu.Lock()
	defer ts.mu.Unlock()
	h, ok := ts.handles[sessionID]
	if !ok {
		return
	}
	h.closeOnce.Do(func() { close(h.done) })
	delete(ts.handles, sessionID)
}

// WaitTurn blocks until the current turn for sessionID ends (EndTurn
// called) or ctx is canceled. Returns ctx.Err() on cancellation. No-op
// when no handle exists (BeginTurn not called yet).
func (ts *TurnState) WaitTurn(ctx context.Context, sessionID string) error {
	if ts == nil || sessionID == "" {
		return nil
	}
	ts.mu.RLock()
	h, ok := ts.handles[sessionID]
	ts.mu.RUnlock()
	if !ok {
		return nil
	}
	select {
	case <-h.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// IsTurnInProgress reports whether sessionID currently has an open turn.
// Useful for tests and diagnostics.
func (ts *TurnState) IsTurnInProgress(sessionID string) bool {
	if ts == nil || sessionID == "" {
		return false
	}
	ts.mu.RLock()
	h, ok := ts.handles[sessionID]
	ts.mu.RUnlock()
	if !ok {
		return false
	}
	select {
	case <-h.done:
		return false
	default:
		return true
	}
}

// TurnNo returns the current turn number for sessionID (0 if no handle).
func (ts *TurnState) TurnNo(sessionID string) int {
	if ts == nil || sessionID == "" {
		return 0
	}
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	if h, ok := ts.handles[sessionID]; ok {
		return h.turnNo
	}
	return 0
}

// TurnInProgressError is returned by BeginTurn when a previous turn for
// the same session_id has not yet ended. Use errors.As (struct) or
// errors.Is (with target TurnInProgressError{}) to detect.
//
// Defined in shared/errors; aliased here for D7 call sites.
type TurnInProgressError = sharederrors.TurnInProgressError

// Legacy struct definition removed — see shared/errors/turn_in_progress.go.