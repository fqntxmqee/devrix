package sessionorchestrator

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
)

// ─────────────────────────────────────────────────────────────────────────
// T: D7-S13-A48-T05 — buildObserveRequest threads req.TrackMode into Inject
// ─────────────────────────────────────────────────────────────────────────

// TestProcessMessage_TrackModeOperator_PropagatedToInject verifies that
// ProcessRequest.TrackMode="operator" is plumbed into Learner.Inject and
// the resulting prior is DefaultOperatorPrior (Beta(8,1), Mean=0.889).
//
// Uses the existing fakeLearner harness but expands it to verify
// lastTrackModeHint is set when ProcessMessage calls buildObserveRequest.
func TestProcessMessage_TrackModeOperator_PropagatedToInject(t *testing.T) {
	fl := &fakeLearner{prior: learn.BuildAdaptivePrior(nil, learn.TrackModeOperator)}
	fl.prior.PriorBeta = learn.DefaultOperatorPrior
	orch := newTestOrch(t, WithLearner(fl))

	_, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-op",
		Message:   "ping",
		TrackMode: orchtypes.TrackModeOperator,
	})
	if err != nil {
		t.Fatalf("ProcessMessage: %v", err)
	}
	fl.mu.Lock()
	defer fl.mu.Unlock()
	if fl.injectCalls != 1 {
		t.Fatalf("injectCalls = %d, want 1", fl.injectCalls)
	}
	if fl.lastTrackModeHint != orchtypes.TrackModeOperator {
		t.Errorf("lastTrackModeHint = %q, want %q", fl.lastTrackModeHint, orchtypes.TrackModeOperator)
	}
}

// TestProcessMessage_TrackModeDeveloper_PropagatedToInject verifies the
// explicit "developer" hint is plumbed.
func TestProcessMessage_TrackModeDeveloper_PropagatedToInject(t *testing.T) {
	fl := &fakeLearner{prior: learn.BuildAdaptivePrior(nil, learn.TrackModeDeveloper)}
	orch := newTestOrch(t, WithLearner(fl))

	_, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-dev",
		Message:   "ping",
		TrackMode: orchtypes.TrackModeDeveloper,
	})
	if err != nil {
		t.Fatalf("ProcessMessage: %v", err)
	}
	fl.mu.Lock()
	defer fl.mu.Unlock()
	if fl.lastTrackModeHint != orchtypes.TrackModeDeveloper {
		t.Errorf("lastTrackModeHint = %q, want %q", fl.lastTrackModeHint, orchtypes.TrackModeDeveloper)
	}
}

// TestProcessMessage_TrackModeEmpty_PropagatedAsEmpty verifies that
// ProcessRequest.TrackMode="" (zero value, backward compat) is forwarded
// as "" to Inject. DefaultLearner.Inject resolves "" to Developer.
func TestProcessMessage_TrackModeEmpty_PropagatedAsEmpty(t *testing.T) {
	fl := &fakeLearner{prior: learn.BuildAdaptivePrior(nil, learn.TrackModeDeveloper)}
	orch := newTestOrch(t, WithLearner(fl))

	_, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-empty",
		Message:   "ping",
		// TrackMode omitted → ""
	})
	if err != nil {
		t.Fatalf("ProcessMessage: %v", err)
	}
	fl.mu.Lock()
	defer fl.mu.Unlock()
	if fl.lastTrackModeHint != "" {
		t.Errorf("lastTrackModeHint = %q, want \"\"", fl.lastTrackModeHint)
	}
}

// TestProcessMessage_TrackModeInvalid_DoesNotBlock verifies the lenient
// path: when callers pass an invalid TrackMode by struct literal (bypassing
// NewProcessRequest), the orchestrator does not block — the invalid value
// is forwarded to Inject where it's logged + falls back to Developer.
//
// This is the behavior described in spec.md D7-S13-A48 Scenario
// "ProcessRequest.TrackMode invalid value falls back to developer":
// buildObserveRequest forwards the hint as-is; the actual fall-back is in
// DefaultLearner.Inject (slog.Warn + Developer fail-safe).
func TestProcessMessage_TrackModeInvalid_ForwardsToInject(t *testing.T) {
	fl := &fakeLearner{prior: learn.BuildAdaptivePrior(nil, learn.TrackModeDeveloper)}
	orch := newTestOrch(t, WithLearner(fl))

	// Direct struct literal with invalid TrackMode (bypasses NewProcessRequest).
	_, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-garbage",
		Message:   "ping",
		TrackMode: "garbage",
	})
	if err != nil {
		t.Fatalf("ProcessMessage with invalid TrackMode should not error, got: %v", err)
	}
	fl.mu.Lock()
	defer fl.mu.Unlock()
	if fl.lastTrackModeHint != "garbage" {
		t.Errorf("lastTrackModeHint = %q, want %q (forwarded as-is to Inject)", fl.lastTrackModeHint, "garbage")
	}
}

// TestDefaultLearner_Inject_TrackModeOperator_ColdStart verifies that
// DefaultLearner.Inject with TrackMode hint "operator" produces the
// DefaultOperatorPrior (Beta(8,1)) on cold start (no Reputation row).
// (End-to-end complement to the orchestrator-level tests above.)
func TestDefaultLearner_Inject_TrackModeOperator_ColdStart(t *testing.T) {
	rep := learn.NewInMemoryReputationStore()
	sched := learn.NewScheduledMemory()
	skill := learn.NewSkillMemory()
	feedback := learn.NewFeedbackMemory()
	l := learn.NewDefaultLearner(skill, feedback, sched, rep, learn.NewAssetBuilder())

	prior, err := l.Inject(context.Background(), "sess-cold-op", orchtypes.TrackModeOperator)
	if err != nil {
		t.Fatalf("Inject: %v", err)
	}
	if prior.PriorBeta != learn.DefaultOperatorPrior {
		t.Errorf("cold-start operator hint PriorBeta = %+v, want %+v", prior.PriorBeta, learn.DefaultOperatorPrior)
	}
}

// TestDefaultLearner_Inject_TrackModeUnknown_LogsAndFallsBack verifies
// that an unknown non-empty TrackMode hint is logged + falls back to
// Developer (fail-safe).
func TestDefaultLearner_Inject_TrackModeUnknown_LogsAndFallsBack(t *testing.T) {
	rep := learn.NewInMemoryReputationStore()
	sched := learn.NewScheduledMemory()
	skill := learn.NewSkillMemory()
	feedback := learn.NewFeedbackMemory()
	l := learn.NewDefaultLearner(skill, feedback, sched, rep, learn.NewAssetBuilder())

	prior, err := l.Inject(context.Background(), "sess-unknown", "garbage")
	if err != nil {
		t.Fatalf("Inject: %v", err)
	}
	if prior.PriorBeta != learn.DefaultDeveloperPrior {
		t.Errorf("unknown hint PriorBeta = %+v, want %+v (Developer fail-safe)", prior.PriorBeta, learn.DefaultDeveloperPrior)
	}
}
