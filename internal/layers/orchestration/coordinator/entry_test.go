package coordinator

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: d7.Entry adapter must satisfy the gateway contract.
//
// T: D7-D1-T01 — Entry.ProcessMessage routes through the orchestrator.
func TestEntry_ProcessMessage(t *testing.T) {
	exec := &fakeD2{}
	orch := NewSessionOrchestrator(DefaultConfig(), exec)
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
	orch := NewSessionOrchestrator(DefaultConfig(), exec)
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
	orch := NewSessionOrchestrator(DefaultConfig(), exec)
	entry := NewEntry(orch)
	// No SetInterruptHandler call — Cancel should still work.
	if err := entry.Cancel(context.Background(), "sess-lazy"); err != nil {
		t.Fatalf("Cancel err: %v", err)
	}
}

// T: orchestrator.ProcessMessageContract (string-args seam).
func TestSessionOrchestrator_ProcessMessageContract(t *testing.T) {
	exec := &fakeD2{}
	orch := NewSessionOrchestrator(DefaultConfig(), exec)
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
func TestSessionOrchestrator_D6Validator_Pass(t *testing.T) {
	exec := &fakeD2{}
	v := &fakeD6Validator{pass: true, reason: "ok"}
	orch := NewSessionOrchestrator(DefaultConfig(), exec, WithValidator(v))
	ch, err := orch.ProcessMessage(context.Background(), ProcessRequest{
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
func TestSessionOrchestrator_D6Validator_AdvisoryFail(t *testing.T) {
	exec := &fakeD2{}
	v := &fakeD6Validator{pass: false, reason: "risky"}
	orch := NewSessionOrchestrator(DefaultConfig(), exec, WithValidator(v))
	ch, err := orch.ProcessMessage(context.Background(), ProcessRequest{
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

type fakeD6Validator struct {
	calls  int
	pass   bool
	reason string
}

func (f *fakeD6Validator) ValidateOrchestration(_ context.Context, _ OrchestrationDecision) ValidationResult {
	f.calls++
	return ValidationResult{Pass: f.pass, Reason: f.reason}
}

// T: WithWorkModel installs a custom WorkModel that the orchestrator
// stores on the field.
func TestWithWorkModel_StoresModel(t *testing.T) {
	exec := &fakeD2{}
	wm := NewDelegatedWorkModel()
	orch := NewSessionOrchestrator(DefaultConfig(), exec, WithWorkModel(wm))
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
	orch := NewSessionOrchestrator(DefaultConfig(), exec)
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
	orch := NewSessionOrchestrator(DefaultConfig(), exec)
	// Replace classifier with one that errors.
	orch.classifier = &errClassifier{}
	_, err := orch.ProcessMessage(context.Background(), ProcessRequest{
		SessionID: "sess-ce",
		Message:   "hi",
	})
	if err == nil {
		t.Fatalf("expected classify error")
	}
}

type errClassifier struct{}

func (errClassifier) Classify(_ context.Context, _ string) (IntentClassification, error) {
	return IntentClassification{}, errors.New("simulated classify failure")
}

// T: NewFastPath with nil cfg falls back to defaults.
func TestNewFastPath_NilCfg(t *testing.T) {
	exec := &fakeD2{}
	fp := NewFastPath(nil, exec, nil)
	if fp.cfg == nil {
		t.Fatalf("cfg should fall back to DefaultConfig")
	}
	if fp.cfg.FastPathThreshold != 90 {
		t.Fatalf("default threshold mismatch: %d", fp.cfg.FastPathThreshold)
	}
}

// T: FastPath requires an executor; nil executor returns an error.
func TestFastPath_NilExecutor(t *testing.T) {
	fp := NewFastPath(DefaultConfig(), nil, nil)
	_, err := fp.Run(context.Background(), ProcessRequest{SessionID: "s", Message: "x"}, "")
	if err == nil {
		t.Fatalf("nil executor should error")
	}
}

// T: ProcessMessage classify-error path returns the wrapped error.
func TestProcessMessage_UnknownIntent(t *testing.T) {
	exec := &fakeD2{}
	orch := NewSessionOrchestrator(DefaultConfig(), exec)
	orch.classifier = &forcedKindClassifier{kind: "weird"}
	_, err := orch.ProcessMessage(context.Background(), ProcessRequest{
		SessionID: "sess-unk",
		Message:   "x",
	})
	if err == nil {
		t.Fatalf("unknown intent kind should error")
	}
}

type forcedKindClassifier struct{ kind IntentKind }

func (f *forcedKindClassifier) Classify(_ context.Context, _ string) (IntentClassification, error) {
	return IntentClassification{Kind: f.kind}, nil
}

// T: BuildConfig with nil file → defaults.
func TestBuildConfig_NilFile(t *testing.T) {
	cfg := BuildConfig(nil)
	def := DefaultConfig()
	if cfg.Enabled != def.Enabled || cfg.FastPathThreshold != def.FastPathThreshold {
		t.Fatalf("nil file should yield defaults")
	}
}

// T: BuildConfig with overrides merges file onto defaults.
func TestBuildConfig_Overrides(t *testing.T) {
	yes, threshold := true, 75
	whitelist := []string{"/x", "/y"}
	file := &FileConfig{
		Enabled:           &yes,
		FastPathThreshold: &threshold,
		CommandWhitelist:  whitelist,
	}
	cfg := BuildConfig(file)
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

// T: BuildConfig with a nil pointer field keeps the default.
func TestBuildConfig_NilFieldKeepsDefault(t *testing.T) {
	yes := true
	cfg := BuildConfig(&FileConfig{Enabled: &yes})
	if cfg.D6ValidationTimeoutMs != 50 {
		t.Fatalf("untouched D6ValidationTimeoutMs should keep default 50, got %d", cfg.D6ValidationTimeoutMs)
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

// T: DelegatedWorkModel methods return errors when not wired.
func TestDelegatedWorkModel_NotWired(t *testing.T) {
	wm := NewDelegatedWorkModel().(*DelegatedWorkModel)
	ctx := context.Background()
	if _, err := wm.CreateTask(ctx, TaskSpec{Subject: "s", Goal: "g"}); err == nil {
		t.Fatalf("CreateTask without wiring should error")
	}
	if err := wm.UpdateStatus(ctx, "t1", TaskStatusCompleted); err == nil {
		t.Fatalf("UpdateStatus without wiring should error")
	}
	// QueryWorkPlan returns an empty snapshot rather than an error in v1.0.
	snap, err := wm.QueryWorkPlan(ctx, "sess-q")
	if err != nil {
		t.Fatalf("QueryWorkPlan should not error in v1.0: %v", err)
	}
	if snap.SessionID != "sess-q" {
		t.Fatalf("QueryWorkPlan session = %q, want sess-q", snap.SessionID)
	}
}

// T: DelegatedWorkModel wired delegates forward correctly.
func TestDelegatedWorkModel_Wired(t *testing.T) {
	wm := NewDelegatedWorkModel().(*DelegatedWorkModel)
	wm.SetCreateTask(func(_ context.Context, subject, goal string) (string, error) {
		if subject != "s" || goal != "g" {
			return "", errors.New("mismatch")
		}
		return "t-1", nil
	})
	wm.SetUpdateStatus(func(_ context.Context, id string, st TaskStatus) error {
		if id != "t-1" || st != TaskStatusCompleted {
			return errors.New("mismatch")
		}
		return nil
	})
	wm.SetQueryPlan(func(_ context.Context, sid string) (WorkPlanSnapshot, error) {
		return WorkPlanSnapshot{SessionID: sid, Tasks: []TaskSpec{{ID: "t-1"}}}, nil
	})
	ctx := context.Background()
	id, err := wm.CreateTask(ctx, TaskSpec{Subject: "s", Goal: "g"})
	if err != nil || id != "t-1" {
		t.Fatalf("CreateTask: id=%q err=%v", id, err)
	}
	if err := wm.UpdateStatus(ctx, "t-1", TaskStatusCompleted); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	snap, err := wm.QueryWorkPlan(ctx, "sess")
	if err != nil {
		t.Fatalf("QueryWorkPlan: %v", err)
	}
	if len(snap.Tasks) != 1 || snap.Tasks[0].ID != "t-1" {
		t.Fatalf("snap.Tasks = %+v", snap.Tasks)
	}
}

// T: ensure types.Message is reachable from the d7 test surface (smoke).
func TestSmoke_MessageImport(t *testing.T) {
	_ = types.Message{Role: "user", Content: "x"}
}
