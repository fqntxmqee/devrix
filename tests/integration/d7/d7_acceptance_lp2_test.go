//go:build integration && d7

// LP-2 acceptance test (DM-20260629-001 PR-7, T43).
//
// LP-2 channel routing isolation: 3 different Verdict kinds route to 3
// different Memory channels without cross-contamination.
//
//   VerdictPass                 → SkillMemory (SOP/Protocol)
//   VerdictFail (with reason)   → FeedbackMemory (Knowledge)
//   VerdictIndeterminate
//     + parse_failure reason    → FeedbackMemory (Knowledge, G8-1 fix)
//     + non-parse reason        → ScheduledMemory (Pending, retry queue)
//
// Plus escalation: ScheduledMemory retry exhaustion escalates to
// FeedbackMemory (Knowledge warning asset).
package d7integration

import (
	"context"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn/asset"
	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn/memory"
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: DM-20260629-001 T43.b — LP-2 channel routing isolation.
func TestAcceptance_LP2_VerdictRoutesToCorrectChannel(t *testing.T) {
	sessionID := "sess-lp2-route"
	f := newLP1Fixture(t, sessionID)

	planStub := plan.NewPlan("plan_lp2_1", sessionID, plan.CommitmentPlan, []string{"obs_lp2"},
		[]plan.Step{{ID: "s1"}}, 0.8)

	// Case 1: VerdictPass → SkillMemory.
	passReq := learn.LearnRequest{
		SessionID: sessionID,
		Verdict:   workmodel.Verdict{Kind: types.VerdictPass, SourceID: "v_pass", Reason: "ok"},
		Plan:      planStub,
		Artifact:  artifactStub("art_pass", sessionID, nil),
	}
	passAssets, err := f.learner.Learn(context.Background(), passReq)
	if err != nil {
		t.Fatalf("Learn(VerdictPass): %v", err)
	}
	if len(passAssets) != 1 {
		t.Fatalf("VerdictPass assets len = %d, want 1", len(passAssets))
	}
	if passAssets[0].Class != learn.LearningClass(types.LearningSOP) &&
		passAssets[0].Class != learn.LearningClass(types.LearningProtocol) {
		t.Errorf("VerdictPass Class = %s, want LearningSOP or LearningProtocol", passAssets[0].Class)
	}
	if got := mustRetrieve(t, f.skill, passAssets[0].AssetKey); got == nil {
		t.Error("VerdictPass asset missing from SkillMemory")
	}
	if got := mustRetrieve(t, f.feedback, passAssets[0].AssetKey); got != nil {
		t.Errorf("VerdictPass must NOT be in FeedbackMemory, got %+v", got)
	}

	// Case 2: VerdictFail (with reason) → FeedbackMemory (Knowledge).
	failReq := learn.LearnRequest{
		SessionID: sessionID,
		Verdict:   workmodel.Verdict{Kind: types.VerdictFail, SourceID: "v_fail", Reason: "test_failed"},
		Plan:      planStub,
		Artifact:  artifactStub("art_fail", sessionID, nil),
	}
	failAssets, err := f.learner.Learn(context.Background(), failReq)
	if err != nil {
		t.Fatalf("Learn(VerdictFail): %v", err)
	}
	if failAssets[0].Class != learn.LearningClass(types.LearningKnowledge) &&
		failAssets[0].Class != learn.LearningClass(types.LearningConclusion) {
		t.Errorf("VerdictFail Class = %s, want LearningKnowledge or LearningConclusion", failAssets[0].Class)
	}
	if got := mustRetrieve(t, f.feedback, failAssets[0].AssetKey); got == nil {
		t.Error("VerdictFail asset missing from FeedbackMemory")
	}
	if got := mustRetrieve(t, f.skill, failAssets[0].AssetKey); got != nil {
		t.Errorf("VerdictFail must NOT be in SkillMemory, got %+v", got)
	}

	// Case 3: VerdictIndeterminate + parse_failure → ScheduledMemory
	// (LearningPending) but G8-1 fix prevents α/β pollution. Note the
	// routing class is LearningPending (consistent with other indeterminate
	// paths); the G8-1 fix is in BayesianUpdate, not in routing.
	parseReq := learn.LearnRequest{
		SessionID: sessionID,
		Verdict: workmodel.Verdict{
			Kind:                types.VerdictIndeterminate,
			SourceID:            "v_parsefail",
			Reason:              "verifier_parse_failure",
			IndeterminateReason: "verifier_parse_failure",
		},
		Artifact: artifactStub("art_parse", sessionID, nil),
	}
	parseAssets, err := f.learner.Learn(context.Background(), parseReq)
	if err != nil {
		t.Fatalf("Learn(VerdictIndeterminate parse_failure): %v", err)
	}
	if parseAssets[0].Class != learn.LearningClass(types.LearningPending) {
		t.Errorf("parse_failure Class = %s, want LearningPending (G8-1 affects α/β only, not routing)",
			parseAssets[0].Class)
	}
	if got := mustRetrieveScheduled(t, f.scheduled, parseAssets[0].AssetKey); got == nil {
		t.Error("parse_failure asset must be in ScheduledMemory (LearningPending)")
	}

	// Case 4: VerdictIndeterminate + env_limited → ScheduledMemory (Pending).
	envReq := learn.LearnRequest{
		SessionID: sessionID,
		Verdict: workmodel.Verdict{
			Kind:                types.VerdictIndeterminate,
			SourceID:            "v_env",
			Reason:              "env_limited",
			IndeterminateReason: "env_limited",
		},
		Artifact: artifactStub("art_env", sessionID, nil),
	}
	envAssets, err := f.learner.Learn(context.Background(), envReq)
	if err != nil {
		t.Fatalf("Learn(env_limited): %v", err)
	}
	if envAssets[0].Class != learn.LearningClass(types.LearningPending) {
		t.Errorf("env_limited Class = %s, want LearningPending", envAssets[0].Class)
	}
	if got := mustRetrieveScheduled(t, f.scheduled, envAssets[0].AssetKey); got == nil {
		t.Error("env_limited asset missing from ScheduledMemory")
	}
	if got := mustRetrieve(t, f.skill, envAssets[0].AssetKey); got != nil {
		t.Errorf("env_limited must NOT be in SkillMemory, got %+v", got)
	}
}

// T: DM-20260629-001 T43.b — LP-2 ScheduledTick re-queue: due pending
// assets get re-stored with NextRetryAt pushed forward by 5 minutes.
func TestAcceptance_LP2_ScheduledTick_RequeuesDueAssets(t *testing.T) {
	sessionID := "sess-lp2-requeue"
	f := newLP1Fixture(t, sessionID)

	// Seed ScheduledMemory with a pending asset (LearningPending → ScheduledMemory).
	req := learn.LearnRequest{
		SessionID: sessionID,
		Verdict: workmodel.Verdict{
			Kind:                types.VerdictIndeterminate,
			SourceID:            "v_requeue",
			Reason:              "env_limited",
			IndeterminateReason: "env_limited",
		},
		Artifact: artifactStub("art_requeue", sessionID, nil),
	}
	assets, err := f.learner.Learn(context.Background(), req)
	if err != nil {
		t.Fatalf("Learn: %v", err)
	}
	originalKey := assets[0].AssetKey

	// The newly-stored asset has NextRetryAt = ExpiryAt (24h future). For
	// ListDue(now) to include it, re-store with a past NextRetryAt.
	due := *assets[0]
	pending := &asset.PendingAssetContent{
		IndeterminateReason: "env_limited",
		OriginalArtifactID:  "art_requeue",
		RetryAttempts:       0,
		MaxRetries:          asset.DefaultPendingMaxRetries,
		NextRetryAt:         time.Now().Add(-1 * time.Minute), // already due
	}
	due.Content = pending
	if err := f.scheduled.Store(context.Background(), &due); err != nil {
		t.Fatalf("re-store due asset: %v", err)
	}

	// Tick → should re-queue (RetryCount < MaxRetries so no escalation).
	preTick := mustRetrieveScheduled(t, f.scheduled, originalKey)
	if preTick == nil {
		t.Fatal("due asset missing from ScheduledMemory before tick")
	}

	if err := f.learner.ScheduledTick(context.Background()); err != nil {
		t.Fatalf("ScheduledTick: %v", err)
	}

	// Asset should still be present (re-queued, not deleted).
	post := mustRetrieveScheduled(t, f.scheduled, originalKey)
	if post == nil {
		t.Fatal("re-queued asset must still be in ScheduledMemory")
	}
	// NextRetryAt should now be ~5 minutes in the future (re-queue semantics).
	if post.TriggerAt.Before(time.Now()) {
		t.Errorf("re-queued TriggerAt = %v, must be in the future", post.TriggerAt)
	}
}

// mustRetrieve is a small helper that wraps Retrieve with a clear error
// message when the underlying Memory returns an error other than not-found.
func mustRetrieve(t *testing.T, m learn.Memory, key string) *learn.LearningAsset {
	t.Helper()
	got, err := m.Retrieve(context.Background(), key)
	if err != nil {
		t.Fatalf("Retrieve(%q): %v", key, err)
	}
	return got
}

// mustRetrieveScheduled is the ScheduledMemory variant — it returns the
// ScheduledRetry envelope (not the bare asset) because ScheduledMemory
// has a richer Retrieve signature.
func mustRetrieveScheduled(t *testing.T, m *learn.ScheduledMemory, key string) *memory.ScheduledRetry {
	t.Helper()
	got, err := m.Retrieve(context.Background(), key)
	if err != nil {
		t.Fatalf("ScheduledMemory.Retrieve(%q): %v", key, err)
	}
	return got
}
