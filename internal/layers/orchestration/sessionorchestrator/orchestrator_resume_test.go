// V5.6 单元测试: T2 ResumeSession 续跑入口 (DM-20260625-003, PR-V5.6)
//
// 守护 6 类 fail-safe + 2 类 terminal decision 短路:
//   - TestApplyResumeSession_NoEngine:        nil engine → fall through
//   - TestApplyResumeSession_NoPending:       resume 找到 → fall through
//   - TestApplyResumeSession_UserAccept:      B user_accept → ForceExit 短路
//   - TestApplyResumeSession_UserCancel:      C user_cancel → AbortWithAudit 短路
//   - TestApplyResumeSession_UserContinue:    A user_continue → fall through
//   - TestApplyResumeSession_ResumeError_Failsafe:  resume error → fall through
//   - TestProcessMessage_WithResume_UserAccept_EarlyClose:  端到端: 短路早退
//   - TestProcessMessage_WithResume_UserCancel_EarlyClose:  端到端: 短路早退
//
// 失败降级: resume 任意错误 → slog.Warn + fall through (不破坏主链路)
package sessionorchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/escape"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// stubResumeArbitrator is a stub HumanArbitrator-shaped resume that allows
// tests to control what ResumeSession returns. We use a real
// *escape.HumanArbitrator (PR-V5.3) so the wiring path is realistic; the
// store is shared so we can pre-seed or induce errors.
//
// For tests that need ResumeSession to return an error, we use a different
// type: the in-memory store's Load returns (zero, false, nil) on miss
// and (zero, false, err) on store-error. We construct a custom
// PendingResolutionStore shim that always errors.
type stubResume struct {
	decision escape.EscapeDecision
	found    bool
	err      error
	calls    int
}

func (s *stubResume) Save(sessionID string, d escape.EscapeDecision) error {
	s.decision = d
	return nil
}
func (s *stubResume) Load(sessionID string) (escape.EscapeDecision, bool, error) {
	s.calls++
	return s.decision, s.found, s.err
}
func (s *stubResume) Delete(sessionID string) error { return nil }

// newResumeEngineFromStore builds an EscapeEngine whose ResumeSession
// delegates to a stubPendingStore so we control found/err directly.
//
// Note: We can NOT use the in-memory store with a real HumanArbitrator
// (the HumanArbitrator takes a `*HumanArbitrator` for its resume field
// in EscapeEngine, not the raw store). To avoid pulling in the real
// HumanArbitrator, we use newFakeEscapeEngine with a custom resume.
//
// Simpler: build a fresh EscapeEngine using the real in-memory store,
// then populate the store via direct Save() and call ResumeSession via
// the engine.
func newResumeEngine(t *testing.T, store escape.PendingResolutionStore) *escape.EscapeEngine {
	t.Helper()
	if store == nil {
		// No-resume engine: returns (decision, false, nil) for all sessions.
		return escape.NewEscapeEngine(
			&stubDepthChecker{decision: escape.EscapeDecision{
				Action: escape.EscapeContinue, Reason: "no_op",
			}},
			nil,
			escape.NewCircuitBreakerSet(),
			nil,
			nil, // resume: nil → engine.ResumeSession returns (zero, false, nil)
		)
	}
	// Build a real HumanArbitrator backed by the in-memory store.
	ha := escape.NewHumanArbitrator(nil, nil, store)
	return escape.NewEscapeEngine(
		&stubDepthChecker{decision: escape.EscapeDecision{
			Action: escape.EscapeContinue, Reason: "no_op",
		}},
		nil,
		escape.NewCircuitBreakerSet(),
		nil,
		ha,
	)
}

// saveDecision is a convenience wrapper.
func saveDecision(t *testing.T, store escape.PendingResolutionStore, sessionID string, d escape.EscapeDecision) {
	t.Helper()
	if err := store.Save(sessionID, d); err != nil {
		t.Fatalf("Save: %v", err)
	}
}

// --- Test 1: nil engine → fall through ------------------------------------

func TestApplyResumeSession_NoEngine(t *testing.T) {
	orch := &SessionOrchestrator{escapeEngine: nil}
	ch, short, err := orch.applyResumeSession(context.Background(),
		orchtypes.ProcessRequest{SessionID: "sess-1"}, nil)
	if err != nil {
		t.Fatalf("err: want nil, got %v", err)
	}
	if short {
		t.Error("shortCircuit: want false (nil engine → fall through), got true")
	}
	if ch != nil {
		t.Error("ch: want nil (fall through), got non-nil")
	}
}

// --- Test 2: resume 找到 = false → fall through --------------------------

func TestApplyResumeSession_NoPending(t *testing.T) {
	store := escape.NewInMemoryPendingResolutionStore()
	engine := newResumeEngine(t, store)
	orch := &SessionOrchestrator{escapeEngine: engine}

	// No Save → Load returns (zero, false, nil) → engine.ResumeSession
	// returns (zero, false, nil) → applyResumeSession fall through.
	ch, short, err := orch.applyResumeSession(context.Background(),
		orchtypes.ProcessRequest{SessionID: "sess-2"}, nil)
	if err != nil {
		t.Fatalf("err: want nil, got %v", err)
	}
	if short {
		t.Error("shortCircuit: want false (no pending → fall through), got true")
	}
	if ch != nil {
		t.Error("ch: want nil (fall through), got non-nil")
	}
}

// --- Test 3: B user_accept → EscapeForceExit 短路 -------------------------

func TestApplyResumeSession_UserAccept(t *testing.T) {
	store := escape.NewInMemoryPendingResolutionStore()
	engine := newResumeEngine(t, store)
	orch := &SessionOrchestrator{escapeEngine: engine}

	// Pre-seed: HumanArbitrator.Arbitrate path would have stored a
	// EscapeForceExit decision. We replicate the storage shape:
	saveDecision(t, store, "sess-3", escape.EscapeDecision{
		Action:     escape.EscapeForceExit,
		Reason:     "user_accept",
		AuditLevel: 1,
		PendingID:  "p-accept",
		SessionID:  "sess-3",
		CreatedAt:  time.Now(),
	})

	ch, short, err := orch.applyResumeSession(context.Background(),
		orchtypes.ProcessRequest{SessionID: "sess-3"}, nil)
	if err != nil {
		t.Fatalf("err: want nil, got %v", err)
	}
	if !short {
		t.Fatal("shortCircuit: want true (terminal decision B), got false")
	}
	if ch == nil {
		t.Fatal("ch: want non-nil, got nil")
	}
	// Channel should emit 1 "complete" event and then close.
	events := []*contracts.EngineEvent{}
	for ev := range ch {
		events = append(events, ev)
	}
	if len(events) != 1 {
		t.Fatalf("events: want 1, got %d", len(events))
	}
	if events[0].Type != "complete" {
		t.Errorf("event.Type: want complete, got %q", events[0].Type)
	}
	if events[0].SessionID != "sess-3" {
		t.Errorf("event.SessionID: want sess-3, got %q", events[0].SessionID)
	}
	if events[0].Metadata["escape.action"] != "force_exit" {
		t.Errorf("event.Metadata[escape.action]: want force_exit, got %q", events[0].Metadata["escape.action"])
	}
	if events[0].Metadata["escape.reason"] != "user_accept" {
		t.Errorf("event.Metadata[escape.reason]: want user_accept, got %q", events[0].Metadata["escape.reason"])
	}
	if !strings.Contains(events[0].Content, "用户接受") {
		t.Errorf("event.Content: want contains '用户接受', got %q", events[0].Content)
	}
}

// --- Test 4: C user_cancel → EscapeAbortWithAudit 短路 --------------------

func TestApplyResumeSession_UserCancel(t *testing.T) {
	store := escape.NewInMemoryPendingResolutionStore()
	engine := newResumeEngine(t, store)
	orch := &SessionOrchestrator{escapeEngine: engine}

	saveDecision(t, store, "sess-4", escape.EscapeDecision{
		Action:     escape.EscapeAbortWithAudit,
		Reason:     "user_cancel",
		AuditLevel: 2,
		PendingID:  "p-cancel",
		SessionID:  "sess-4",
		CreatedAt:  time.Now(),
	})

	ch, short, err := orch.applyResumeSession(context.Background(),
		orchtypes.ProcessRequest{SessionID: "sess-4"}, nil)
	if err != nil {
		t.Fatalf("err: want nil, got %v", err)
	}
	if !short {
		t.Fatal("shortCircuit: want true (terminal decision C), got false")
	}
	if ch == nil {
		t.Fatal("ch: want non-nil, got nil")
	}
	events := []*contracts.EngineEvent{}
	for ev := range ch {
		events = append(events, ev)
	}
	if len(events) != 1 {
		t.Fatalf("events: want 1, got %d", len(events))
	}
	if events[0].Metadata["escape.action"] != "abort_with_audit" {
		t.Errorf("event.Metadata[escape.action]: want abort_with_audit, got %q", events[0].Metadata["escape.action"])
	}
	if !strings.Contains(events[0].Content, "用户取消") {
		t.Errorf("event.Content: want contains '用户取消', got %q", events[0].Content)
	}
}

// --- Test 5: A user_continue → fall through -------------------------------

func TestApplyResumeSession_UserContinue(t *testing.T) {
	store := escape.NewInMemoryPendingResolutionStore()
	engine := newResumeEngine(t, store)
	orch := &SessionOrchestrator{escapeEngine: engine}

	saveDecision(t, store, "sess-5", escape.EscapeDecision{
		Action:     escape.EscapeContinue,
		Reason:     "user_continue",
		AuditLevel: 1,
		PendingID:  "p-continue",
		SessionID:  "sess-5",
		CreatedAt:  time.Now(),
	})

	ch, short, err := orch.applyResumeSession(context.Background(),
		orchtypes.ProcessRequest{SessionID: "sess-5"}, nil)
	if err != nil {
		t.Fatalf("err: want nil, got %v", err)
	}
	if short {
		t.Error("shortCircuit: want false (user_continue → fall through), got true")
	}
	if ch != nil {
		t.Error("ch: want nil (fall through), got non-nil")
	}
}

// --- Test 6: ResumeSession error → fail-safe ------------------------------

// errStore implements escape.PendingResolutionStore that always returns
// an error on Load. Save/Delete are no-ops.
type errStore struct{}

func (errStore) Save(_ string, _ escape.EscapeDecision) error { return nil }
func (errStore) Load(_ string) (escape.EscapeDecision, bool, error) {
	return escape.EscapeDecision{}, false, errors.New("simulated store error")
}
func (errStore) Delete(_ string) error { return nil }

func TestApplyResumeSession_ResumeError_Failsafe(t *testing.T) {
	ha := escape.NewHumanArbitrator(nil, nil, errStore{})
	engine := escape.NewEscapeEngine(
		&stubDepthChecker{decision: escape.EscapeDecision{
			Action: escape.EscapeContinue, Reason: "no_op",
		}},
		nil,
		escape.NewCircuitBreakerSet(),
		nil,
		ha,
	)
	orch := &SessionOrchestrator{escapeEngine: engine}

	ch, short, err := orch.applyResumeSession(context.Background(),
		orchtypes.ProcessRequest{SessionID: "sess-6"}, nil)
	if err != nil {
		t.Fatalf("err: want nil (fail-safe should not propagate), got %v", err)
	}
	if short {
		t.Error("shortCircuit: want false (ResumeSession error → fall through), got true")
	}
	if ch != nil {
		t.Error("ch: want nil (fall through), got non-nil")
	}
}

// --- Test 7: 端到端 B user_accept → ProcessMessage 短路早退 ---------------

func TestProcessMessage_WithResume_UserAccept_EarlyClose(t *testing.T) {
	store := escape.NewInMemoryPendingResolutionStore()
	engine := newResumeEngine(t, store)

	// Save a pre-decided user_accept decision.
	saveDecision(t, store, "sess-e2e-b", escape.EscapeDecision{
		Action:     escape.EscapeForceExit,
		Reason:     "user_accept",
		AuditLevel: 1,
		PendingID:  "p-e2e-b",
		SessionID:  "sess-e2e-b",
		CreatedAt:  time.Now(),
	})

	orch := NewSessionOrchestrator(
		orchtypes.DefaultConfig(),
		&completingExecutor{eventType: "complete"},
		WithEscapeEngine(engine),
	)

	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-e2e-b",
		Message:   "hi",
	})
	if err != nil {
		t.Fatalf("ProcessMessage: want nil, got %v", err)
	}
	if ch == nil {
		t.Fatal("ProcessMessage: want non-nil channel, got nil")
	}
	// Collect events: should be exactly 1 "complete" event then close.
	events := []*contracts.EngineEvent{}
	for ev := range ch {
		events = append(events, ev)
	}
	if len(events) != 1 {
		t.Fatalf("events: want 1, got %d", len(events))
	}
	if events[0].Type != "complete" {
		t.Errorf("event.Type: want complete, got %q", events[0].Type)
	}
	if events[0].Metadata["escape.resume"] != "true" {
		t.Errorf("event.Metadata[escape.resume]: want true, got %q", events[0].Metadata["escape.resume"])
	}
}

// --- Test 8: 端到端 C user_cancel → ProcessMessage 短路早退 ---------------

func TestProcessMessage_WithResume_UserCancel_EarlyClose(t *testing.T) {
	store := escape.NewInMemoryPendingResolutionStore()
	engine := newResumeEngine(t, store)

	saveDecision(t, store, "sess-e2e-c", escape.EscapeDecision{
		Action:     escape.EscapeAbortWithAudit,
		Reason:     "user_cancel",
		AuditLevel: 2,
		PendingID:  "p-e2e-c",
		SessionID:  "sess-e2e-c",
		CreatedAt:  time.Now(),
	})

	orch := NewSessionOrchestrator(
		orchtypes.DefaultConfig(),
		&completingExecutor{eventType: "complete"},
		WithEscapeEngine(engine),
	)

	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-e2e-c",
		Message:   "hi",
	})
	if err != nil {
		t.Fatalf("ProcessMessage: want nil, got %v", err)
	}
	if ch == nil {
		t.Fatal("ProcessMessage: want non-nil channel, got nil")
	}
	events := []*contracts.EngineEvent{}
	for ev := range ch {
		events = append(events, ev)
	}
	if len(events) != 1 {
		t.Fatalf("events: want 1, got %d", len(events))
	}
	if events[0].Type != "complete" {
		t.Errorf("event.Type: want complete, got %q", events[0].Type)
	}
	if events[0].Metadata["escape.action"] != "abort_with_audit" {
		t.Errorf("event.Metadata[escape.action]: want abort_with_audit, got %q", events[0].Metadata["escape.action"])
	}
}
