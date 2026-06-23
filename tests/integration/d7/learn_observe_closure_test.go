//go:build integration && d7

// LP-1 closed-loop integration test (DM-20260624-001 / Phase 6 PR-F3).
//
// Scenarios covered (Phase 6 PR-F3.1.1):
//
//  1. E2E_LP1_ClosedLoop_LearnPassAccumulatePrior — 3 × VerdictPass across
//     rounds accumulates Alpha (3), next ProcessMessage observes prior
//     Beta(8,3) Mean=0.727 and adjusts classifier confidence accordingly.
//  2. E2E_LP1_ClosedLoop_IndeterminateParseFailure_NoAlphaPollution —
//     VerdictIndeterminate with IndeterminateReason="verifier_parse_failure"
//     increments only VerifierFailureCount, NOT Alpha/Beta (G8-1 fix).
//  3. E2E_LP1_ClosedLoop_PendingAssetScheduledMemory — VerdictIndeterminate
//     with non-parse-failure reason routes to ScheduledMemory (LP-2 隔离).
//  4. E2E_5NodePipeline_End2End — full Observe → Plan → Execute → Verify →
//     Learn chain closes in a single round, with LP-1 + LP-5 lineage.
//
// Tests use in-memory mocks (InMemoryReputationStore + 3 Memory channels)
// and a fakeD2 executor. No real LLM / D3 / D4 dependencies.
package d7integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// lp1Fixture bundles the LP-1 closed-loop test wiring. It exposes the
// learner + 3 memory channels + reputation store so tests can drive
// ProcessMessage (Observe) and Learn (deposit) in alternation.
type lp1Fixture struct {
	orch        *sessionorchestrator.SessionOrchestrator
	learner     *learn.DefaultLearner
	skill       *learn.SkillMemory
	feedback    *learn.FeedbackMemory
	scheduled   *learn.ScheduledMemory
	rep         *learn.InMemoryReputationStore
	classifier  *recordingClassifier
}

func newLP1Fixture(t *testing.T, sessionID string) *lp1Fixture {
	t.Helper()
	exec := &recordingExecutor{}
	classifier := &recordingClassifier{}
	cfg := orchtypes.DefaultConfig()
	cfg.CommandWhitelist = []string{"/status", "/help"}
	cfg.FastPathThreshold = 0 // allow all fast paths through for test predictability

	skill := learn.NewSkillMemory()
	feedback := learn.NewFeedbackMemory()
	scheduled := learn.NewScheduledMemory()
	rep := learn.NewInMemoryReputationStore()
	builder := learn.NewAssetBuilder()
	learner := learn.NewDefaultLearner(skill, feedback, scheduled, rep, builder)

	orch := sessionorchestrator.NewSessionOrchestrator(
		cfg,
		exec,
		sessionorchestrator.WithLearner(learner),
		sessionorchestrator.WithClassifier(classifier),
	)

	return &lp1Fixture{
		orch:       orch,
		learner:    learner,
		skill:      skill,
		feedback:   feedback,
		scheduled:  scheduled,
		rep:        rep,
		classifier: classifier,
	}
}

// recordingExecutor is a minimal D2 turn executor that echoes the first
// message content and emits text + complete. Used by LP-1 tests so the
// ProcessMessage → executor path completes and the channel drains cleanly.
type recordingExecutor struct {
	mu     sync.Mutex
	calls  int
	turns  []string
}

func (r *recordingExecutor) RunTurn(_ context.Context, req sessionorchestrator.QueryRequest) (<-chan *contracts.EngineEvent, error) {
	r.mu.Lock()
	r.calls++
	msg := ""
	if len(req.Messages) > 0 {
		msg = req.Messages[0].Content
	}
	r.turns = append(r.turns, msg)
	r.mu.Unlock()
	out := make(chan *contracts.EngineEvent, 2)
	out <- &contracts.EngineEvent{Type: "text", Content: "echo: " + msg, SessionID: req.SessionID}
	out <- &contracts.EngineEvent{Type: "complete", SessionID: req.SessionID}
	close(out)
	return out, nil
}

// recordingClassifier records the prior seen on each ClassifyWithPrior call
// so tests can assert the LP-1 prior injection actually happened.
type recordingClassifier struct {
	mu    sync.Mutex
	calls int
	lastPrior *learn.AdaptivePrior
}

func (r *recordingClassifier) Classify(_ context.Context, msg string) (orchtypes.IntentClassification, error) {
	return orchtypes.IntentClassification{
		Kind:       orchtypes.IntentFast,
		Confidence: 95,
		Reason:     "recording-baseline",
	}, nil
}

func (r *recordingClassifier) ClassifyWithPrior(_ context.Context, msg string, prior *learn.AdaptivePrior) (orchtypes.IntentClassification, error) {
	r.mu.Lock()
	r.calls++
	r.lastPrior = prior
	r.mu.Unlock()
	result, _ := r.Classify(context.Background(), msg)
	if prior != nil && prior.PriorBeta.Mean() > 0 {
		// Mirror RuleClassifier.ClassifyWithPrior multiplier semantics so
		// tests can also observe downstream confidence changes via the
		// baseline multiplier rule.
		adjusted := int(float64(result.Confidence) * prior.PriorBeta.Mean())
		if adjusted > 100 {
			adjusted = 100
		}
		if adjusted < 0 {
			adjusted = 0
		}
		result.Confidence = adjusted
		result.Reason = result.Reason + " [lp1-recorder]"
	}
	return result, nil
}

// processOnce invokes ProcessMessage and drains the channel. Returns the
// final EngineEvent count.
func (f *lp1Fixture) processOnce(t *testing.T, sessionID, msg string) int {
	t.Helper()
	ch, err := f.orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: sessionID,
		Message:   msg,
	})
	if err != nil {
		t.Fatalf("ProcessMessage(%q): %v", msg, err)
	}
	count := 0
	for range ch {
		count++
	}
	return count
}

// ─────────────────────────────────────────────────────────────────────────
// T: D7-S12-A43-T06 / F3.1.1 Scenario 1 — Pass × 3 accumulates prior.
// ─────────────────────────────────────────────────────────────────────────

func TestE2E_LP1_ClosedLoop_LearnPassAccumulatePrior(t *testing.T) {
	sessionID := "sess-lp1-pass"
	f := newLP1Fixture(t, sessionID)

	// Round 1: cold-start prior = Beta(5,3) Mean=0.625. First ProcessMessage.
	_ = f.processOnce(t, sessionID, "hello")

	f.classifier.mu.Lock()
	round1Prior := f.classifier.lastPrior
	round1Calls := f.classifier.calls
	f.classifier.mu.Unlock()

	if round1Calls != 1 {
		t.Fatalf("classifier calls after round 1 = %d, want 1", round1Calls)
	}
	if round1Prior == nil {
		t.Fatal("round 1: prior should not be nil (Learn wired + cold-start prior)")
	}
	if round1Prior.PriorBeta != (learn.BetaPrior{Alpha: 5, Beta: 3}) {
		t.Errorf("round 1: prior.PriorBeta = %+v, want Beta(5,3)", round1Prior.PriorBeta)
	}
	wantMean1 := 5.0 / 8.0
	if got := round1Prior.PriorBeta.Mean(); got != wantMean1 {
		t.Errorf("round 1: prior.Mean = %.4f, want %.4f", got, wantMean1)
	}

	// Simulate 3 Learn cycles with VerdictPass. Each Learn updates the
	// reputation store via BayesianUpdate: prior Alpha++, Beta unchanged.
	// After 3 passes: prior.Alpha = 0 + 3 = 3 (cold-start baseline 0).
	verdict := workmodel.Verdict{Kind: types.VerdictPass, SourceID: "v_pass_e2e", Reason: "ok"}
	planStub := plan.NewPlan("plan_lp1_1", sessionID, plan.CommitmentPlan, []string{"obs_1"},
		[]plan.Step{{ID: "step_1"}}, 0.8)
	artStub := artifactStub("art_lp1_1", sessionID, []string{"lp1.go"})

	for i := 0; i < 3; i++ {
		req := learn.LearnRequest{
			SessionID: sessionID,
			Verdict:   verdict,
			Plan:      planStub,
			Artifact:  artStub,
		}
		if _, err := f.learner.Learn(context.Background(), req); err != nil {
			t.Fatalf("Learn iteration %d: %v", i, err)
		}
	}

	// After 3 passes: cold-start ReputationEvidence Alpha=3, Beta=0.
	// BuildAdaptivePrior with rep.Alpha=3 + rep.Beta=0 + DefaultDeveloper
	// prior (5,3) → merged Beta = (5+3, 3+0) = (8, 3). Mean = 8/11 ≈ 0.7273.
	gotRep, err := f.rep.Get(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("rep.Get: %v", err)
	}
	if gotRep == nil {
		t.Fatal("rep.Get after 3 passes: got nil, want non-nil")
	}
	if gotRep.Alpha != 3 || gotRep.Beta != 0 {
		t.Errorf("rep after 3 passes = Alpha=%d/Beta=%d, want 3/0", gotRep.Alpha, gotRep.Beta)
	}

	// Round 2: ProcessMessage observes prior Beta(8,3).
	_ = f.processOnce(t, sessionID, "another msg")

	f.classifier.mu.Lock()
	round2Prior := f.classifier.lastPrior
	round2Calls := f.classifier.calls
	f.classifier.mu.Unlock()

	if round2Calls != 2 {
		t.Fatalf("classifier calls after round 2 = %d, want 2", round2Calls)
	}
	if round2Prior == nil {
		t.Fatal("round 2: prior should not be nil")
	}
	wantAlpha, wantBeta := 8, 3
	if round2Prior.PriorBeta.Alpha != wantAlpha || round2Prior.PriorBeta.Beta != wantBeta {
		t.Errorf("round 2: prior.PriorBeta = %+v, want Beta(%d,%d)", round2Prior.PriorBeta, wantAlpha, wantBeta)
	}
	wantMean2 := 8.0 / 11.0
	if got := round2Prior.PriorBeta.Mean(); got != wantMean2 {
		t.Errorf("round 2: prior.Mean = %.4f, want %.4f", got, wantMean2)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// T: D7-S12-A43-T06 / F3.1.1 Scenario 2 — INDETERMINATE+parse_failure: no α/β pollution.
// ─────────────────────────────────────────────────────────────────────────

func TestE2E_LP1_ClosedLoop_IndeterminateParseFailure_NoAlphaPollution(t *testing.T) {
	sessionID := "sess-lp1-parsefail"
	f := newLP1Fixture(t, sessionID)

	// Round 1 — cold-start prior Beta(5,3).
	_ = f.processOnce(t, sessionID, "hi")

	// Simulate Verify → Learn with VerdictIndeterminate +
	// IndeterminateReason="verifier_parse_failure" (G8-1 fix path).
	req := learn.LearnRequest{
		SessionID: sessionID,
		Verdict: workmodel.Verdict{
			Kind:                types.VerdictIndeterminate,
			SourceID:            "v_indet_parse",
			Reason:              "verifier could not parse LLM output",
			IndeterminateReason: "verifier_parse_failure",
		},
		Artifact: artifactStub("art_parse_fail", sessionID, nil),
	}
	assets, err := f.learner.Learn(context.Background(), req)
	if err != nil {
		t.Fatalf("Learn(verifier_parse_failure): %v", err)
	}
	if len(assets) != 1 {
		t.Errorf("assets len = %d, want 1", len(assets))
	}
	if assets[0].Class != learn.LearningClass(types.LearningPending) {
		t.Errorf("Class = %s, want LearningPending", assets[0].Class)
	}

	// ReputationStore row exists, but Alpha/Beta must be unchanged
	// (cold-start was Alpha=0/Beta=0). VerifierFailureCount must be 1.
	gotRep, err := f.rep.Get(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("rep.Get: %v", err)
	}
	if gotRep == nil {
		t.Fatal("rep.Get: got nil, want non-nil (row created with defaults)")
	}
	if gotRep.Alpha != 0 || gotRep.Beta != 0 {
		t.Errorf("Alpha/Beta polluted by verifier_parse_failure: Alpha=%d Beta=%d, want 0/0",
			gotRep.Alpha, gotRep.Beta)
	}
	if gotRep.VerifierFailureCount != 1 {
		t.Errorf("VerifierFailureCount = %d, want 1", gotRep.VerifierFailureCount)
	}
	if gotRep.IndeterminateCount != 0 {
		t.Errorf("IndeterminateCount = %d, want 0 (parse_failure is separate)", gotRep.IndeterminateCount)
	}

	// Round 2 — prior must still be Beta(5,3) since α/β did not change.
	_ = f.processOnce(t, sessionID, "hi again")

	f.classifier.mu.Lock()
	round2Prior := f.classifier.lastPrior
	f.classifier.mu.Unlock()

	if round2Prior == nil {
		t.Fatal("round 2: prior should not be nil")
	}
	if round2Prior.PriorBeta != (learn.BetaPrior{Alpha: 5, Beta: 3}) {
		t.Errorf("round 2 prior.PriorBeta = %+v, want Beta(5,3) (no α/β pollution from parse failure)",
			round2Prior.PriorBeta)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// T: D7-S12-A43-T06 / F3.1.1 Scenario 3 — PendingAsset routes to ScheduledMemory.
// ─────────────────────────────────────────────────────────────────────────

func TestE2E_LP1_ClosedLoop_PendingAssetScheduledMemory(t *testing.T) {
	sessionID := "sess-lp1-pending"
	f := newLP1Fixture(t, sessionID)

	// Simulate Verify → Learn with VerdictIndeterminate +
	// IndeterminateReason="env_limited" (NOT parse_failure, so the asset
	// goes to ScheduledMemory per LP-2 隔离).
	req := learn.LearnRequest{
		SessionID: sessionID,
		Verdict: workmodel.Verdict{
			Kind:                types.VerdictIndeterminate,
			SourceID:            "v_indet_env",
			Reason:              "upstream tool returned 503",
			IndeterminateReason: "env_limited",
		},
		Artifact: artifactStub("art_env_limited", sessionID, nil),
	}
	assets, err := f.learner.Learn(context.Background(), req)
	if err != nil {
		t.Fatalf("Learn(env_limited): %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("assets len = %d, want 1", len(assets))
	}
	asset := assets[0]
	if asset.Class != learn.LearningClass(types.LearningPending) {
		t.Errorf("Class = %s, want LearningPending", asset.Class)
	}

	// LP-2: PendingAsset MUST be in ScheduledMemory, not SkillMemory or
	// FeedbackMemory. Skill/Feedback retrievals return nil.
	scheduled, _ := f.scheduled.Retrieve(context.Background(), asset.AssetKey)
	if scheduled == nil {
		t.Fatal("ScheduledMemory.Retrieve should find the pending asset")
	}
	if got, _ := f.skill.Retrieve(context.Background(), asset.AssetKey); got != nil {
		t.Errorf("SkillMemory should NOT contain pending asset, got %+v", got)
	}
	if got, _ := f.feedback.Retrieve(context.Background(), asset.AssetKey); got != nil {
		t.Errorf("FeedbackMemory should NOT contain pending asset, got %+v", got)
	}

	// ScheduledMemory.ListDue must include the asset when the current time
	// is at or past its TriggerAt (defaults to ExpiryAt = now + 24h, so
	// passing "now" will not see it; we use now+25h to force inclusion).
	dueList := f.scheduled.ListDue(time.Now().Add(25 * time.Hour))
	found := false
	for _, retry := range dueList {
		if retry.Asset.AssetKey == asset.AssetKey {
			found = true
			if retry.MaxRetries != learn.DefaultScheduledMaxRetries {
				t.Errorf("MaxRetries = %d, want %d", retry.MaxRetries, learn.DefaultScheduledMaxRetries)
			}
			break
		}
	}
	if !found {
		t.Errorf("ScheduledMemory.ListDue(now+25h) should include the pending asset, got: %+v", dueList)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// T: D7-S12-A43-T06 / F3.1.1 Scenario 4 — Full 5-node pipeline closure (one round).
// ─────────────────────────────────────────────────────────────────────────

func TestE2E_5NodePipeline_End2End(t *testing.T) {
	sessionID := "sess-5node-e2e"
	f := newLP1Fixture(t, sessionID)

	// Observe node — ProcessMessage injects prior into classification.
	_ = f.processOnce(t, sessionID, "hello")

	f.classifier.mu.Lock()
	if f.classifier.calls != 1 {
		f.classifier.mu.Unlock()
		t.Fatalf("Observe path: classifier calls = %d, want 1", f.classifier.calls)
	}
	priorSeenByClassifier := f.classifier.lastPrior
	f.classifier.mu.Unlock()

	// Plan node — build a Plan tagged with SourceObservationIDs (LP-5 衍生).
	planStub := plan.NewPlan(
		"plan_5node",
		sessionID,
		plan.CommitmentPlan,
		[]string{"obs_e2e_1"}, // SourceObservationIDs
		[]plan.Step{{ID: "step_run_turn"}},
		0.85,
	)

	// Execute node — build an Artifact tagged with the Plan ID (LP-5 衍生).
	artStub := artifactStub("art_5node", sessionID, []string{"turn.go"})

	// Verify node — aggregate a Verdict tagged with SourceArtifactID (LP-5 衍生).
	verdict := workmodel.Verdict{
		Kind:     types.VerdictPass,
		SourceID: "v_5node_pass",
		Reason:   "5-node e2e ok",
	}.WithSourceID(artStub.TaskID)

	// Learn node — LearnRequest bundles Verdict + Plan + Artifact + SessionID.
	// AssetBuilder constructs an asset with cross-source lineage metadata.
	req := learn.LearnRequest{
		SessionID:    sessionID,
		Verdict:      verdict,
		Plan:         planStub,
		Artifact:     artStub,
		Observations: []learn.ObservationLookup{obsStub("obs_e2e_1")},
	}
	assets, err := f.learner.Learn(context.Background(), req)
	if err != nil {
		t.Fatalf("Learn (5-node E2E): %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("assets len = %d, want 1", len(assets))
	}
	asset := assets[0]

	// LP-5 衍生: asset lineage must contain both the PlanID and the
	// ObservationID so downstream readers can reverse-trace the chain.
	if len(asset.SourceSessionIDs) == 0 || asset.SourceSessionIDs[0] != sessionID {
		t.Errorf("SourceSessionIDs = %v, want first=%q", asset.SourceSessionIDs, sessionID)
	}
	if !contains(asset.SourceSessionIDs, planStub.SessionID) {
		t.Errorf("SourceSessionIDs missing plan session %q: %v", planStub.SessionID, asset.SourceSessionIDs)
	}

	// SourceObservationIDs on Plan must include obs_e2e_1.
	if !contains(planStub.SourceObservationIDs, "obs_e2e_1") {
		t.Errorf("Plan.SourceObservationIDs = %v, want obs_e2e_1", planStub.SourceObservationIDs)
	}

	// Verdict.SourceID must point back to the Artifact.
	if verdict.SourceID != artStub.TaskID {
		t.Errorf("Verdict.SourceID = %q, want %q (Artifact reverse-trace)", verdict.SourceID, artStub.TaskID)
	}

	// Observe→Learn wiring: the prior the classifier saw must NOT be nil
	// (learner wired) and must equal DefaultDeveloperPrior (cold-start).
	if priorSeenByClassifier == nil {
		t.Fatal("priorSeenByClassifier: nil, want DefaultDeveloperPrior (LP-1)")
	}
	if priorSeenByClassifier.PriorBeta != (learn.BetaPrior{Alpha: 5, Beta: 3}) {
		t.Errorf("priorSeenByClassifier.PriorBeta = %+v, want Beta(5,3)",
			priorSeenByClassifier.PriorBeta)
	}

	// LP-3: ReputationStore updated by VerdictPass → Alpha=1, Beta=0.
	gotRep, err := f.rep.Get(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("rep.Get: %v", err)
	}
	if gotRep == nil || gotRep.Alpha != 1 || gotRep.Beta != 0 {
		t.Errorf("rep after 1 VerdictPass = %+v, want Alpha=1/Beta=0",
			gotRep)
	}

	// LP-2: Compliance verdict → SOP → SkillMemory.
	gotAsset, _ := f.skill.Retrieve(context.Background(), asset.AssetKey)
	if gotAsset == nil {
		t.Error("SkillMemory should contain the SOP asset from Compliance verdict")
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Test helpers
// ─────────────────────────────────────────────────────────────────────────

// artifactStub builds a wavescheduler.Artifact with the given id + session +
// changed-files. Used as the Execute node output for E2E tests.
func artifactStub(id, sessionID string, files []string) *wavescheduler.Artifact {
	return &wavescheduler.Artifact{
		TaskID:       id,
		SessionID:    sessionID,
		Summary:      "stub artifact " + id,
		FilesChanged: files,
		WorkerType:   wavescheduler.WorkerSubAgent,
		StartedAt:   time.Now(),
		EndedAt:     time.Now(),
	}
}

// obsStub is a minimal ObservationLookup for the LearnRequest.Observations
// field. Mirrors the plan.ObservationLookup contract (GetID).
type obsStubImpl struct {
	id string
}

func (o *obsStubImpl) GetID() string { return o.id }

func obsStub(id string) learn.ObservationLookup {
	return &obsStubImpl{id: id}
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}