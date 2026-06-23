// Package learn: Learner interface + DefaultLearner (PR-E5 E5.1).
//
// Learner is the Learn node's contract. It exposes three responsibilities:
//
//   Learn          — translate a Verdict (+ Plan + Observation + Artifact)
//                    into a typed LearningAsset, route it to the right
//                    Memory channel (LP-2), and update ReputationStore
//                    (LP-3).
//   Inject         — build an AdaptivePrior for the next Observe call
//                    (LP-1 closed loop). Read-side; no writes.
//   ScheduledTick  — drain ScheduledMemory entries whose TriggerAt has
//                    passed; re-queue or escalate to FeedbackMemory when
//                    MaxRetries is exhausted.
//
// DefaultLearner is the production implementation that wires together
// SkillMemory + FeedbackMemory + ScheduledMemory + ReputationStore +
// AssetBuilder + BayesianUpdater.
package learn

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// Learner is the LP-1 closed-loop node contract.
type Learner interface {
	// Learn ingests one upstream LearnRequest and returns the assets
	// constructed (typically 1; LearningPending produces 1 in ScheduledMemory
	// plus an optional feedback entry on retry-exhaustion).
	//
	// Side effects (Memory.Store + ReputationStore.Update) happen in this
	// call. The Bayesian update uses the prior returned by
	// ReputationStore.Get.
	Learn(ctx context.Context, req LearnRequest) ([]*LearningAsset, error)

	// Inject returns an AdaptivePrior for the next Observe call. Reads
	// ReputationStore (no writes); returns a fail-safe prior when the store
	// has no row for sessionID (cold start) or returns an error.
	Inject(ctx context.Context, sessionID string) (*AdaptivePrior, error)

	// ScheduledTick drains ScheduledMemory entries whose TriggerAt has
	// passed. Exhausted retries are escalated to FeedbackMemory (warning
	// channel) and removed from ScheduledMemory.
	ScheduledTick(ctx context.Context) error
}

// DefaultLearner is the production Learner implementation. All dependencies
// are injected so tests can swap in in-memory mocks.
type DefaultLearner struct {
	SkillMem     Memory
	FeedbackMem  Memory
	ScheduledMem *ScheduledMemory // typed (not Memory) so we can call ListDue
	Reputation   ReputationStore
	Builder      *AssetBuilder
	// BayesianUpdater is the verdict → prior-evidence mutator. Injected so
	// tests can supply a stub; production passes BayesianUpdate.
	BayesianUpdater func(prior *ReputationEvidence, v workmodel.Verdict) *ReputationEvidence
}

// NewDefaultLearner wires the 3 memory channels + reputation store +
// builder + BayesianUpdater (defaults to BayesianUpdate if nil).
func NewDefaultLearner(
	skill, feedback Memory,
	scheduled *ScheduledMemory,
	rep ReputationStore,
	builder *AssetBuilder,
) *DefaultLearner {
	if builder == nil {
		builder = NewAssetBuilder()
	}
	return &DefaultLearner{
		SkillMem:        skill,
		FeedbackMem:     feedback,
		ScheduledMem:    scheduled,
		Reputation:      rep,
		Builder:         builder,
		BayesianUpdater: BayesianUpdate,
	}
}

// Learn is the LP-1 deposit phase: Verdict → LearningAsset + ReputationStore
// update. Returns the assets stored (typically 1; LearningPending produces 1
// in ScheduledMemory; on retry-exhaustion the asset is moved to FeedbackMemory
// which counts as 1 in the result).
func (l *DefaultLearner) Learn(ctx context.Context, req LearnRequest) ([]*LearningAsset, error) {
	if req.SessionID == "" {
		return nil, ErrAssetIncomplete
	}
	if l.Builder == nil {
		return nil, ErrAssetBuildFailed
	}

	class := classFromVerdictKind(req.Verdict.Kind, req.Verdict.Reason)
	if class == LearningClass(types.LearningUnknown) {
		return nil, fmt.Errorf("%w: unsupported verdict kind %v", ErrAssetBuildFailed, req.Verdict.Kind)
	}

	asset, err := l.Builder.Build(ctx, req, class)
	if err != nil {
		return nil, err
	}
	if asset == nil {
		return nil, ErrAssetBuildFailed
	}

	// LP-2: route to the correct Memory channel.
	switch class {
	case LearningClass(types.LearningSOP), LearningClass(types.LearningProtocol):
		if err := l.SkillMem.Store(ctx, asset); err != nil {
			return nil, fmt.Errorf("learn: SkillMemory.Store: %w", err)
		}
	case LearningClass(types.LearningKnowledge), LearningClass(types.LearningConclusion):
		if err := l.FeedbackMem.Store(ctx, asset); err != nil {
			return nil, fmt.Errorf("learn: FeedbackMemory.Store: %w", err)
		}
	case LearningClass(types.LearningPending):
		// LearningPending always goes to ScheduledMemory (LP-2). It may
		// later be escalated to FeedbackMemory by ScheduledTick on
		// MaxRetries exhaustion.
		if err := l.ScheduledMem.Store(ctx, asset); err != nil {
			return nil, fmt.Errorf("learn: ScheduledMemory.Store: %w", err)
		}
	default:
		return nil, ErrAssetClassMismatch
	}

	// LP-3: Bayesian update on ReputationStore.
	if l.Reputation != nil && l.BayesianUpdater != nil {
		prior, err := l.Reputation.Get(ctx, req.SessionID)
		if err != nil && !errors.Is(err, ErrReputationStoreUnavailable) {
			return nil, fmt.Errorf("learn: ReputationStore.Get: %w", err)
		}
		if prior == nil {
			// Cold start: bootstrap with Developer track (fail-safe).
			prior, err = NewReputationEvidence(req.SessionID, TrackModeDeveloper)
			if err != nil {
				return nil, fmt.Errorf("learn: cold-start reputation: %w", err)
			}
		}
		next := l.BayesianUpdater(prior, req.Verdict)
		if err := l.Reputation.Update(ctx, next); err != nil {
			return nil, fmt.Errorf("learn: ReputationStore.Update: %w", err)
		}
	}

	return []*LearningAsset{asset}, nil
}

// Inject is the LP-1 read phase: read ReputationStore → BuildAdaptivePrior.
func (l *DefaultLearner) Inject(ctx context.Context, sessionID string) (*AdaptivePrior, error) {
	if sessionID == "" {
		return nil, ErrAdaptivePriorNotReady
	}
	var (
		rep *ReputationEvidence
		err error
	)
	if l.Reputation != nil {
		rep, err = l.Reputation.Get(ctx, sessionID)
		if err != nil && !errors.Is(err, ErrReputationStoreUnavailable) {
			return nil, fmt.Errorf("learn: ReputationStore.Get: %w", err)
		}
	}
	trackMode := TrackModeDeveloper
	if rep != nil && rep.TrackMode != "" {
		trackMode = rep.TrackMode
	}
	return BuildAdaptivePrior(rep, trackMode), nil
}

// ScheduledTick drains ScheduledMemory entries whose TriggerAt has passed:
//   - RetryCount < MaxRetries → increment RetryCount, push TriggerAt forward
//   - RetryCount >= MaxRetries → escalate to FeedbackMemory + delete from
//     ScheduledMemory
//
// This method is idempotent — calling it twice in the same instant is a
// no-op (no entries past TriggerAt).
func (l *DefaultLearner) ScheduledTick(ctx context.Context) error {
	if l.ScheduledMem == nil {
		return nil
	}
	now := time.Now()
	due := l.ScheduledMem.ListDue(now)
	for _, retry := range due {
		if retry.IsExhausted() {
			// Escalate to FeedbackMemory as a "knowledge" warning asset.
			escalated := &LearningAsset{
				ID:               NewAssetID(),
				SessionID:        retry.Asset.SessionID,
				Class:            LearningClass(types.LearningKnowledge),
				Strength:         StrengthKnowledge,
				SourceSessionIDs: retry.Asset.SourceSessionIDs,
				SourceVerdictIDs: retry.Asset.SourceVerdictIDs,
				Content: &KnowledgeAssetContent{
					Topic:      "scheduled_retry_exhausted",
					Hypothesis: fmt.Sprintf("retry budget exhausted for artifact %s", retry.Asset.AssetKey),
					Evidence:   []string{retry.Asset.AssetKey},
					Confidence: 0.0,
				},
				AssetKey:         "escalated:" + retry.Asset.AssetKey,
				FailureCriterion: "manual_review",
				CreatedAt:        now,
				ExpiryAt:         now.Add(DefaultAssetTTL),
				LastUsedAt:       now,
			}
			if err := l.FeedbackMem.Store(ctx, escalated); err != nil {
				return fmt.Errorf("learn: FeedbackMem.Store (escalation): %w", err)
			}
			if err := l.ScheduledMem.Delete(ctx, retry.Asset.AssetKey); err != nil {
				return fmt.Errorf("learn: ScheduledMem.Delete (escalation): %w", err)
			}
			continue
		}
		// Increment retry counter + push TriggerAt forward by 5 minutes.
		retry.RetryCount++
		retry.LastRetryAt = now
		retry.TriggerAt = now.Add(5 * time.Minute)
	}
	return nil
}