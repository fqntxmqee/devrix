// Package learn: force_plan link (DM-20260707-001 PR-E, T63).
//
// force_plan is the reputation-driven Plan-bypass signal: when the post-
// BayesianUpdate β/(α+β) ratio crosses the configured threshold (default 0.7),
// the next Observe call MUST skip the observational fast-path and run the full
// Plan / Execute / Verify pipeline.
//
// This file implements the bridge between BayesianUpdate (which mutates α/β)
// and the next Observe call (which reads force_plan and decides whether to
// invoke the fast-return gate).
//
// Wiring:
//
//	1. DefaultLearner.Learn (after BayesianUpdate success)
//	    → calls BayesianUpdateWithPolicy
//	    → returns the post-update ReputationEvidence + a ForcePlanSignal
//	2. AsyncLearner / ItemPipelineRunner persists ForcePlanSignal
//	    → ReputationStore.Update (evidence) + ReputationStore.SetMetadata
//	      (force_plan flag + threshold + ratio + alpha + beta)
//	3. Next round's Observe path (item_pipeline.go's maybeObservationalAnswer
//	    gate from DM-20260706-011) reads ReputationStore.GetMetadata
//	    → if force_plan=true, gate skipped (always Plan).
//
// Why a separate type (vs. adding force_plan to ReputationEvidence directly):
// ReputationEvidence is the Bayesian state; force_plan is a transient
// routing signal. Keeping them separate avoids polluting the reputation
// record with non-statistical fields (the dashboard would otherwise need
// to filter them out on every read).
package learn

import (
	"fmt"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn/reputation"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

// ForcePlanSignal is the metadata payload the next Observe call reads to
// decide whether to bypass the observational fast-path. The struct is the
// value type — persistence (in-memory map, Redis, etc.) is the caller's job.
type ForcePlanSignal struct {
	// Triggered is true when β/(α+β) > forcePlanThreshold at the time
	// this signal was computed.
	Triggered bool `json:"triggered"`

	// BetaRatio is the post-BayesianUpdate β/(α+β) value (0 if total=0).
	BetaRatio float64 `json:"beta_ratio"`

	// Alpha / Beta at the time of computation (for dashboards).
	Alpha int `json:"alpha"`
	Beta  int `json:"beta"`

	// Reason is the human-readable reason this signal was emitted
	// ("force_plan_threshold_crossed" / "below_threshold" / "cold_start").
	Reason string `json:"reason"`

	// ComputedAt is the timestamp the signal was computed.
	ComputedAt time.Time `json:"computed_at"`

	// SessionID is the session this signal applies to.
	SessionID string `json:"session_id"`
}

// String renders the signal for logging / dashboard.
func (s ForcePlanSignal) String() string {
	return fmt.Sprintf("force_plan=%t ratio=%.2f alpha=%d beta=%d reason=%s",
		s.Triggered, s.BetaRatio, s.Alpha, s.Beta, s.Reason)
}

// BayesianUpdateWithPolicy wraps BayesianUpdate with force_plan detection.
// Returns:
//
//   - the post-update ReputationEvidence (or nil if BayesianUpdate failed;
//     caller should treat that as a cold-start signal and not emit force_plan)
//   - the ForcePlanSignal carrying the post-update β/(α+β) classification
//   - any error from BayesianUpdate (caller decides whether to abort the
//     Learn or log + continue)
//
// This is the only entry point DefaultLearner should use for Bayesian
// updates; the bare reputation.BayesianUpdate is preserved as a re-export
// for callers that don't care about force_plan (e.g. tests).
func BayesianUpdateWithPolicy(
	sessionID string,
	prior *reputation.ReputationEvidence,
	verdict workmodel.Verdict,
) (*reputation.ReputationEvidence, *ForcePlanSignal, error) {
	next, err := reputation.BayesianUpdate(prior, verdict)
	if err != nil {
		// Cold-start / store unavailable: no force_plan to emit.
		return nil, &ForcePlanSignal{
			Triggered:  false,
			Reason:     "cold_start_or_store_unavailable",
			ComputedAt: time.Now(),
			SessionID:  sessionID,
		}, err
	}

	signal := computeForcePlanSignal(sessionID, next.Alpha, next.Beta)
	return next, signal, nil
}

// computeForcePlanSignal derives a ForcePlanSignal from the post-update α/β.
// Pure function (no side effects) — exposed at package scope for direct test
// access. The threshold is the same forcePlanThreshold const used by
// ClassifyScenarioWithReputation (single source of truth).
func computeForcePlanSignal(sessionID string, alpha, beta int) *ForcePlanSignal {
	total := alpha + beta
	signal := &ForcePlanSignal{
		Alpha:     alpha,
		Beta:      beta,
		ComputedAt: time.Now(),
		SessionID: sessionID,
	}
	if total == 0 {
		signal.Triggered = false
		signal.Reason = "cold_start"
		signal.BetaRatio = 0
		return signal
	}
	ratio := float64(beta) / float64(total)
	signal.BetaRatio = ratio
	if ratio > forcePlanThreshold {
		signal.Triggered = true
		signal.Reason = "force_plan_threshold_crossed"
	} else {
		signal.Triggered = false
		signal.Reason = "below_threshold"
	}
	return signal
}

// EmitForcePlanMetadata is the helper that converts a ForcePlanSignal into
// the metadata map shape the Observe gate (DM-20260706-011) reads. The
// caller persists the metadata alongside ReputationStore.Update.
//
// Returns nil when the signal is not triggered (no metadata written —
// Observe reads the absence of "force_plan=true" as a no-op).
//
// Cross-package contract (PR-F T71): the field names emitted here MUST match
// the keys in plan.ForcePlanMetaKey / plan.ForcePlanMetaRatioKey / etc.
// The contract is enforced by
// mups/learn/force_plan_integration_test.go (TestEmitMetadataContract_*).
func EmitForcePlanMetadata(signal *ForcePlanSignal) map[string]string {
	if signal == nil || !signal.Triggered {
		return nil
	}
	return map[string]string{
		"force_plan":             "true",
		"force_plan_ratio":       fmt.Sprintf("%.2f", signal.BetaRatio),
		"force_plan_alpha":       fmt.Sprintf("%d", signal.Alpha),
		"force_plan_beta":        fmt.Sprintf("%d", signal.Beta),
		"force_plan_reason":      signal.Reason,
		"force_plan_computed_at": signal.ComputedAt.Format(time.RFC3339Nano),
		"force_plan_session_id":  signal.SessionID,
	}
}

// ShouldForcePlan is the convenience read-side helper the Observe gate uses.
// Given the persisted metadata map (from EmitForcePlanMetadata), it returns
// true when force_plan=true. Returns false on nil/empty metadata.
func ShouldForcePlan(metadata map[string]string) bool {
	if metadata == nil {
		return false
	}
	return metadata["force_plan"] == "true"
}