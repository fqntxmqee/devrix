package learn

import (
	"context"
	"errors"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// ─────────────────────────────────────────────────────────────────────────
// Test helpers
// ─────────────────────────────────────────────────────────────────────────

func newTestLearner(t *testing.T) (*DefaultLearner, *InMemoryReputationStore) {
	t.Helper()
	skill := NewSkillMemory()
	feedback := NewFeedbackMemory()
	scheduled := NewScheduledMemory()
	rep := NewInMemoryReputationStore()
	builder := NewAssetBuilder()
	return NewDefaultLearner(skill, feedback, scheduled, rep, builder), rep
}

// ─────────────────────────────────────────────────────────────────────────
// DefaultLearner interface compliance
// ─────────────────────────────────────────────────────────────────────────

func TestDefaultLearner_ImplementsLearner(t *testing.T) {
	var _ Learner = (*DefaultLearner)(nil)
}

// ─────────────────────────────────────────────────────────────────────────
// Learn: Verdict → LearningAsset routing (4 verdict kinds)
// ─────────────────────────────────────────────────────────────────────────

func TestLearner_Learn_VerdictPass_RoutesToSkillMemory(t *testing.T) {
	l, _ := newTestLearner(t)
	ctx := context.Background()

	p := plan.NewPlan("plan_1", "sess_1", plan.CommitmentPlan, []string{"obs_1"}, []plan.Step{{ID: "step_1"}}, 0.8)
	req := LearnRequest{
		SessionID: "sess_1",
		Verdict:   workmodel.Verdict{Kind: types.VerdictPass, SourceID: "v_pass", Reason: "ok"},
		Plan:      p,
		Artifact:  waveschedulerArtifactStub("art_1", "sess_1", []string{"foo.go"}),
	}
	assets, err := l.Learn(ctx, req)
	if err != nil {
		t.Fatalf("Learn: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("assets len = %d, want 1", len(assets))
	}
	if assets[0].Class != LearningClass(types.LearningSOP) {
		t.Errorf("Class = %s, want LearningSOP", assets[0].Class)
	}
	got, _ := l.SkillMem.Retrieve(ctx, assets[0].AssetKey)
	if got == nil {
		t.Error("SkillMemory.Retrieve should find the stored SOP asset")
	}
}

func TestLearner_Learn_VerdictPartial_RoutesToSkillMemory(t *testing.T) {
	l, _ := newTestLearner(t)
	ctx := context.Background()

	p := plan.NewPlan("plan_1", "sess_1", plan.ProtocolPlan, []string{"obs_1"}, []plan.Step{{ID: "step_1"}}, 0.7)
	req := LearnRequest{
		SessionID: "sess_1",
		Verdict:   workmodel.Verdict{Kind: types.VerdictPartial, SourceID: "v_partial", Reason: "partial"},
		Plan:      p,
	}
	assets, err := l.Learn(ctx, req)
	if err != nil {
		t.Fatalf("Learn: %v", err)
	}
	if assets[0].Class != LearningClass(types.LearningProtocol) {
		t.Errorf("Class = %s, want LearningProtocol", assets[0].Class)
	}
}

func TestLearner_Learn_VerdictFail_RoutesToFeedbackMemory(t *testing.T) {
	l, _ := newTestLearner(t)
	ctx := context.Background()

	req := LearnRequest{
		SessionID: "sess_1",
		Verdict:   workmodel.Verdict{Kind: types.VerdictFail, SourceID: "v_fail", Reason: "root cause: timeout"},
		Artifact:  waveschedulerArtifactStub("art_1", "sess_1", nil),
	}
	assets, err := l.Learn(ctx, req)
	if err != nil {
		t.Fatalf("Learn: %v", err)
	}
	if assets[0].Class != LearningClass(types.LearningKnowledge) {
		t.Errorf("Class = %s, want LearningKnowledge", assets[0].Class)
	}
}

func TestLearner_Learn_VerdictIndeterminate_RoutesToScheduledMemory(t *testing.T) {
	l, _ := newTestLearner(t)
	ctx := context.Background()

	req := LearnRequest{
		SessionID: "sess_1",
		Verdict: workmodel.Verdict{
			Kind:               types.VerdictIndeterminate,
			SourceID:           "v_indet",
			IndeterminateReason: "env_limited",
		},
		Artifact: waveschedulerArtifactStub("art_orig", "sess_1", nil),
	}
	assets, err := l.Learn(ctx, req)
	if err != nil {
		t.Fatalf("Learn: %v", err)
	}
	if assets[0].Class != LearningClass(types.LearningPending) {
		t.Errorf("Class = %s, want LearningPending", assets[0].Class)
	}
	got, _ := l.ScheduledMem.Retrieve(ctx, assets[0].AssetKey)
	if got == nil {
		t.Error("ScheduledMemory.Retrieve should find the stored Pending asset")
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Learn: Bayesian update on ReputationStore
// ─────────────────────────────────────────────────────────────────────────

func TestLearner_Learn_BayesianUpdate(t *testing.T) {
	l, rep := newTestLearner(t)
	ctx := context.Background()

	// VerdictPass → LearningSOP, needs Plan or Artifact for builder to succeed.
	p := plan.NewPlan("plan_1", "sess_1", plan.CommitmentPlan, []string{"obs_1"}, []plan.Step{{ID: "step_1"}}, 0.8)
	req := LearnRequest{
		SessionID: "sess_1",
		Verdict:   workmodel.Verdict{Kind: types.VerdictPass, SourceID: "v_1"},
		Plan:      p,
	}
	if _, err := l.Learn(ctx, req); err != nil {
		t.Fatalf("Learn #1: %v", err)
	}
	r, _ := rep.Get(ctx, "sess_1")
	if r == nil {
		t.Fatal("ReputationStore should have row after Learn")
	}
	if r.Alpha != 1 || r.UpdateCount != 1 {
		t.Errorf("After 1 Pass: Alpha=%d UpdateCount=%d, want 1/1", r.Alpha, r.UpdateCount)
	}
}

func TestLearner_Learn_ColdStart_DefaultTrackMode(t *testing.T) {
	l, rep := newTestLearner(t)
	ctx := context.Background()

	// VerdictFail → LearningKnowledge, needs Reason to build Topic/Hypothesis.
	req := LearnRequest{
		SessionID: "cold_session",
		Verdict:   workmodel.Verdict{Kind: types.VerdictFail, SourceID: "v_1", Reason: "timeout"},
	}
	if _, err := l.Learn(ctx, req); err != nil {
		t.Fatalf("Learn: %v", err)
	}
	r, _ := rep.Get(ctx, "cold_session")
	if r.TrackMode != TrackModeDeveloper {
		t.Errorf("cold start TrackMode = %s, want Developer", r.TrackMode)
	}
	if r.Beta != 1 {
		t.Errorf("cold start Beta = %d, want 1", r.Beta)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Inject: read AdaptivePrior
// ─────────────────────────────────────────────────────────────────────────

func TestLearner_Inject_ColdStart_DefaultDeveloperPrior(t *testing.T) {
	l, _ := newTestLearner(t)
	ctx := context.Background()

	prior, err := l.Inject(ctx, "fresh_session", "")
	if err != nil {
		t.Fatalf("Inject: %v", err)
	}
	if prior.PriorBeta != DefaultDeveloperPrior {
		t.Errorf("cold start PriorBeta = %+v, want %+v", prior.PriorBeta, DefaultDeveloperPrior)
	}
	if prior.Reputation != nil {
		t.Error("cold start Reputation should be nil")
	}
}

func TestLearner_Inject_BuildsAdaptivePrior(t *testing.T) {
	l, rep := newTestLearner(t)
	ctx := context.Background()

	// First seed the reputation store.
	rep0, _ := NewReputationEvidence("sess_1", TrackModeOperator)
	rep0.Alpha = 3
	rep0.Beta = 1
	if err := rep.Update(ctx, rep0); err != nil {
		t.Fatalf("Update: %v", err)
	}

	prior, err := l.Inject(ctx, "sess_1", "")
	if err != nil {
		t.Fatalf("Inject: %v", err)
	}
	// Operator Beta(8,1) + rep(3,1) = Beta(11, 2)
	want := BetaPrior{Alpha: 11, Beta: 2}
	if prior.PriorBeta != want {
		t.Errorf("PriorBeta = %+v, want %+v", prior.PriorBeta, want)
	}
}

func TestLearner_Inject_EmptySession_FailFast(t *testing.T) {
	l, _ := newTestLearner(t)
	_, err := l.Inject(context.Background(), "", "")
	if !errors.Is(err, ErrAdaptivePriorNotReady) {
		t.Errorf("Inject('') err = %v, want ErrAdaptivePriorNotReady", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// ScheduledTick: retry + exhaustion escalation
// ─────────────────────────────────────────────────────────────────────────

func TestLearner_ScheduledTick_NoDue_NoOp(t *testing.T) {
	l, _ := newTestLearner(t)
	ctx := context.Background()
	req := LearnRequest{
		SessionID: "sess_1",
		Verdict: workmodel.Verdict{
			Kind:                types.VerdictIndeterminate,
			SourceID:            "v_1",
			IndeterminateReason: "env_limited",
		},
		Artifact: waveschedulerArtifactStub("art_1", "sess_1", nil),
	}
	if _, err := l.Learn(ctx, req); err != nil {
		t.Fatalf("Learn: %v", err)
	}
	// TriggerAt = asset.ExpiryAt (default now+24h) — should not be due.
	if err := l.ScheduledTick(ctx); err != nil {
		t.Fatalf("ScheduledTick: %v", err)
	}
	retry, _ := l.ScheduledMem.Retrieve(ctx, "pending:" /* placeholder */)
	_ = retry
	// Count: pending asset should still be there.
	list, _ := l.ScheduledMem.List(ctx, MemoryFilter{})
	if len(list) != 1 {
		t.Errorf("expected 1 pending asset, got %d", len(list))
	}
}

func TestLearner_ScheduledTick_ExhaustedMaxRetries_Escalate(t *testing.T) {
	l, _ := newTestLearner(t)
	ctx := context.Background()

	// Create a pending asset and force-exhaust its retries.
	req := LearnRequest{
		SessionID: "sess_1",
		Verdict: workmodel.Verdict{
			Kind:                types.VerdictIndeterminate,
			SourceID:            "v_1",
			IndeterminateReason: "env_limited",
		},
		Artifact: waveschedulerArtifactStub("art_1", "sess_1", nil),
	}
	assets, err := l.Learn(ctx, req)
	if err != nil {
		t.Fatalf("Learn: %v", err)
	}
	assetKey := assets[0].AssetKey

	// Force the retry to be exhausted.
	retry, _ := l.ScheduledMem.Retrieve(ctx, assetKey)
	if retry == nil {
		t.Fatal("ScheduledMemory should have the retry envelope")
	}
	retry.RetryCount = retry.MaxRetries
	retry.TriggerAt = timePast() // make it due
	l.ScheduledMem.mu.Lock()
	l.ScheduledMem.store[assetKey] = retry
	l.ScheduledMem.mu.Unlock()

	if err := l.ScheduledTick(ctx); err != nil {
		t.Fatalf("ScheduledTick: %v", err)
	}

	// Pending should be deleted from ScheduledMemory.
	if got, _ := l.ScheduledMem.Retrieve(ctx, assetKey); got != nil {
		t.Error("exhausted retry should be deleted from ScheduledMemory")
	}
	// Should be escalated to FeedbackMemory.
	feedback, _ := l.FeedbackMem.List(ctx, MemoryFilter{})
	if len(feedback) != 1 {
		t.Errorf("expected 1 feedback asset after escalation, got %d", len(feedback))
	} else {
		if feedback[0].Class != LearningClass(types.LearningKnowledge) {
			t.Errorf("escalated asset Class = %s, want LearningKnowledge", feedback[0].Class)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────
// LP-1 closed-loop: Learn → ReputationStore updates → Inject returns merged prior
// ─────────────────────────────────────────────────────────────────────────

func TestLP1_ClosedLoop_LearnThenInject(t *testing.T) {
	l, rep := newTestLearner(t)
	ctx := context.Background()

	// Step 1: Process 3 PASS verdicts (each with Plan so SOP builder succeeds).
	p := plan.NewPlan("plan_1", "sess_1", plan.CommitmentPlan, []string{"obs_1"}, []plan.Step{{ID: "step_1"}}, 0.8)
	for i := 0; i < 3; i++ {
		req := LearnRequest{
			SessionID: "sess_1",
			Verdict:   workmodel.Verdict{Kind: types.VerdictPass, SourceID: "v_pass"},
			Plan:      p,
		}
		if _, err := l.Learn(ctx, req); err != nil {
			t.Fatalf("Learn #%d: %v", i, err)
		}
	}
	// Step 2: Inject reads the updated ReputationStore → builds AdaptivePrior.
	prior, err := l.Inject(ctx, "sess_1", "")
	if err != nil {
		t.Fatalf("Inject: %v", err)
	}
	// After 3 PASS, Alpha=3 Beta=0; merged with Developer Beta(5,3) = Beta(8,3).
	want := BetaPrior{Alpha: 8, Beta: 3}
	if prior.PriorBeta != want {
		t.Errorf("LP-1 closed-loop PriorBeta = %+v, want %+v", prior.PriorBeta, want)
	}
	// Step 3: ReputationStore.Get confirms the prior.
	r, _ := rep.Get(ctx, "sess_1")
	if r.Alpha != 3 {
		t.Errorf("ReputationStore.Alpha = %d, want 3", r.Alpha)
	}
}

func TestLP1_ClosedLoop_INDETERMINATE_DoesNotPolluteAlphaBeta(t *testing.T) {
	l, rep := newTestLearner(t)
	ctx := context.Background()

	// 1 PASS (with Plan), then 1 INDETERMINATE.
	p := plan.NewPlan("plan_1", "sess_1", plan.CommitmentPlan, []string{"obs_1"}, []plan.Step{{ID: "step_1"}}, 0.8)
	pass := LearnRequest{
		SessionID: "sess_1",
		Verdict:   workmodel.Verdict{Kind: types.VerdictPass, SourceID: "v_pass"},
		Plan:      p,
	}
	if _, err := l.Learn(ctx, pass); err != nil {
		t.Fatalf("Learn pass: %v", err)
	}
	indet := LearnRequest{
		SessionID: "sess_1",
		Verdict: workmodel.Verdict{
			Kind:                types.VerdictIndeterminate,
			SourceID:            "v_indet",
			IndeterminateReason: "verifier_parse_failure",
		},
		Artifact: waveschedulerArtifactStub("art_orig", "sess_1", nil),
	}
	if _, err := l.Learn(ctx, indet); err != nil {
		t.Fatalf("Learn indet: %v", err)
	}
	r, _ := rep.Get(ctx, "sess_1")
	// ⭐G8-1 fix: INDETERMINATE verifier_parse_failure MUST NOT touch α/β.
	if r.Alpha != 1 || r.Beta != 0 {
		t.Errorf("LP-1 G8-1: Alpha=%d Beta=%d, want 1/0 (INDETERMINATE should not pollute)", r.Alpha, r.Beta)
	}
	if r.VerifierFailureCount != 1 {
		t.Errorf("VerifierFailureCount = %d, want 1", r.VerifierFailureCount)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Failure modes
// ─────────────────────────────────────────────────────────────────────────

func TestLearner_Learn_EmptySessionID_FailFast(t *testing.T) {
	l, _ := newTestLearner(t)
	_, err := l.Learn(context.Background(), LearnRequest{})
	if !errors.Is(err, ErrAssetIncomplete) {
		t.Errorf("Learn(empty) err = %v, want ErrAssetIncomplete", err)
	}
}

func TestLearner_Learn_UnsupportedVerdictKind_FailFast(t *testing.T) {
	l, _ := newTestLearner(t)
	req := LearnRequest{
		SessionID: "sess_1",
		Verdict:   workmodel.Verdict{Kind: types.VerdictKind(99)},
	}
	_, err := l.Learn(context.Background(), req)
	if err == nil {
		t.Error("unsupported VerdictKind should fail-fast")
	}
	if !errors.Is(err, ErrAssetBuildFailed) {
		t.Errorf("err = %v, want ErrAssetBuildFailed", err)
	}
}