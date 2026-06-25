package orchtypes

import (
	"fmt"

	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
)

// ObserveRequest is the unified input to all Observer submodules
// (IntentQuantizer + AnomalyDetector + RuleClassifier). It carries
// the AdaptivePrior from Learner.Inject so the Observer submodules
// can read the user's cross-session reputation as a confidence /
// threshold multiplier.
//
// Phase 6 PR-F1 (D7-S12-A41-T01). Prior field is nullable: prior == nil
// means "no prior" (cold start or omitted). EffectivePrior() returns the
// actual prior to use (prior if non-nil, else a freshly-built
// DefaultDeveloperPrior).
type ObserveRequest struct {
	// SessionID — owner session (必填, used as the key into ReputationStore).
	SessionID string

	// Message — raw user message (必填, source for all 3 Observer submodules).
	Message string

	// Prior — AdaptivePrior from Learner.Inject. May be nil; readers should
	// call EffectivePrior() to get a fail-safe default.
	Prior *learn.AdaptivePrior
}

// NewObserveRequest constructs an ObserveRequest with fail-fast validation.
// sessionID and message must be non-empty; prior is optional.
func NewObserveRequest(sessionID, message string, prior *learn.AdaptivePrior) (ObserveRequest, error) {
	if sessionID == "" {
		return ObserveRequest{}, fmt.Errorf("orchtypes: ObserveRequest.SessionID is empty")
	}
	if message == "" {
		return ObserveRequest{}, fmt.Errorf("orchtypes: ObserveRequest.Message is empty")
	}
	return ObserveRequest{
		SessionID: sessionID,
		Message:   message,
		Prior:     prior,
	}, nil
}

// EffectivePrior returns the prior to use for downstream Observer
// submodules. If Prior is set, returns it. Otherwise constructs a
// DefaultDeveloperPrior (Beta(5,3)) so cold-start callers get a
// non-nil prior to read.
//
// The returned AdaptivePrior is freshly built on each call when Prior
// is nil; callers MUST NOT mutate it (immutable pattern, same as
// ReputationEvidence).
func (r ObserveRequest) EffectivePrior() *learn.AdaptivePrior {
	if r.Prior != nil {
		return r.Prior
	}
	return learn.BuildAdaptivePrior(nil, learn.TrackModeDeveloper)
}

// Validate returns nil if the request is well-formed. Required: non-empty
// SessionID and Message. Prior is optional.
func (r ObserveRequest) Validate() error {
	if r.SessionID == "" {
		return fmt.Errorf("orchtypes: ObserveRequest.SessionID is empty")
	}
	if r.Message == "" {
		return fmt.Errorf("orchtypes: ObserveRequest.Message is empty")
	}
	return nil
}
