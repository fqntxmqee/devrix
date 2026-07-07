// Package learn: Learner interface + DefaultLearner (PR-E5 E5.1).
//
// Learner is the Learn node's contract. It exposes three responsibilities:
//
//   Learn          — translate a Verdict (+ Plan + Observation + Artifact)
//                    into a typed LearningAsset, route it to the right
//                    memory.Memory channel (LP-2), and update ReputationStore
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
	"log/slog"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn/asset"
	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn/memory"
	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn/prior"
	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn/reputation"
	"github.com/devrix/devrix/internal/layers/orchestration/hardening"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// Re-export commonly used subpackage types so callers (DefaultLearner, tests,
// external packages) can keep using the short learn.XXX qualifier.
type (
	// LearnRequest — re-exported from asset.AssetBuilder.
	LearnRequest = asset.LearnRequest

	// LearningAsset — re-exported from asset.
	LearningAsset = asset.LearningAsset

	// LearningClass — re-exported from asset (= types.LearningClass).
	LearningClass = asset.LearningClass

	// AssetBuilder — re-exported from asset.
	AssetBuilder = asset.AssetBuilder

	// KnowledgeAssetContent / PendingAssetContent — re-exported from asset.
	KnowledgeAssetContent = asset.KnowledgeAssetContent
	PendingAssetContent   = asset.PendingAssetContent

	// AdaptivePrior — re-exported from prior.
	AdaptivePrior = prior.AdaptivePrior

	// ReputationEvidence / ReputationStore — re-exported from reputation.
	ReputationEvidence = reputation.ReputationEvidence
	ReputationStore    = reputation.ReputationStore

	// ParentEvidence / ChildVerdict — re-exported from reputation (PR-C).
	// ParentEvidence is the rollup aggregator's snapshot; ChildVerdict is
	// one child's terminal Verdict + SegmentID for rollup audit lineage.
	ParentEvidence = reputation.ParentEvidence
	ChildVerdict   = reputation.ChildVerdict

	// ScheduledMemory — re-exported from memory (concrete type, not the
	// memory.Memory interface, so DefaultLearner can call ListDue).
	ScheduledMemory = memory.ScheduledMemory
)

// Re-export constructor + error + helper symbols used by callers.
var (
	ErrAssetIncomplete         = asset.ErrAssetIncomplete
	ErrAssetClassMismatch      = asset.ErrAssetClassMismatch
	ErrAssetBuildFailed        = asset.ErrAssetBuildFailed
	ErrScheduledRetryExhausted = asset.ErrScheduledRetryExhausted
	ErrAdaptivePriorNotReady   = prior.ErrAdaptivePriorNotReady
	ErrReputationStoreUnavailable = reputation.ErrReputationStoreUnavailable
)

// Strength re-exports from asset.
const (
	StrengthUnknown   = asset.StrengthUnknown
	StrengthPending   = asset.StrengthPending
	StrengthConclusion = asset.StrengthConclusion
	StrengthKnowledge = asset.StrengthKnowledge
	StrengthProtocol  = asset.StrengthProtocol
	StrengthSOP       = asset.StrengthSOP
)

// DefaultAssetTTL re-export.
const DefaultAssetTTL = asset.DefaultAssetTTL

// Track mode re-exports.
const (
	TrackModeDeveloper = reputation.TrackModeDeveloper
	TrackModeOperator  = reputation.TrackModeOperator
)

// Learning asset class re-exports (so callers can do
// learn.LearningSOP / learn.LearningProtocol / etc).
const (
	LearningUnknown   = types.LearningUnknown
	LearningSOP       = types.LearningSOP
	LearningProtocol  = types.LearningProtocol
	LearningKnowledge = types.LearningKnowledge
	LearningConclusion = types.LearningConclusion
	LearningPending   = types.LearningPending
)

// Constructor re-exports.
var (
	NewAssetBuilder        = asset.NewAssetBuilder
	NewAssetID             = asset.NewAssetID
	NewReputationEvidence  = reputation.NewReputationEvidence
	BayesianUpdate         = reputation.BayesianUpdate
	BuildAdaptivePrior     = prior.BuildAdaptivePrior

	// PR-C rollup aggregator helpers — re-exported so sessionorchestrator
	// can call them without importing reputation directly (keeps the
	// learn→reputation import boundary clean for downstream callers).
	AggregateParentEvidence  = reputation.AggregateParentEvidence
	SynthesizeRollupVerdict  = reputation.SynthesizeRollupVerdict

	// Memory constructors (re-exported from memory package so callers
	// don't need to import the subpackage just to construct the default
	// in-memory implementations).
	NewSkillMemory     = memory.NewSkillMemory
	NewFeedbackMemory  = memory.NewFeedbackMemory
	NewScheduledMemory = memory.NewScheduledMemory
	NewInMemoryReputationStore = reputation.NewInMemoryReputationStore
)

// Re-export MemoryFilter so callers (e.g. tests) can build filters without
// importing the memory subpackage.
type MemoryFilter = memory.MemoryFilter

// Prior constants + BetaPrior re-exported from prior package.
// (DefaultDeveloperPrior / DefaultOperatorPrior are vars, not consts, since
// BetaPrior is a struct; re-export as var to preserve mutability semantics.)
var (
	DefaultDeveloperPrior = prior.DefaultDeveloperPrior
	DefaultOperatorPrior  = prior.DefaultOperatorPrior
)
type BetaPrior = prior.BetaPrior

// InMemoryReputationStore re-exported from reputation.
type InMemoryReputationStore = reputation.InMemoryReputationStore

// Memory (LP-2 channel contract interface) re-exported from memory.
type Memory = memory.Memory
type MemoryChannel = memory.MemoryChannel

// Concrete memory channel types re-exported from memory.
type SkillMemory = memory.SkillMemory
type FeedbackMemory = memory.FeedbackMemory

// ObservationLookup re-exported from asset (AssetBuilder input).
type ObservationLookup = asset.ObservationLookup

// Learner is the LP-1 closed-loop node contract.
type Learner interface {
	// Learn ingests one upstream LearnRequest and returns the assets
	// constructed (typically 1; LearningPending produces 1 in ScheduledMemory
	// plus an optional feedback entry on retry-exhaustion).
	//
	// Side effects (memory.Memory.Store + ReputationStore.Update) happen in this
	// call. The Bayesian update uses the prior returned by
	// ReputationStore.Get.
	Learn(ctx context.Context, req LearnRequest) ([]*LearningAsset, error)

	// Inject returns an AdaptivePrior for the next Observe call. Reads
	// ReputationStore (no writes); returns a fail-safe prior when the store
	// has no row for sessionID (cold start) or returns an error.
	//
	// Phase 7 PR-7.2 (D7-S13-A48-T05): trackModeHint is a per-request hint
	// ("developer" / "operator" / "") that selects the prior Beta:
	//   - "" (zero value) → use Reputation's TrackMode if present, else Developer
	//   - "developer"     → use DefaultDeveloperPrior (Beta(5,3))
	//   - "operator"      → use DefaultOperatorPrior (Beta(8,1))
	//   - other non-empty → log warn + fall back to Developer
	// When the ReputationStore has a row for sessionID, the Reputation's
	// TrackMode takes precedence over the hint (cross-session state wins
	// over the per-request hint, since the row is the persistent record).
	Inject(ctx context.Context, sessionID, trackModeHint string) (*AdaptivePrior, error)

	// ScheduledTick drains ScheduledMemory entries whose TriggerAt has
	// passed. Exhausted retries are escalated to FeedbackMemory (warning
	// channel) and removed from ScheduledMemory.
	ScheduledTick(ctx context.Context) error
}

// DefaultLearner is the production Learner implementation. All dependencies
// are injected so tests can swap in in-memory mocks.
type DefaultLearner struct {
	SkillMem     memory.Memory
	FeedbackMem  memory.Memory
	ScheduledMem *ScheduledMemory // typed (not memory.Memory) so we can call ListDue
	Reputation   ReputationStore
	Builder      *AssetBuilder
	// BayesianUpdater is the verdict → prior-evidence mutator. Injected so
	// tests can supply a stub; production passes BayesianUpdate.
	//
	// DM-20260707-001 PR-C Risk A4: signature is now (prior, verdict) →
	// (*ReputationEvidence, error). prior==nil surfaces ErrReputationStoreUnavailable
	// so callers MUST handle the cold-start path explicitly (matches
	// BayesianUpdate's new public signature).
	BayesianUpdater func(prior *ReputationEvidence, v workmodel.Verdict) (*ReputationEvidence, error)
}

// NewDefaultLearner wires the 3 memory channels + reputation store +
// builder + BayesianUpdater (defaults to BayesianUpdate if nil).
func NewDefaultLearner(
	skill, feedback memory.Memory,
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

	class := asset.ClassFromVerdictKind(req.Verdict.Kind, req.Verdict.Reason)
	if class == LearningClass(types.LearningUnknown) {
		return nil, fmt.Errorf("%w: unsupported verdict kind %v", ErrAssetBuildFailed, req.Verdict.Kind)
	}

	builtAsset, err := l.Builder.Build(ctx, req, class)
	if err != nil {
		return nil, err
	}
	if builtAsset == nil {
		return nil, ErrAssetBuildFailed
	}

	// LP-3: Bayesian update on ReputationStore.
	//
	// Order matters: update Reputation FIRST so the Bayesian evidence is
	// committed before the asset becomes visible. The previous order —
	// memory.Memory.Store then Reputation.Update — left a partial-state window
	// where a crash between Store and Update produced an asset in memory.Memory
	// without its corresponding Bayesian evidence. On the next Observe,
	// the asset was retrieved as if it had been "learned" but the
	// Reputation row reflected only the prior state, so Inject would
	// replay with stale statistics. Updating Reputation first means a
	// crash before memory.Memory.Store leaves the Reputation row one step ahead
	// of memory.Memory (over-count by one), which Inject handles correctly via
	// the BayesianUpdate mechanics — over-counting is a benign
	// statistical artifact, while under-counting (the old bug) means
	// Inject ignores a Learn that the rest of the system already
	// acknowledged. The caller retries on error; ReputationStore.Update
	// is idempotent for the same prior+verdict pair (same next state).
	if l.Reputation != nil && l.BayesianUpdater != nil {
		priorEv, err := l.Reputation.Get(ctx, req.SessionID)
		if err != nil && !errors.Is(err, ErrReputationStoreUnavailable) {
			return nil, fmt.Errorf("learn: ReputationStore.Get: %w", err)
		}
		if priorEv == nil {
			// Cold start: bootstrap with Developer track (fail-safe).
			priorEv, err = NewReputationEvidence(req.SessionID, TrackModeDeveloper)
			if err != nil {
				return nil, fmt.Errorf("learn: cold-start reputation: %w", err)
			}
		}
		next, err := l.BayesianUpdater(priorEv, req.Verdict)
		if err != nil {
			return nil, fmt.Errorf("learn: BayesianUpdater: %w", err)
		}
		if next == nil {
			return nil, fmt.Errorf("learn: BayesianUpdater returned nil evidence for prior=%+v", priorEv)
		}

		// DM-20260629-001 PR-6 t-span-coverage (T39): emit a dedicated
		// D7_LongTerm_Reputation_Update span around the BayesianUpdate so
		// LP-1 acceptance traces show both the prior and posterior Beta
		// parameters side-by-side (LP-1 closure evidence).
		endRep := hardening.EmitLongTermReputationUpdate(
			ctx,
			req.SessionID,
			float64(priorEv.Alpha), float64(priorEv.Beta),
			float64(next.Alpha), float64(next.Beta),
			next.ConfidenceLow, next.ConfidenceHigh,
			next.VerifierFailureCount,
			string(priorEv.TrackMode),
		)
		endRep(nil)
		if err := l.Reputation.Update(ctx, next); err != nil {
			return nil, fmt.Errorf("learn: ReputationStore.Update: %w", err)
		}
	}

	// LP-2: route to the correct memory.Memory channel.
	switch class {
	case LearningClass(types.LearningSOP), LearningClass(types.LearningProtocol):
		if err := l.SkillMem.Store(ctx, builtAsset); err != nil {
			return nil, fmt.Errorf("learn: SkillMemory.Store: %w", err)
		}
	case LearningClass(types.LearningKnowledge), LearningClass(types.LearningConclusion):
		if err := l.FeedbackMem.Store(ctx, builtAsset); err != nil {
			return nil, fmt.Errorf("learn: FeedbackMemory.Store: %w", err)
		}
	case LearningClass(types.LearningPending):
		// LearningPending always goes to ScheduledMemory (LP-2). It may
		// later be escalated to FeedbackMemory by ScheduledTick on
		// MaxRetries exhaustion.
		if err := l.ScheduledMem.Store(ctx, builtAsset); err != nil {
			return nil, fmt.Errorf("learn: ScheduledMemory.Store: %w", err)
		}
	default:
		return nil, ErrAssetClassMismatch
	}

	return []*LearningAsset{builtAsset}, nil
}

// Inject is the LP-1 read phase: read ReputationStore → BuildAdaptivePrior.
//
// Phase 7 PR-7.2 (D7-S13-A48-T05): trackModeHint is a per-request hint
// ("developer" / "operator" / "") that selects the prior Beta. When the
// ReputationStore has a row for sessionID, the row's TrackMode wins over
// the hint (cross-session state takes precedence over a per-request
// suggestion). Empty / unknown hints are normalized to Developer.
func (l *DefaultLearner) Inject(ctx context.Context, sessionID, trackModeHint string) (*AdaptivePrior, error) {
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
	// Phase 7 PR-7.2: track-mode resolution policy (3 cases):
	//
	// 1. rep != nil && rep.TrackMode != ""  → use rep.TrackMode (persisted state wins)
	// 2. rep == nil or rep.TrackMode == ""  → use normalized hint
	// 3. hint empty / unknown                → log warn (only when non-empty but
	//                                          unknown) + fall back to Developer
	trackMode := TrackModeDeveloper
	if rep != nil && rep.TrackMode != "" {
		trackMode = rep.TrackMode
	} else {
		switch trackModeHint {
		case "":
			// Default (zero value) → no log, use Developer.
			trackMode = TrackModeDeveloper
		case string(TrackModeOperator):
			trackMode = TrackModeOperator
		case string(TrackModeDeveloper):
			trackMode = TrackModeDeveloper
		default:
			// Unknown non-empty hint → log warn + fall back to Developer.
			slog.Warn("learn: Inject trackModeHint is unknown, falling back to Developer",
				"session_id", sessionID, "track_mode_hint", trackModeHint)
			trackMode = TrackModeDeveloper
		}
	}
	return BuildAdaptivePrior(rep, trackMode), nil
}

// ScheduledTick drains ScheduledMemory entries whose TriggerAt has passed:
//   - RetryCount < MaxRetries → increment RetryCount, push TriggerAt forward
//   - RetryCount >= MaxRetries → escalate to FeedbackMemory + delete from
//     ScheduledMemory
//
// ListDue returns DEEP COPIES of retry envelopes (see
// ScheduledMemory.ListDue), so we cannot mutate the underlying store
// in place. The re-queue path therefore: (a) Deletes the existing
// envelope, (b) builds a new PendingAssetContent with the updated
// NextRetryAt, (c) Re-Stores the asset under the same AssetKey. The
// PendingAssetContent validator caps NextRetryAt at ExpiryAt, so this
// loop respects the asset TTL contract. The escalate path is simpler —
// it Stores a new asset in FeedbackMemory (different AssetKey, prefixed
// "escalated:") and Deletes the original.
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
		// Re-queue: delete the old envelope, build a new asset with
		// NextRetryAt pushed forward by 5 minutes, re-store under the
		// same AssetKey.
		if err := l.ScheduledMem.Delete(ctx, retry.Asset.AssetKey); err != nil {
			return fmt.Errorf("learn: ScheduledMem.Delete (requeue): %w", err)
		}
		pending, ok := retry.Asset.Content.(*PendingAssetContent)
		if !ok {
			// Defensive: a LearningPending asset must have a
			// PendingAssetContent payload. If it does not (corruption /
			// future evolution), skip rather than panic — the operator
			// can investigate via logs.
			slog.Warn("learn: ScheduledTick requeue skipped (non-pending content)",
				"asset_key", retry.Asset.AssetKey, "session_id", retry.Asset.SessionID)
			continue
		}
		pending.RetryAttempts = retry.RetryCount + 1
		pending.NextRetryAt = now.Add(5 * time.Minute)
		retry.Asset.Content = pending
		retry.Asset.LastUsedAt = now
		if err := l.ScheduledMem.Store(ctx, retry.Asset); err != nil {
			return fmt.Errorf("learn: ScheduledMem.Store (requeue): %w", err)
		}
	}
	return nil
}
