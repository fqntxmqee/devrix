package sessionorchestrator

import (
	"context"
	"errors"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: d7.Entry adapter must satisfy the gateway contract.
//
// T: D7-D1-T01 — Entry.ProcessMessage routes through the orchestrator.
func TestEntry_ProcessMessage(t *testing.T) {
	exec := &fakeD2{}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec)
	entry := NewEntry(orch)
	ch, err := entry.ProcessMessage(context.Background(), "sess-entry", "hello")
	if err != nil {
		t.Fatalf("ProcessMessage err: %v", err)
	}
	var events []*contracts.EngineEvent
	for ev := range ch {
		events = append(events, ev)
	}
	if len(events) < 2 {
		t.Fatalf("want ≥ 2 events, got %d", len(events))
	}
	if events[0].Type != "text" {
		t.Fatalf("first event should be text, got %q", events[0].Type)
	}
	if exec.calls != 1 {
		t.Fatalf("want 1 D2 call, got %d", exec.calls)
	}
	if exec.executedMsgs[0] != "hello" {
		t.Fatalf("message not forwarded: %q", exec.executedMsgs[0])
	}
}

// T: d7.Entry adapter Cancel path uses InterruptHandler.
func TestEntry_Cancel_WithWiredHandler(t *testing.T) {
	exec := &fakeD2{}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec)
	sink := &fakeSink{}
	var procCalled int32
	orch.SetInterruptHandler(NewInterruptHandler(orch, InterruptOptions{
		ProcessCanceler: func(_ string) error {
			atomic.AddInt32(&procCalled, 1)
			return nil
		},
		Sink: sink,
	}))
	entry := NewEntry(orch)
	if err := entry.Cancel(context.Background(), "sess-c"); err != nil {
		t.Fatalf("Cancel err: %v", err)
	}
	if atomic.LoadInt32(&procCalled) != 1 {
		t.Fatalf("ProcessCanceler not invoked")
	}
	if len(sink.events) != 1 {
		t.Fatalf("want 1 stopped event, got %d", len(sink.events))
	}
}

// T: d7.Entry Cancel lazily constructs the InterruptHandler if bootstrap
// did not wire one. This is the "cancel from gateway before bootstrap"
// defensive path.
func TestEntry_Cancel_LazyHandler(t *testing.T) {
	exec := &fakeD2{}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec)
	entry := NewEntry(orch)
	// No SetInterruptHandler call — Cancel should still work.
	if err := entry.Cancel(context.Background(), "sess-lazy"); err != nil {
		t.Fatalf("Cancel err: %v", err)
	}
}

// T: orchestrator.ProcessMessageContract (string-args seam).
func TestSessionOrchestrator_ProcessMessageContract(t *testing.T) {
	exec := &fakeD2{}
	cfg := orchtypes.DefaultConfig()
	cfg.FastPathThreshold = 0 // allow all fast-path classifications through
	orch := NewSessionOrchestrator(cfg, exec)
	ch, err := orch.ProcessMessageContract(context.Background(), "sess-pmc", "ping")
	if err != nil {
		t.Fatalf("ProcessMessageContract err: %v", err)
	}
	for range ch {
	}
	if exec.calls != 1 {
		t.Fatalf("want 1 D2 call, got %d", exec.calls)
	}
	if exec.executedMsgs[0] != "ping" {
		t.Fatalf("message not forwarded: %q", exec.executedMsgs[0])
	}
}

// T: D6 validator is invoked when wired, and the result is consumed
// (timeout → pass per the contract).
func TestSessionOrchestrator_AdvisoryValidator_Pass(t *testing.T) {
	exec := &fakeD2{}
	v := &fakeAdvisoryValidator{pass: true, reason: "ok"}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec, WithValidator(v))
	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-v",
		Message:   "hi",
	})
	if err != nil {
		t.Fatalf("ProcessMessage err: %v", err)
	}
	for range ch {
	}
	if v.calls != 1 {
		t.Fatalf("validator calls = %d, want 1", v.calls)
	}
}

// T: D6 validator pass=false still lets the request through (advisory).
// Per R1 Q11, validation is advisory; failure is logged but does not block.
func TestSessionOrchestrator_AdvisoryValidator_AdvisoryFail(t *testing.T) {
	exec := &fakeD2{}
	v := &fakeAdvisoryValidator{pass: false, reason: "risky"}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec, WithValidator(v))
	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-vfail",
		Message:   "hi",
	})
	if err != nil {
		t.Fatalf("ProcessMessage err: %v", err)
	}
	for range ch {
	}
	if exec.calls != 1 {
		t.Fatalf("advisory fail must not block D2, got calls=%d", exec.calls)
	}
}

type fakeAdvisoryValidator struct {
	calls  int
	pass   bool
	reason string
}

func (f *fakeAdvisoryValidator) ValidateOrchestration(_ context.Context, _ OrchestrationDecision) ValidationResult {
	f.calls++
	return ValidationResult{Pass: f.pass, Reason: f.reason}
}

// T: WithWorkModel installs a custom WorkModel that the orchestrator
// stores on the field.
func TestWithWorkModel_StoresModel(t *testing.T) {
	exec := &fakeD2{}
	wm := NewLocalWorkModel(nil)
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec, WithWorkModel(wm))
	if orch.workModel == nil {
		t.Fatalf("WithWorkModel should store the WorkModel")
	}
}

// T: registerInterrupt / unregisterInterrupt are internal but exercised
// here to keep the activeSessions map correct. We can't reach the
// registered func from outside, but we can confirm that successive
// register calls replace the prior cancel.
func TestSessionOrchestrator_RegisterInterrupt_Replace(t *testing.T) {
	exec := &fakeD2{}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec)
	var first, second int32
	cancel1 := func() { atomic.AddInt32(&first, 1) }
	cancel2 := func() { atomic.AddInt32(&second, 1) }
	orch.registerInterrupt("sess-r", cancel1)
	orch.registerInterrupt("sess-r", cancel2)
	// Registering a second cancel for the same session must invoke the
	// first (best-effort cancel of the previous run).
	if atomic.LoadInt32(&first) != 1 {
		t.Fatalf("first cancel not invoked, got %d", first)
	}
	if atomic.LoadInt32(&second) != 0 {
		t.Fatalf("second cancel should not run until explicit unregister")
	}
	orch.unregisterInterrupt("sess-r")
	// After unregister, the map entry is gone.
	orch.mu.Lock()
	if _, ok := orch.activeSessions["sess-r"]; ok {
		t.Fatalf("entry should be removed after unregister")
	}
	orch.mu.Unlock()
}

// T: ProcessMessage propagates classifier errors.
func TestSessionOrchestrator_ClassifyError(t *testing.T) {
	exec := &fakeD2{}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec)
	// Replace classifier with one that errors.
	orch.classifier = &errClassifier{}
	_, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-ce",
		Message:   "hi",
	})
	if err == nil {
		t.Fatalf("expected classify error")
	}
}

type errClassifier struct{}

func (errClassifier) Classify(_ context.Context, _ string) (orchtypes.IntentClassification, error) {
	return orchtypes.IntentClassification{}, errors.New("simulated classify failure")
}

func (errClassifier) ClassifyWithPrior(_ context.Context, _ string, _ *learn.AdaptivePrior) (orchtypes.IntentClassification, error) {
	return orchtypes.IntentClassification{}, errors.New("simulated classify failure")
}

// T: NewFastPath with nil cfg falls back to defaults.
func TestNewFastPath_NilCfg(t *testing.T) {
	exec := &fakeD2{}
	fp := NewFastPath(nil, exec, nil)
	if fp.cfg == nil {
		t.Fatalf("cfg should fall back to orchtypes.DefaultConfig")
	}
	if fp.cfg.FastPathThreshold != 90 {
		t.Fatalf("default threshold mismatch: %d", fp.cfg.FastPathThreshold)
	}
}

// T: FastPath requires an executor; nil executor returns an error.
func TestFastPath_NilExecutor(t *testing.T) {
	fp := NewFastPath(orchtypes.DefaultConfig(), nil, nil)
	_, err := fp.Run(context.Background(), orchtypes.ProcessRequest{SessionID: "s", Message: "x"}, "")
	if err == nil {
		t.Fatalf("nil executor should error")
	}
}

// T: ProcessMessage classify-error path returns the wrapped error.
func TestProcessMessage_UnknownIntent(t *testing.T) {
	exec := &fakeD2{}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec)
	orch.classifier = &forcedKindClassifier{kind: "weird"}
	_, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-unk",
		Message:   "x",
	})
	if err == nil {
		t.Fatalf("unknown intent kind should error")
	}
}

type forcedKindClassifier struct{ kind orchtypes.IntentKind }

func (f *forcedKindClassifier) Classify(_ context.Context, _ string) (orchtypes.IntentClassification, error) {
	return orchtypes.IntentClassification{Kind: f.kind}, nil
}

func (f *forcedKindClassifier) ClassifyWithPrior(_ context.Context, _ string, _ *learn.AdaptivePrior) (orchtypes.IntentClassification, error) {
	return orchtypes.IntentClassification{Kind: f.kind}, nil
}

// T: orchtypes.BuildConfig with nil file → defaults.
func TestBuildConfig_NilFile(t *testing.T) {
	cfg := orchtypes.BuildConfig(nil)
	def := orchtypes.DefaultConfig()
	if cfg.Enabled != def.Enabled || cfg.FastPathThreshold != def.FastPathThreshold {
		t.Fatalf("nil file should yield defaults")
	}
}

// T: orchtypes.BuildConfig with overrides merges file onto defaults.
func TestBuildConfig_Overrides(t *testing.T) {
	yes, threshold := true, 75
	whitelist := []string{"/x", "/y"}
	file := &orchtypes.FileConfig{
		Enabled:           &yes,
		FastPathThreshold: &threshold,
		CommandWhitelist:  whitelist,
	}
	cfg := orchtypes.BuildConfig(file)
	if !cfg.Enabled {
		t.Fatalf("Enabled override not applied")
	}
	if cfg.FastPathThreshold != 75 {
		t.Fatalf("threshold override = %d, want 75", cfg.FastPathThreshold)
	}
	if len(cfg.CommandWhitelist) != 2 || cfg.CommandWhitelist[0] != "/x" {
		t.Fatalf("whitelist not applied: %v", cfg.CommandWhitelist)
	}
	// Untouched fields keep defaults.
	if !cfg.CommandFirst {
		t.Fatalf("untouched CommandFirst should be default true")
	}
}

// T: orchtypes.BuildConfig with a nil pointer field keeps the default.
func TestBuildConfig_NilFieldKeepsDefault(t *testing.T) {
	yes := true
	cfg := orchtypes.BuildConfig(&orchtypes.FileConfig{Enabled: &yes})
	if cfg.AdvisoryValidationTimeoutMs != 50 {
		t.Fatalf("untouched AdvisoryValidationTimeoutMs should keep default 50, got %d", cfg.AdvisoryValidationTimeoutMs)
	}
}

// T: durationOrDefault edge cases.
func TestDurationOrDefault(t *testing.T) {
	if durationOrDefault(0) != 50*time.Millisecond {
		t.Fatalf("zero should fall back to 50ms default")
	}
	if durationOrDefault(10) != 10*time.Millisecond {
		t.Fatalf("explicit 10 should yield 10ms")
	}
	if durationOrDefault(-1) != 50*time.Millisecond {
		t.Fatalf("negative should fall back to default")
	}
}

// T: D7-S1-T07 LocalWorkModel.QueryWorkPlan returns Background tasks from provider.
func TestLocalWorkModel_QueryWorkPlan_Background(t *testing.T) {
	wm := NewLocalWorkModel(workmodel.NewTaskManager())

	snap, err := wm.QueryWorkPlan(context.Background(), "sess-bg")
	if err != nil {
		t.Fatalf("QueryWorkPlan: %v", err)
	}
	if snap.Background != nil {
		t.Fatalf("Background without provider should be nil, got %+v", snap.Background)
	}

	wm.SetBackgroundProvider(func(sid string) []BackgroundLite {
		if sid != "sess-bg" {
			t.Errorf("provider called with session %q, want sess-bg", sid)
		}
		return []BackgroundLite{
			{RunID: "bg-1", Status: orchtypes.TaskStatusInProgress, Output: ""},
			{RunID: "bg-2", Status: orchtypes.TaskStatusCompleted, Output: "done"},
		}
	})

	snap, err = wm.QueryWorkPlan(context.Background(), "sess-bg")
	if err != nil {
		t.Fatalf("QueryWorkPlan: %v", err)
	}
	if len(snap.Background) != 2 {
		t.Fatalf("Background count = %d, want 2: %+v", len(snap.Background), snap.Background)
	}
	if snap.Background[0].RunID != "bg-1" || snap.Background[0].Status != orchtypes.TaskStatusInProgress {
		t.Errorf("Background[0] = %+v", snap.Background[0])
	}
	if snap.Background[1].RunID != "bg-2" || snap.Background[1].Status != orchtypes.TaskStatusCompleted || snap.Background[1].Output != "done" {
		t.Errorf("Background[1] = %+v", snap.Background[1])
	}
}

// T: ensure types.Message is reachable from the d7 test surface (smoke).
func TestSmoke_MessageImport(t *testing.T) {
	_ = types.Message{Role: "user", Content: "x"}
}
