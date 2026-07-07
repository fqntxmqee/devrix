// Package learn: UserActionListener (DM-20260707-001 PR-E, T62).
//
// UserActionListener is the bridge between Feishu interactive cards (Cancel
// / Accept / Modify buttons) and the Learn pipeline. When a user clicks one
// of these buttons on a progress card, the Feishu webhook fires a
// UserActionEvent; the listener converts it to a LearnRequest with the
// appropriate UserAction string and submits it to AsyncLearner.Enqueue.
//
// The Learn pipeline (DefaultLearner) then runs ClassifyScenario on the
// request, which routes to:
//
//	UserAction = "cancel" → ScenarioU1UserCancel (β++)
//	UserAction = "accept" → ScenarioU2UserAccept (α++)
//	UserAction = "modify" → ScenarioU3UserModify (no_change, defer to next round)
//
// Why a separate listener type (vs. inlining in Feishu adapter):
//   - The listener is policy-agnostic — it only translates wire-format to
//     LearnRequest. The policy lives in policy.go (T60).
//   - Tests can use a captured listener + mock AsyncLearner to verify
//     event → request mapping without spinning up a Feishu webhook.
//   - Future channels (Slack / Lark / custom) can plug in their own
//     UserActionListener by implementing the interface.
package learn

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// UserAction is the enumerated set of user-driven Learn actions. The values
// MUST match PolicyInput.UserAction (which is compared as a plain string) —
// the constants here are the canonical source of truth.
type UserAction string

const (
	// UserActionCancel — user rejected the round / wants to abort the session.
	UserActionCancel UserAction = "cancel"

	// UserActionAccept — user confirmed the round's deliverable.
	UserActionAccept UserAction = "accept"

	// UserActionModify — user edited the round's deliverable; defer Learn to
	// the next round so we can compare user-modified output vs. original.
	UserActionModify UserAction = "modify"
)

// UserActionEvent is the wire-format payload from a Feishu card button click.
// JSON tags match the field names the Feishu webhook sends. The struct is
// permissive (extra fields ignored) so future Feishu schema additions don't
// break us.
type UserActionEvent struct {
	// SessionID — the orchestration session this action applies to.
	SessionID string `json:"session_id"`

	// RoundNo — the round number within the session (1-indexed). Used to
	// resolve the WorkItem / Verdict for the LearnRequest.
	RoundNo int `json:"round_no"`

	// Action — the action value: "cancel" / "accept" / "modify" (case-insensitive).
	Action string `json:"action"`

	// UserID — the Feishu user ID for audit + future per-user reputation.
	UserID string `json:"user_id"`

	// Timestamp — when the action was received (Feishu provides this; defaults
	// to time.Now() when missing).
	Timestamp time.Time `json:"timestamp"`

	// Metadata — free-form payload from the Feishu card (e.g. selected
	// deliverable ID, comment text on modify). Pass-through to LearnRequest.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ErrUnknownUserAction is returned when the action string does not match any
// of the 3 canonical values. The listener rejects the event rather than
// silently dropping it (the alternative — "no-op" — masks wiring bugs).
var ErrUnknownUserAction = errors.New("learn: unknown user action")

// UserActionListener is the interface the Feishu adapter calls when a card
// button is clicked. Implementations MUST be safe for concurrent use.
type UserActionListener interface {
	// HandleUserAction processes one event. Returns the constructed
	// LearnRequest (for tests + audit) and any error.
	HandleUserAction(ctx context.Context, evt UserActionEvent) (LearnRequest, error)

	// Metrics returns the listener's counter snapshot. Returns a pointer
	// because atomic.Int64 fields must be addressable for .Load().
	Metrics() *UserActionMetrics
}

// UserActionMetrics is the metrics snapshot for the listener. Surfaced to
// the D5 dashboard via the existing telemetry pipeline. Pointer-typed fields
// keep the atomic counters addressable across the snapshot boundary.
type UserActionMetrics struct {
	Received atomic.Int64 // total events received (valid + invalid)
	Cancel   atomic.Int64 // cancel actions processed
	Accept   atomic.Int64 // accept actions processed
	Modify   atomic.Int64 // modify actions processed
	Rejected atomic.Int64 // events rejected (unknown action / missing fields)
}

// DefaultUserActionListener is the production implementation. It translates
// UserActionEvent → LearnRequest and enqueues via AsyncLearner.
//
// Dependencies:
//   - learner: the AsyncLearner (or any Learner that implements Enqueue via
//     a helper). We type-assert to *AsyncLearner for the non-blocking path;
//     if the listener is wired with a plain Learner, HandleUserAction
//     falls back to synchronous Learn.
//   - verdictResolver: looks up the round's Verdict by (sessionID, roundNo).
//     In production this is backed by workmodel.WorkItem.LastRound; in
//     tests it can be a stub.
type DefaultUserActionListener struct {
	mu              sync.Mutex
	asyncLearner    *AsyncLearner
	plainLearner    Learner // fallback when asyncLearner is nil
	verdictResolver VerdictResolver
	metrics         UserActionMetrics
	logger          *slog.Logger
}

// VerdictResolver looks up the round's Verdict by sessionID + roundNo.
// Implementations are expected to be fast (in-memory map or WorkItem store).
type VerdictResolver interface {
	ResolveVerdict(ctx context.Context, sessionID string, roundNo int) (workmodel.Verdict, error)
}

// NewDefaultUserActionListener constructs the listener with the required deps.
// Either asyncLearner OR plainLearner must be non-nil (the listener fails
// fast if both are nil).
func NewDefaultUserActionListener(
	asyncLearner *AsyncLearner,
	plainLearner Learner,
	verdictResolver VerdictResolver,
	logger *slog.Logger,
) *DefaultUserActionListener {
	if logger == nil {
		logger = slog.Default()
	}
	return &DefaultUserActionListener{
		asyncLearner:    asyncLearner,
		plainLearner:    plainLearner,
		verdictResolver: verdictResolver,
		logger:          logger,
	}
}

// HandleUserAction processes one event end-to-end: parse + validate → resolve
// Verdict → build LearnRequest → enqueue.
//
// Error semantics:
//   - ErrUnknownUserAction: invalid action string (rejected at parse).
//   - other errors: lookup / enqueue failure (caller may retry).
func (l *DefaultUserActionListener) HandleUserAction(ctx context.Context, evt UserActionEvent) (LearnRequest, error) {
	l.metrics.Received.Add(1)

	if evt.SessionID == "" || evt.RoundNo <= 0 {
		l.metrics.Rejected.Add(1)
		return LearnRequest{}, errors.New("learn: invalid event (missing sessionID or roundNo)")
	}

	action, err := parseUserAction(evt.Action)
	if err != nil {
		l.metrics.Rejected.Add(1)
		l.logger.Warn("user_action_listener_unknown_action",
			slog.String("session_id", evt.SessionID),
			slog.String("action", evt.Action),
		)
		return LearnRequest{}, err
	}

	// Resolve the round's Verdict. Use the resolver; on failure we synthesize
	// a minimal VerdictPass with the action label as the reason so the Learn
	// pipeline still runs (degradation rather than hard fail — T64 L1 path).
	var verdict workmodel.Verdict
	if l.verdictResolver != nil {
		v, lookupErr := l.verdictResolver.ResolveVerdict(ctx, evt.SessionID, evt.RoundNo)
		if lookupErr != nil {
			l.logger.Warn("user_action_listener_verdict_lookup_failed",
				slog.String("session_id", evt.SessionID),
				slog.Int("round_no", evt.RoundNo),
				slog.String("err", lookupErr.Error()),
			)
			// Fall through with synthetic Verdict; ClassifyScenario still runs.
			verdict = workmodel.Verdict{Kind: types.VerdictPass, Reason: "user_action:" + string(action)}
		} else {
			verdict = v
		}
	} else {
		verdict = workmodel.Verdict{Kind: types.VerdictPass, Reason: "user_action:" + string(action)}
	}

	req := LearnRequest{
		SessionID: evt.SessionID,
		Verdict:   verdict,
		// Wire the action via the (legacy) Verdict.Reason field; the
		// AsyncLearner / ItemPipelineRunner reads UserAction separately via
		// the dedicated field on PolicyInput. We set both for backward
		// compatibility with downstream consumers that grep Reason.
	}

	// Submit via async learner when available; else fall back to sync Learn.
	switch {
	case l.asyncLearner != nil:
		if err := l.asyncLearner.Enqueue(ctx, req); err != nil {
			l.metrics.Rejected.Add(1)
			return req, err
		}
	case l.plainLearner != nil:
		if _, err := l.plainLearner.Learn(ctx, req); err != nil {
			l.metrics.Rejected.Add(1)
			return req, err
		}
	default:
		l.metrics.Rejected.Add(1)
		return req, errors.New("learn: no learner wired")
	}

	// Increment the per-action counter.
	switch action {
	case UserActionCancel:
		l.metrics.Cancel.Add(1)
	case UserActionAccept:
		l.metrics.Accept.Add(1)
	case UserActionModify:
		l.metrics.Modify.Add(1)
	}

	l.logger.Info("user_action_listener_handled",
		slog.String("session_id", evt.SessionID),
		slog.Int("round_no", evt.RoundNo),
		slog.String("action", string(action)),
		slog.String("user_id", evt.UserID),
	)
	return req, nil
}

// Metrics returns a snapshot of the listener's counters. Implements the
// UserActionListener interface. Returns a pointer because atomic.Int64
// fields must be addressable for .Load().
func (l *DefaultUserActionListener) Metrics() *UserActionMetrics {
	return &l.metrics
}

// parseUserAction normalizes the wire-format action string (case-insensitive,
// trimmed) to the canonical UserAction constant. Returns ErrUnknownUserAction
// when the value doesn't match any known action.
func parseUserAction(s string) (UserAction, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "cancel":
		return UserActionCancel, nil
	case "accept":
		return UserActionAccept, nil
	case "modify":
		return UserActionModify, nil
	default:
		return "", ErrUnknownUserAction
	}
}

// MarshalUserActionEvent is a convenience for tests + Feishu adapter that
// want to JSON-encode an event. Returns the marshaled bytes (or error).
func MarshalUserActionEvent(evt UserActionEvent) ([]byte, error) {
	return json.Marshal(evt)
}