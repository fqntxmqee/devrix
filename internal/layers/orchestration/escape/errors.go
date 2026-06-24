package escape

import (
	"errors"

	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// Sentinel errors for the escape package. Use these as the unwrapped
// inner error; wrap with sharederrors.WithCode when returning to callers
// so the canonical code travels with the error.
//
// Codes live in the 7100-7199 range (orchestration escape subdomain).
var (
	// ErrLoopContextSessionIDRequired is returned when LoopContext.SessionID
	// is empty (must be a non-empty session identifier).
	ErrLoopContextSessionIDRequired = errors.New("escape: LoopContext SessionID required")

	// ErrLoopDepthTrackerNotInitialized is returned when a public method
	// is called on a zero-value LoopDepthTracker (callers must use New).
	ErrLoopDepthTrackerNotInitialized = errors.New("escape: LoopDepthTracker not initialized (use NewLoopDepthTracker)")

	// ErrLoopDepthExceeded is returned / wrapped when the configured
	// MaxDepth has been reached and a ForceExit decision is emitted.
	ErrLoopDepthExceeded = errors.New("escape: loop depth exceeded MaxDepth")

	// ErrInvalidMaxDepth is returned by NewLoopDepthTracker when MaxDepth < 1.
	ErrInvalidMaxDepth = errors.New("escape: MaxDepth must be >= 1")

	// ErrInvalidEscapeAction is returned when an unknown EscapeAction is
	// passed to processEscapeDecision (default branch兜底).
	ErrInvalidEscapeAction = errors.New("escape: invalid EscapeAction")
)

// Wrap helpers — produce a *sharederrors.SentinelError with a stable code.
// Code range: 7100-7199 (escape subdomain).
func NewLoopContextSessionIDRequiredError() *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_ESCAPE_LOOPCTX_7101",
		"LoopContext.SessionID is required",
		ErrLoopContextSessionIDRequired,
	)
}

func NewLoopDepthExceededError(depth, maxDepth int) *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_ESCAPE_LOOPDEPTH_7102",
		"loop depth exceeded MaxDepth",
		ErrLoopDepthExceeded,
	)
}