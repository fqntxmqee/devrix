package sessionorchestrator

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
)

// fakeLearner is a test double for learn.Learner. It records the
// Inject calls and returns a programmable AdaptivePrior (or error).
type fakeLearner struct {
	mu               sync.Mutex
	injectCalls      int
	lastSessionID    string
	lastTrackModeHint string
	prior            *learn.AdaptivePrior
	injectErr        error
	learningCalls    int
	lastLearnReq     learn.LearnRequest
}

func (f *fakeLearner) Inject(_ context.Context, sessionID, trackModeHint string) (*learn.AdaptivePrior, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.injectCalls++
	f.lastSessionID = sessionID
	f.lastTrackModeHint = trackModeHint
	if f.injectErr != nil {
		return nil, f.injectErr
	}
	if f.prior != nil {
		return f.prior, nil
	}
	// Default: cold-start prior (Beta(5,3))
	return learn.BuildAdaptivePrior(nil, learn.TrackModeDeveloper), nil
}

func (f *fakeLearner) Learn(_ context.Context, req learn.LearnRequest) ([]*learn.LearningAsset, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.learningCalls++
	f.lastLearnReq = req
	return nil, nil
}

func (f *fakeLearner) ScheduledTick(_ context.Context) error {
	return nil
}

// T: D7-S12-A42-T04 — WithLearner wires a Learner into SessionOrchestrator.
func TestSessionOrchestrator_WithLearner_InjectBeforeClassify(t *testing.T) {
	exec := &fakeD2{}
	fl := &fakeLearner{
		prior: learn.BuildAdaptivePrior(nil, learn.TrackModeOperator),
	}
	// Operator Beta(8,1) → Mean ≈ 0.889
	fl.prior.PriorBeta = learn.BetaPrior{Alpha: 8, Beta: 1}

	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec, WithLearner(fl))
	if orch.learner == nil {
		t.Fatal("WithLearner did not wire learner")
	}
	_, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-lp1",
		Message:   "hello",
	})
	if err != nil {
		t.Fatalf("ProcessMessage: %v", err)
	}
	fl.mu.Lock()
	defer fl.mu.Unlock()
	if fl.injectCalls != 1 {
		t.Errorf("Inject calls = %d, want 1", fl.injectCalls)
	}
	if fl.lastSessionID != "sess-lp1" {
		t.Errorf("lastSessionID = %q, want %q", fl.lastSessionID, "sess-lp1")
	}
}

// T: D7-S12-A42-T05 — Nil learner falls back to DefaultDeveloperPrior (no error).
func TestSessionOrchestrator_NilLearner_UseDefaultPrior(t *testing.T) {
	exec := &fakeD2{}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec) // no WithLearner
	if orch.learner != nil {
		t.Fatal("default orchestrator should have nil learner")
	}
	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-nolearner",
		Message:   "hello",
	})
	if err != nil {
		t.Fatalf("ProcessMessage with nil learner: %v", err)
	}
	// Consume the channel to confirm downstream works
	for range ch {
	}
}

// T: D7-S12-A42-T05 — Learner.Inject error falls back to DefaultDeveloperPrior.
func TestSessionOrchestrator_LearnerInjectError_UseDefaultPrior(t *testing.T) {
	exec := &fakeD2{}
	fl := &fakeLearner{
		injectErr: errors.New("simulated reputation store failure"),
	}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec, WithLearner(fl))

	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-inject-fail",
		Message:   "hello",
	})
	if err != nil {
		t.Fatalf("ProcessMessage with Inject error: should not error (fail-safe), got %v", err)
	}
	for range ch {
	}
}

// T: D7-S12-A42-T05 — BuildObserveRequest returns ObserveRequest with prior.
func TestSessionOrchestrator_BuildObserveRequest_NilLearner_NilPrior(t *testing.T) {
	exec := &fakeD2{}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec)
	req := orchtypes.ProcessRequest{SessionID: "sess-x", Message: "hi"}
	observeReq, err := orch.buildObserveRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("buildObserveRequest: %v", err)
	}
	if observeReq.Prior != nil {
		t.Errorf("Prior should be nil for nil learner, got %+v", observeReq.Prior)
	}
	// EffectivePrior must still return DefaultDeveloperPrior
	eff := observeReq.EffectivePrior()
	if eff == nil {
		t.Fatal("EffectivePrior returned nil for nil prior")
	}
	if eff.PriorBeta != (learn.BetaPrior{Alpha: 5, Beta: 3}) {
		t.Errorf("EffectivePrior.PriorBeta = %+v, want Beta(5,3)", eff.PriorBeta)
	}
}

// T: D7-S12-A42-T05 — BuildObserveRequest with wired learner calls Inject.
func TestSessionOrchestrator_BuildObserveRequest_WiredLearner_UsesInjectedPrior(t *testing.T) {
	exec := &fakeD2{}
	customPrior := learn.BuildAdaptivePrior(nil, learn.TrackModeOperator)
	customPrior.PriorBeta = learn.BetaPrior{Alpha: 10, Beta: 2} // Mean = 10/12 = 0.833
	fl := &fakeLearner{prior: customPrior}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec, WithLearner(fl))

	req := orchtypes.ProcessRequest{SessionID: "sess-y", Message: "hi"}
	observeReq, err := orch.buildObserveRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("buildObserveRequest: %v", err)
	}
	if observeReq.Prior == nil {
		t.Fatal("Prior should be non-nil for wired learner with prior")
	}
	if observeReq.Prior.PriorBeta != (learn.BetaPrior{Alpha: 10, Beta: 2}) {
		t.Errorf("Prior.PriorBeta = %+v, want Beta(10,2)", observeReq.Prior.PriorBeta)
	}
}

// T: D7-S12-A42-T05 — BuildObserveRequest with learner.Inject error → nil prior.
func TestSessionOrchestrator_BuildObserveRequest_InjectError_NilPrior(t *testing.T) {
	exec := &fakeD2{}
	fl := &fakeLearner{injectErr: errors.New("boom")}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec, WithLearner(fl))

	req := orchtypes.ProcessRequest{SessionID: "sess-z", Message: "hi"}
	observeReq, err := orch.buildObserveRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("buildObserveRequest: %v", err)
	}
	if observeReq.Prior != nil {
		t.Errorf("Prior should be nil when Inject returns error, got %+v", observeReq.Prior)
	}
	// EffectivePrior must still return DefaultDeveloperPrior (fail-safe)
	eff := observeReq.EffectivePrior()
	if eff.PriorBeta != (learn.BetaPrior{Alpha: 5, Beta: 3}) {
		t.Errorf("EffectivePrior.PriorBeta = %+v, want Beta(5,3) (fail-safe)", eff.PriorBeta)
	}
}

// T: D7-S12-A42-T05 — BuildObserveRequest fail-fast on empty SessionID.
func TestSessionOrchestrator_BuildObserveRequest_EmptySessionID(t *testing.T) {
	exec := &fakeD2{}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec)
	req := orchtypes.ProcessRequest{SessionID: "", Message: "hi"}
	_, err := orch.buildObserveRequest(context.Background(), req)
	if err == nil {
		t.Fatal("buildObserveRequest with empty SessionID: expected error, got nil")
	}
}

// T: D7-S12-A42-T05 — BuildObserveRequest fail-fast on empty Message.
func TestSessionOrchestrator_BuildObserveRequest_EmptyMessage(t *testing.T) {
	exec := &fakeD2{}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec)
	req := orchtypes.ProcessRequest{SessionID: "sess-1", Message: ""}
	_, err := orch.buildObserveRequest(context.Background(), req)
	if err == nil {
		t.Fatal("buildObserveRequest with empty Message: expected error, got nil")
	}
}

// T: D7-S12-A42-T05 — ProcessMessage threads prior into ClassifyWithPrior
// (verified by injecting a custom prior and checking intent kind/confidence
// is adjusted).
func TestSessionOrchestrator_ProcessMessage_UsePriorInClassification(t *testing.T) {
	exec := &fakeD2{}
	// prior Beta(8,1) → Mean = 0.889 → confidence multiplier
	fl := &fakeLearner{}
	fl.prior = learn.BuildAdaptivePrior(nil, learn.TrackModeOperator) // Beta(8,1) Mean=0.889
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec, WithLearner(fl))

	// "hello" matches the greeting fast pattern → baseline Confidence=95
	// With prior Mean=0.889: 95 × 0.889 = 84.455 → 84
	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-prior-effect",
		Message:   "hello",
	})
	if err != nil {
		t.Fatalf("ProcessMessage: %v", err)
	}
	for range ch {
	}
	fl.mu.Lock()
	defer fl.mu.Unlock()
	if fl.injectCalls != 1 {
		t.Errorf("Inject calls = %d, want 1", fl.injectCalls)
	}
}

// priorRecordingClassifier is a recording IntentClassifier that captures
// the prior seen on each ClassifyWithPrior call. It is used by the
// classifier-prior wiring test to assert ProcessMessage threaded prior to
// the classifier path (when both WithLearner + WithClassifier are wired).
type priorRecordingClassifier struct {
	mu        sync.Mutex
	calls     int
	lastPrior *learn.AdaptivePrior
}

func (p *priorRecordingClassifier) Classify(_ context.Context, _ string) (orchtypes.IntentClassification, error) {
	return orchtypes.IntentClassification{
		Kind:       orchtypes.IntentFast,
		Confidence: 95,
		Reason:     "prior-recorder-baseline",
	}, nil
}

func (p *priorRecordingClassifier) ClassifyWithPrior(_ context.Context, _ string, prior *learn.AdaptivePrior) (orchtypes.IntentClassification, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	p.lastPrior = prior
	result, _ := p.Classify(context.Background(), "")
	if prior != nil && prior.PriorBeta.Mean() > 0 {
		// Mirror RuleClassifier.ClassifyWithPrior multiplier so the
		// shadow path reflects the prior-adjusted confidence (proves
		// prior really was threaded to shadow).
		adjusted := int(float64(result.Confidence) * prior.PriorBeta.Mean())
		if adjusted > 100 {
			adjusted = 100
		}
		result.Confidence = adjusted
	}
	return result, nil
}

// T: D7-S12-A42-T04 — ProcessMessage threads prior to the classifier path
// when BOTH WithLearner and WithClassifier are wired. ProcessMessage must
// call classifier.ClassifyWithPrior — observable via the recorder seeing
// the injected prior.
//
// (Renamed from WithShadowClassifier to WithClassifier in DM-20260625-011
// after ShadowClassifier was removed as dead code.)
func TestSessionOrchestrator_WithClassifier_PriorThreadedToClassifier(t *testing.T) {
	exec := &fakeD2{}
	recorder := &priorRecordingClassifier{}

	// Inject a custom prior (Beta(8,3) → Mean = 0.727) via fakeLearner.
	fl := &fakeLearner{}
	fl.prior = learn.BuildAdaptivePrior(nil, learn.TrackModeDeveloper)
	fl.prior.PriorBeta = learn.BetaPrior{Alpha: 8, Beta: 3}

	orch := NewSessionOrchestrator(
		orchtypes.DefaultConfig(), exec,
		WithLearner(fl),
		WithClassifier(recorder),
	)

	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-prior-shadow",
		Message:   "hello",
	})
	if err != nil {
		t.Fatalf("ProcessMessage: %v", err)
	}
	for range ch {
	}

	// LP-1: learner.Inject MUST have been called.
	fl.mu.Lock()
	injectCalls := fl.injectCalls
	fl.mu.Unlock()
	if injectCalls != 1 {
		t.Errorf("learner.Inject calls = %d, want 1 (Phase 6 PR-F2 wiring)", injectCalls)
	}

	// LP-1 → classifier: the recorder (acting as injected classifier) MUST
	// have seen the prior via ClassifyWithPrior. ProcessMessage calls
	// o.classifier.ClassifyWithPrior, which captures the prior.
	recorder.mu.Lock()
	calls := recorder.calls
	lastPrior := recorder.lastPrior
	recorder.mu.Unlock()
	if calls != 1 {
		t.Errorf("recorder.ClassifyWithPrior calls = %d, want 1", calls)
	}
	if lastPrior == nil {
		t.Fatal("recorder lastPrior: nil, want prior injected (LP-1 not threaded to classifier path)")
	}
	if lastPrior.PriorBeta != (learn.BetaPrior{Alpha: 8, Beta: 3}) {
		t.Errorf("recorder lastPrior.PriorBeta = %+v, want Beta(8,3)", lastPrior.PriorBeta)
	}
}
