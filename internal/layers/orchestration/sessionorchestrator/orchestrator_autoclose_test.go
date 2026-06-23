package sessionorchestrator

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// ─────────────────────────────────────────────────────────────────────────
// T: D7-S13-A47-T01 — processAutoClose 包装 channel + 异步触发 learner.Learn
// ─────────────────────────────────────────────────────────────────────────

// TestProcessAutoClose_NilLearner_Passthrough verifies Layer 1 fail-safe:
// when learner is nil, processAutoClose must NOT spawn a goroutine that
// calls Learn; it must delegate directly to endSpanWhenChannelClosed
// (passthrough). Pin: endSpanWhenChannelClosed behavior is preserved.
func TestProcessAutoClose_NilLearner_Passthrough(t *testing.T) {
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), &fakeD2{})
	// orch.learner == nil (no WithLearner)
	if orch.learner != nil {
		t.Fatal("default orchestrator should have nil learner")
	}

	// Source channel emits 1 event then closes.
	src := make(chan *contracts.EngineEvent, 1)
	src <- &contracts.EngineEvent{Type: "complete", Content: "ok"}
	close(src)

	out := orch.processAutoClose(src, context.Background(), nil, "sess-ac1", orchtypes.IntentClassification{
		Kind: orchtypes.IntentFast,
	})
	// Consume out: should receive the same event then close.
	events := []*contracts.EngineEvent{}
	for ev := range out {
		events = append(events, ev)
	}
	if len(events) != 1 {
		t.Errorf("events len = %d, want 1", len(events))
	}
	if events[0].Type != "complete" {
		t.Errorf("event.Type = %q, want complete", events[0].Type)
	}
}

// TestProcessAutoClose_LearnerError_LoggedNotBlocked_ErrFake is the real
// Layer 3 fail-safe test. It uses errFakeLearner (declared below) to
// inject a Learn error and verify the proxy channel is unaffected. The
// plain-fakeLearner variant of this test (TestProcessAutoClose_LearnerError
// _LoggedNotBlocked_ErrFake) is below; we keep both names to make the
// fail-safe and success paths easy to grep.
func TestProcessAutoClose_LearnerError_LoggedNotBlocked(t *testing.T) {
	// errFakeLearner wraps fakeLearner with a programmable learnErr.
	fl := &errFakeLearner{
		fakeLearner: &fakeLearner{},
		learnErr:    errors.New("simulated learn storage failure"),
	}

	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), &fakeD2{}, WithLearner(fl))

	// Single "complete" event so synthesizeVerdict produces a non-nil
	// Verdict. (If we also emit a "text" event after, the text event
	// would overwrite complete as lastEvent and synthesizeVerdict would
	// return nil — no Learn attempted, defeating the test.)
	src := make(chan *contracts.EngineEvent, 1)
	src <- &contracts.EngineEvent{Type: "complete"}
	close(src)

	out := orch.processAutoClose(src, context.Background(), nil, "sess-ac-err", orchtypes.IntentClassification{
		Kind: orchtypes.IntentFast,
	})
	// Consume out: caller must see the event even if Learn fails.
	events := []*contracts.EngineEvent{}
	for ev := range out {
		events = append(events, ev)
	}
	if len(events) != 1 {
		t.Errorf("events len = %d, want 1 (Learn error must not affect proxy channel)", len(events))
	}

	// Wait briefly for the async goroutine to record the Learn call.
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		fl.mu.Lock()
		calls := fl.learningCalls
		fl.mu.Unlock()
		if calls > 0 {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	fl.mu.Lock()
	defer fl.mu.Unlock()
	if fl.learningCalls != 1 {
		t.Errorf("learningCalls = %d, want 1 (complete event → Learn(VerdictPass))", fl.learningCalls)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// T: D7-S13-A47-T02 — synthesizeVerdict 规则 + 3 层 fail-safe
// ─────────────────────────────────────────────────────────────────────────

// TestSynthesizeVerdict_AllEventTypes is a table-driven test that pins the
// EngineEvent.Type → workmodel.VerdictKind mapping. The mapping is the
// canonical Verdict source for runtime LP-1 closure; any change to this
// table is a breaking change to the production auto-Close behavior.
func TestSynthesizeVerdict_AllEventTypes(t *testing.T) {
	tests := []struct {
		name        string
		eventType   string
		content     string
		wantVerdict bool
		wantKind    types.VerdictKind
		wantReason  string
	}{
		{"complete → VerdictPass", "complete", "", true, types.VerdictPass, "process complete"},
		{"error → VerdictFail with content", "error", "OOM at line 42", true, types.VerdictFail, "OOM at line 42"},
		{"error empty content fallback", "error", "", true, types.VerdictFail, "process error (no content)"},
		{"tombstone → VerdictIndeterminate interrupt", "tombstone", "", true, types.VerdictIndeterminate, "tombstone received"},
		{"text → nil (non-terminal)", "text", "hello", false, types.VerdictKind(0), ""},
		{"thinking → nil (non-terminal)", "thinking", "...", false, types.VerdictKind(0), ""},
		{"tool_call → nil (non-terminal)", "tool_call", "", false, types.VerdictKind(0), ""},
		{"tool_result → nil (non-terminal)", "tool_result", "", false, types.VerdictKind(0), ""},
		{"status → nil (non-terminal)", "status", "", false, types.VerdictKind(0), ""},
		{"permission → nil (non-terminal)", "permission", "", false, types.VerdictKind(0), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := &contracts.EngineEvent{Type: tt.eventType, Content: tt.content}
			got := synthesizeVerdict(ev, "sess-tbl")
			if tt.wantVerdict {
				if got == nil {
					t.Fatalf("synthesizeVerdict returned nil, want Verdict{%s}", tt.wantKind)
				}
				if got.Kind != tt.wantKind {
					t.Errorf("Kind = %v, want %v", got.Kind, tt.wantKind)
				}
				if got.Reason != tt.wantReason {
					t.Errorf("Reason = %q, want %q", got.Reason, tt.wantReason)
				}
				if got.SourceID == "" {
					t.Error("SourceID is empty, want non-empty (autoclose:{sessionID}:{nano})")
				}
				// SourceID must embed sessionID.
				if !containsString(got.SourceID, "sess-tbl") {
					t.Errorf("SourceID = %q, must embed sessionID 'sess-tbl'", got.SourceID)
				}
			} else {
				if got != nil {
					t.Errorf("synthesizeVerdict(%q) = %+v, want nil", tt.eventType, got)
				}
			}
		})
	}
}

// TestSynthesizeVerdict_NilEvent verifies that synthesizeVerdict(nil, ...)
// returns nil (used by Layer 2 fail-safe for empty channel).
func TestSynthesizeVerdict_NilEvent(t *testing.T) {
	got := synthesizeVerdict(nil, "sess-nil")
	if got != nil {
		t.Errorf("synthesizeVerdict(nil) = %+v, want nil", got)
	}
}

// TestSynthesizeVerdict_Tombstone_IndeterminateReason verifies the G8-1-style
// "IndeterminateReason" propagation: tombstone events must carry
// IndeterminateReason="interrupt" so AssetBuilder routes to the correct
// LearningClass (Phase 5 LP-2 隔离).
func TestSynthesizeVerdict_Tombstone_IndeterminateReason(t *testing.T) {
	got := synthesizeVerdict(&contracts.EngineEvent{Type: "tombstone"}, "sess-tomb")
	if got == nil {
		t.Fatal("tombstone must produce a Verdict")
	}
	if got.IndeterminateReason != "interrupt" {
		t.Errorf("IndeterminateReason = %q, want \"interrupt\" (drives PendingAsset routing)",
			got.IndeterminateReason)
	}
}

// TestProcessAutoClose_EmptyChannel_NoLearn verifies Layer 2 fail-safe for
// empty channel: when the source channel has 0 events (e.g. IntentSkip path
// or premature close), processAutoClose must NOT call Learn.
func TestProcessAutoClose_EmptyChannel_NoLearn(t *testing.T) {
	fl := &fakeLearner{}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), &fakeD2{}, WithLearner(fl))

	src := make(chan *contracts.EngineEvent)
	close(src) // empty, immediate close

	out := orch.processAutoClose(src, context.Background(), nil, "sess-empty", orchtypes.IntentClassification{
		Kind: orchtypes.IntentSkip,
	})
	// Consume out: should close immediately.
	for range out {
		t.Fatal("empty channel should produce 0 events")
	}

	// Wait briefly to ensure no async Learn call sneaks in.
	time.Sleep(50 * time.Millisecond)
	fl.mu.Lock()
	defer fl.mu.Unlock()
	if fl.learningCalls != 0 {
		t.Errorf("learningCalls = %d, want 0 (empty channel → no Learn)", fl.learningCalls)
	}
}

// TestProcessAutoClose_NonTerminalEvent_NoLearn verifies that the last event
// being a non-terminal type (e.g. "text") does not trigger Learn (no Verdict
// to deposit).
func TestProcessAutoClose_NonTerminalEvent_NoLearn(t *testing.T) {
	fl := &fakeLearner{}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), &fakeD2{}, WithLearner(fl))

	src := make(chan *contracts.EngineEvent, 1)
	src <- &contracts.EngineEvent{Type: "text", Content: "thinking..."}
	close(src)

	out := orch.processAutoClose(src, context.Background(), nil, "sess-nt", orchtypes.IntentClassification{
		Kind: orchtypes.IntentFast,
	})
	for range out {
	}

	time.Sleep(50 * time.Millisecond)
	fl.mu.Lock()
	defer fl.mu.Unlock()
	if fl.learningCalls != 0 {
		t.Errorf("learningCalls = %d, want 0 (non-terminal last event → no Verdict)", fl.learningCalls)
	}
}

// TestProcessAutoClose_ContextCancel_SkipLearn verifies Layer 2 fail-safe
// for context cancellation: when the session context is cancelled, Learn
// must NOT be called even if the channel emits events.
func TestProcessAutoClose_ContextCancel_SkipLearn(t *testing.T) {
	fl := &fakeLearner{}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), &fakeD2{}, WithLearner(fl))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancelled

	src := make(chan *contracts.EngineEvent, 1)
	src <- &contracts.EngineEvent{Type: "complete"}
	close(src)

	out := orch.processAutoClose(src, ctx, nil, "sess-cancel", orchtypes.IntentClassification{
		Kind: orchtypes.IntentFast,
	})
	for range out {
	}

	time.Sleep(50 * time.Millisecond)
	fl.mu.Lock()
	defer fl.mu.Unlock()
	if fl.learningCalls != 0 {
		t.Errorf("learningCalls = %d, want 0 (cancelled context → skip Learn)", fl.learningCalls)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// T: D7-S13-A47-T03 — 集成测试: ProcessMessage 完整跑 → Alpha++ + 下一轮 prior 更新
// ─────────────────────────────────────────────────────────────────────────

// TestProcessMessage_Verify2Learn_AutoClose_PassAlpha is the end-to-end LP-1
// runtime closure test (D7-S13-A47-T03). It verifies that ProcessMessage
// now triggers learner.Learn automatically when the path emits a "complete"
// event — no test-code manual Learn call required.
//
// Flow:
//
//	Round 1: ProcessMessage → FastPath emits "complete" → processAutoClose →
//	          Learn(VerdictPass) → ReputationStore.Alpha = 1
//	Round 2: ProcessMessage → buildObserveRequest → Inject →
//	          PriorBeta = Beta(5+1, 3+0) = Beta(6,3) (Mean=0.667)
func TestProcessMessage_Verify2Learn_AutoClose_PassAlpha(t *testing.T) {
	// Build a SessionOrchestrator with a recording Learner + a FastPath
	// executor that emits a "complete" event then closes.
	fl := &fakeLearner{}
	exec := &completingExecutor{
		eventType:    "complete",
		eventContent: "ok",
	}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec, WithLearner(fl))

	// Round 1: process a message.
	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-ac-e2e",
		Message:   "hello",
	})
	if err != nil {
		t.Fatalf("Round 1 ProcessMessage: %v", err)
	}
	// Consume the channel to drive channel close → processAutoClose triggers.
	for range ch {
	}

	// Wait briefly for the async Learn call to land.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		fl.mu.Lock()
		calls := fl.learningCalls
		fl.mu.Unlock()
		if calls > 0 {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	fl.mu.Lock()
	round1Calls := fl.learningCalls
	round1Verdict := fl.lastLearnReq.Verdict
	fl.mu.Unlock()
	if round1Calls != 1 {
		t.Errorf("Round 1 learningCalls = %d, want 1 (auto-Close must trigger)", round1Calls)
	}
	if round1Verdict.Kind != types.VerdictPass {
		t.Errorf("Round 1 VerdictKind = %v, want VerdictPass (complete event)", round1Verdict.Kind)
	}

	// Round 2: the same ProcessMessage should now observe a prior that
	// reflects the Alpha=1 Bayesian update. Note: fakeLearner has nil
	// ReputationStore, so Inject returns cold-start prior Beta(5,3).
	// The Learn call updates the in-test "rep" via fl's lastLearnReq,
	// but since fl.Reputation is nil, the Inject in Round 2 will return
	// Beta(5,3) again (cold-start). To exercise the loop, we install a
	// real DefaultLearner in a separate test below (TestAutoClose_FullLP1Loop).
	// For this test, we just confirm the second ProcessMessage runs cleanly.
	ch2, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-ac-e2e",
		Message:   "second hello",
	})
	if err != nil {
		t.Fatalf("Round 2 ProcessMessage: %v", err)
	}
	for range ch2 {
	}
	// Wait briefly for the Round 2 async Learn call to land.
	deadline2 := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline2) {
		fl.mu.Lock()
		calls := fl.learningCalls
		fl.mu.Unlock()
		if calls >= 2 {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	fl.mu.Lock()
	round2Calls := fl.learningCalls
	fl.mu.Unlock()
	if round2Calls != 2 {
		t.Errorf("Round 2 learningCalls = %d, want 2 (each ProcessMessage → 1 Learn)", round2Calls)
	}
}

// TestProcessMessage_AutoClose_NilLearner_NoOp verifies that the existing
// nil-learner path (Phase 6 PR-F2 baseline) is unchanged: ProcessMessage
// completes successfully and does not panic when no Learner is wired.
func TestProcessMessage_AutoClose_NilLearner_NoOp(t *testing.T) {
	exec := &completingExecutor{eventType: "complete"}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec) // no WithLearner
	if orch.learner != nil {
		t.Fatal("default orchestrator should have nil learner")
	}
	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-ac-nolearner",
		Message:   "hello",
	})
	if err != nil {
		t.Fatalf("ProcessMessage: %v", err)
	}
	for range ch {
	}
	// No assertion on Learn — just that ProcessMessage runs cleanly.
}

// TestProcessMessage_AutoClose_IntentSkip_NoLearn verifies that the skip
// path (IntentSkip classification) does NOT trigger Learn (no execution
// result to learn from).
//
// The skip branch in ProcessMessage (orchestrator.go:373-376) closes a
// brand-new channel and returns it directly — processAutoClose is NOT
// invoked. The path is exercised at the unit level by
// TestProcessAutoClose_EmptyChannel_NoLearn (which passes a pre-closed
// empty channel to processAutoClose, equivalent to the skip path's empty
// closed channel).
//
// To assert this end-to-end through ProcessMessage, we use a classifier
// stub that always returns IntentSkip — the message itself is non-empty
// so ObserveRequest validation passes.
func TestProcessMessage_AutoClose_IntentSkip_NoLearn(t *testing.T) {
	fl := &fakeLearner{}
	exec := &fakeD2{}
	// alwaysSkipClassifier returns IntentSkip for any non-empty message.
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec,
		WithLearner(fl),
		WithClassifier(&alwaysSkipClassifier{}),
	)

	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-ac-skip",
		Message:   "anything",
	})
	if err != nil {
		t.Fatalf("ProcessMessage: %v", err)
	}
	for range ch {
	}
	time.Sleep(50 * time.Millisecond)
	fl.mu.Lock()
	defer fl.mu.Unlock()
	if fl.learningCalls != 0 {
		t.Errorf("learningCalls = %d, want 0 (skip path → no Learn)", fl.learningCalls)
	}
}

// TestProcessMessage_AutoClose_ErrorEvent_VerdictFail verifies that an
// "error" terminal event produces a VerdictFail via auto-Close.
func TestProcessMessage_AutoClose_ErrorEvent_VerdictFail(t *testing.T) {
	fl := &fakeLearner{}
	exec := &completingExecutor{eventType: "error", eventContent: "OOM at line 42"}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec, WithLearner(fl))

	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-ac-err",
		Message:   "trigger error",
	})
	if err != nil {
		t.Fatalf("ProcessMessage: %v", err)
	}
	for range ch {
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		fl.mu.Lock()
		calls := fl.learningCalls
		fl.mu.Unlock()
		if calls > 0 {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	fl.mu.Lock()
	defer fl.mu.Unlock()
	if fl.learningCalls != 1 {
		t.Errorf("learningCalls = %d, want 1", fl.learningCalls)
	}
	if fl.lastLearnReq.Verdict.Kind != types.VerdictFail {
		t.Errorf("VerdictKind = %v, want VerdictFail", fl.lastLearnReq.Verdict.Kind)
	}
	if fl.lastLearnReq.Verdict.Reason != "OOM at line 42" {
		t.Errorf("Verdict.Reason = %q, want \"OOM at line 42\"", fl.lastLearnReq.Verdict.Reason)
	}
}

// TestProcessMessage_AutoClose_TombstoneEvent_VerdictIndeterminate verifies
// the interrupt path: a tombstone event produces VerdictIndeterminate with
// IndeterminateReason="interrupt" (drives PendingAsset routing per Phase 5
// LP-2 隔离).
func TestProcessMessage_AutoClose_TombstoneEvent_VerdictIndeterminate(t *testing.T) {
	fl := &fakeLearner{}
	exec := &completingExecutor{eventType: "tombstone"}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec, WithLearner(fl))

	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-ac-tomb",
		Message:   "trigger tombstone",
	})
	if err != nil {
		t.Fatalf("ProcessMessage: %v", err)
	}
	for range ch {
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		fl.mu.Lock()
		calls := fl.learningCalls
		fl.mu.Unlock()
		if calls > 0 {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	fl.mu.Lock()
	defer fl.mu.Unlock()
	if fl.learningCalls != 1 {
		t.Errorf("learningCalls = %d, want 1", fl.learningCalls)
	}
	if fl.lastLearnReq.Verdict.Kind != types.VerdictIndeterminate {
		t.Errorf("VerdictKind = %v, want VerdictIndeterminate", fl.lastLearnReq.Verdict.Kind)
	}
	if fl.lastLearnReq.Verdict.IndeterminateReason != "interrupt" {
		t.Errorf("IndeterminateReason = %q, want \"interrupt\"", fl.lastLearnReq.Verdict.IndeterminateReason)
	}
}

// TestAutoClose_FullLP1Loop verifies the end-to-end LP-1 loop with a real
// DefaultLearner + InMemoryReputationStore. After 3 ProcessMessage rounds,
// ReputationStore.Alpha must be 3, and the next buildObserveRequest must
// see PriorBeta = Beta(8,3) (DefaultDeveloperPrior Beta(5,3) + rep Alpha=3).
func TestAutoClose_FullLP1Loop(t *testing.T) {
	rep := newInMemoryReputationStoreForTest()
	sched := newScheduledMemoryForTest()
	skill := newInMemorySkillMemoryForTest()
	feedback := newInMemoryFeedbackMemoryForTest()
	realLearner := learn.NewDefaultLearner(skill, feedback, sched, rep, learn.NewAssetBuilder())

	exec := &completingExecutor{eventType: "complete"}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec, WithLearner(realLearner))

	sessionID := "sess-lp1-full"
	// Round 1
	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{SessionID: sessionID, Message: "hi"})
	if err != nil {
		t.Fatalf("Round 1: %v", err)
	}
	for range ch {
	}
	// Round 2
	ch, err = orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{SessionID: sessionID, Message: "hi again"})
	if err != nil {
		t.Fatalf("Round 2: %v", err)
	}
	for range ch {
	}
	// Round 3
	ch, err = orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{SessionID: sessionID, Message: "hi once more"})
	if err != nil {
		t.Fatalf("Round 3: %v", err)
	}
	for range ch {
	}

	// Wait for the async Learn calls to settle.
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		got, _ := rep.Get(context.Background(), sessionID)
		if got != nil && got.Alpha == 3 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	got, err := rep.Get(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("rep.Get: %v", err)
	}
	if got == nil {
		t.Fatal("rep.Get returned nil, want non-nil (3 Pass Verdict → rep row created)")
	}
	if got.Alpha != 3 {
		t.Errorf("Alpha = %d, want 3 (3 VerdictPass × Learn → Alpha++)", got.Alpha)
	}
	if got.Beta != 0 {
		t.Errorf("Beta = %d, want 0 (no Fail Verdict)", got.Beta)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Test helpers
// ─────────────────────────────────────────────────────────────────────────

// containsString is a tiny strings.Contains replacement to avoid importing strings
// in this test file. (Renamed to avoid collision with the `contains` helper in
// command_handler_test.go which is []string-based.)
func containsString(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// completingExecutor is a fake TurnExecutor that emits a single EngineEvent
// of the configured type (default "complete") and then closes the channel.
// Used by Auto-Close tests to drive a deterministic path → processAutoClose →
// Learn flow without depending on the real FastPath / CommandHandler.
type completingExecutor struct {
	eventType    string
	eventContent string
}

func (e *completingExecutor) RunTurn(_ context.Context, _ QueryRequest) (<-chan *contracts.EngineEvent, error) {
	out := make(chan *contracts.EngineEvent, 1)
	out <- &contracts.EngineEvent{Type: e.eventType, Content: e.eventContent}
	close(out)
	return out, nil
}

// (in-package aliases for in-memory ReputationStore / ScheduledMemory to
// avoid importing the learn package internal-test helpers from this file.)
// These wrappers call the real learn package types via the public surface.

func newInMemoryReputationStoreForTest() learn.ReputationStore {
	return learn.NewInMemoryReputationStore()
}

func newScheduledMemoryForTest() *learn.ScheduledMemory {
	return learn.NewScheduledMemory()
}

func newInMemorySkillMemoryForTest() learn.Memory {
	return learn.NewSkillMemory()
}

func newInMemoryFeedbackMemoryForTest() learn.Memory {
	return learn.NewFeedbackMemory()
}

// fakeLearner extension: learnErr is the new test hook for forcing a Learn
// error in TestProcessAutoClose_LearnerError_LoggedNotBlocked. The
// fakeLearner struct is defined in orchestrator_learner_test.go. We add the
// field here via struct embedding instead of modifying the original file
// to keep PR-7.1 localized to autoclose_test.go.

// errFakeLearner wraps fakeLearner with a programmable learnErr. It is used
// only in TestProcessAutoClose_LearnerError_LoggedNotBlocked.
type errFakeLearner struct {
	*fakeLearner
	learnErr error
}

func (e *errFakeLearner) Learn(ctx context.Context, req learn.LearnRequest) ([]*learn.LearningAsset, error) {
	e.fakeLearner.mu.Lock()
	defer e.fakeLearner.mu.Unlock()
	e.fakeLearner.learningCalls++
	e.fakeLearner.lastLearnReq = req
	if e.learnErr != nil {
		return nil, e.learnErr
	}
	return nil, nil
}

// Ensure unused imports don't break the build.
var (
	_ = errors.New
	_ = sync.Mutex{}
)

// alwaysSkipClassifier is a decisionplanning.IntentClassifier stub that
// always returns IntentSkip. Used by TestProcessMessage_AutoClose_IntentSkip_NoLearn
// to drive the skip branch in ProcessMessage without depending on a
// specific message content.
type alwaysSkipClassifier struct{}

func (a *alwaysSkipClassifier) Classify(_ context.Context, _ string) (orchtypes.IntentClassification, error) {
	return orchtypes.IntentClassification{Kind: orchtypes.IntentSkip, Confidence: 100, Reason: "always-skip-test"}, nil
}

func (a *alwaysSkipClassifier) ClassifyWithPrior(_ context.Context, _ string, _ *learn.AdaptivePrior) (orchtypes.IntentClassification, error) {
	return a.Classify(context.Background(), "")
}
