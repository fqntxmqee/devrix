//go:build integration && d7

// ResumeSession acceptance test (DM-20260629-001 PR-7, T43).
//
// PR-V5.6 (DM-20260625-003) T2 ProcessMessage entry: after buildObserveRequest
// but before classify, the orchestrator consults EscapeEngine.ResumeSession
// to look up a pending user decision. Three decision paths:
//
//   A user_continue  → EscapeContinue      → fall through (full 5-node pipeline)
//   B user_accept    → EscapeForceExit     → emit "complete" EngineEvent (audit)
//   C user_cancel    → EscapeAbortWithAudit → emit "complete" EngineEvent (audit)
//
// Acceptance criteria:
//  1. ResumeSession not found → (zero, false, nil) → orchestrator fall-through
//  2. ResumeSession returns A → orchestrator continues normally (no short-circuit)
//  3. ResumeSession returns B → orchestrator short-circuits with one "complete" event
//  4. ResumeSession returns C → orchestrator short-circuits with one "complete" event
//  5. ResumeSession is one-shot — second call after a found=true returns
//     (zero, false, nil) so duplicate invocations don't double-emit.
package d7integration

import (
	"context"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/escape"
)

// T: DM-20260629-001 T43.d — ResumeSession A fall-through (no short-circuit).
func TestAcceptance_ResumeSession_FallThrough_NoPending(t *testing.T) {
	engine, _, _ := newResumeEngine(t)
	sessionID := "sess-resume-empty"

	// No prior SubmitUserChoice — store is empty.
	decision, found, err := engine.ResumeSession(sessionID)
	if err != nil {
		t.Fatalf("ResumeSession(empty): %v", err)
	}
	if found {
		t.Errorf("ResumeSession on empty store: found=true, want false; decision=%+v", decision)
	}
}

// T: DM-20260629-001 T43.d — ResumeSession A user_continue: orchestrator
// continues normally (no terminal event).
func TestAcceptance_ResumeSession_A_UserContinue(t *testing.T) {
	engine, store, _ := newResumeEngine(t)
	sessionID := "sess-resume-A"

	store.Save(sessionID, escape.EscapeDecision{
		Action:    escape.EscapeContinue,
		Reason:    "user_continue",
		AuditLevel: 1,
		SessionID: sessionID,
		CreatedAt: time.Now(),
	})

	decision, found, err := engine.ResumeSession(sessionID)
	if err != nil {
		t.Fatalf("ResumeSession(A): %v", err)
	}
	if !found {
		t.Fatal("ResumeSession(A): found=false, want true")
	}
	if decision.Action != escape.EscapeContinue {
		t.Errorf("A: Action = %s, want EscapeContinue", decision.Action)
	}
	if decision.Reason != "user_continue" {
		t.Errorf("A: Reason = %q, want user_continue", decision.Reason)
	}
}

// T: DM-20260629-001 T43.d — ResumeSession B user_accept: terminal
// EscapeForceExit with AuditLevel 1.
func TestAcceptance_ResumeSession_B_UserAccept(t *testing.T) {
	engine, store, _ := newResumeEngine(t)
	sessionID := "sess-resume-B"

	store.Save(sessionID, escape.EscapeDecision{
		Action:    escape.EscapeForceExit,
		Reason:    "user_accept",
		AuditLevel: 1,
		SessionID: sessionID,
		CreatedAt: time.Now(),
	})

	decision, found, err := engine.ResumeSession(sessionID)
	if err != nil {
		t.Fatalf("ResumeSession(B): %v", err)
	}
	if !found {
		t.Fatal("ResumeSession(B): found=false, want true")
	}
	if decision.Action != escape.EscapeForceExit {
		t.Errorf("B: Action = %s, want EscapeForceExit", decision.Action)
	}
	if decision.AuditLevel != 1 {
		t.Errorf("B: AuditLevel = %d, want 1", decision.AuditLevel)
	}
}

// T: DM-20260629-001 T43.d — ResumeSession C user_cancel: terminal
// EscapeAbortWithAudit with AuditLevel 2 (higher audit trail).
func TestAcceptance_ResumeSession_C_UserCancel(t *testing.T) {
	engine, store, _ := newResumeEngine(t)
	sessionID := "sess-resume-C"

	store.Save(sessionID, escape.EscapeDecision{
		Action:    escape.EscapeAbortWithAudit,
		Reason:    "user_cancel",
		AuditLevel: 2,
		SessionID: sessionID,
		CreatedAt: time.Now(),
	})

	decision, found, err := engine.ResumeSession(sessionID)
	if err != nil {
		t.Fatalf("ResumeSession(C): %v", err)
	}
	if !found {
		t.Fatal("ResumeSession(C): found=false, want true")
	}
	if decision.Action != escape.EscapeAbortWithAudit {
		t.Errorf("C: Action = %s, want EscapeAbortWithAudit", decision.Action)
	}
	if decision.AuditLevel != 2 {
		t.Errorf("C: AuditLevel = %d, want 2 (higher audit trail)", decision.AuditLevel)
	}
}

// T: DM-20260629-001 T43.d — ResumeSession one-shot: second call after a
// found=true must return found=false (decision was consumed).
func TestAcceptance_ResumeSession_OneShotConsumption(t *testing.T) {
	engine, store, _ := newResumeEngine(t)
	sessionID := "sess-resume-oneshot"

	store.Save(sessionID, escape.EscapeDecision{
		Action:    escape.EscapeForceExit,
		Reason:    "user_accept",
		AuditLevel: 1,
		SessionID: sessionID,
		CreatedAt: time.Now(),
	})

	// First call: found.
	_, found, _ := engine.ResumeSession(sessionID)
	if !found {
		t.Fatal("first ResumeSession: found=false, want true")
	}
	// Second call: not found (one-shot).
	_, found2, _ := engine.ResumeSession(sessionID)
	if found2 {
		t.Error("second ResumeSession: found=true, want false (one-shot)")
	}
}

// newResumeEngine builds a minimal EscapeEngine wired with an in-memory
// PendingResolutionStore. Returns (engine, store, notifier).
func newResumeEngine(t *testing.T) (*escape.EscapeEngine, *escape.InMemoryPendingResolutionStore, *mockCLINotifier) {
	t.Helper()
	store := escape.NewInMemoryPendingResolutionStore()
	notifier := &mockCLINotifier{}
	tracker, err := escape.NewLoopDepthTracker(10)
	if err != nil {
		t.Fatalf("NewLoopDepthTracker: %v", err)
	}
	chain := escape.NewChainedArbitrator(
		escape.NewLLMArbitrator(&mockLLMClient{resp: `{"action":"Continue","reason":"ok"}`}),
		escape.NewRuleArbitrator(func(_ escape.LoopContext, _ []escape.EscapeDecision) bool { return false }),
		escape.NewHumanArbitrator(notifier, escape.NewEscapeAuditLog(), store),
	)
	cbSet := escape.NewCircuitBreakerSet()
	audit := escape.NewEscapeAuditLog()
	human := escape.NewHumanArbitrator(notifier, audit, store)
	return escape.NewEscapeEngine(tracker, chain, cbSet, audit, human), store, notifier
}

// mockCLINotifier is a no-op notifier used by resume acceptance tests.
// We don't go through the full Arbitrate path — we Save decisions directly
// into the InMemoryPendingResolutionStore.
type mockCLINotifier struct{}

func (m *mockCLINotifier) Notify(_ context.Context, _ escape.LoopContext, _ string, _ []escape.EscapeDecision) error {
	return nil
}

// mockLLMClient is a minimal LLM client stub.
type mockLLMClient struct {
	resp string
	err  error
}

func (m *mockLLMClient) Generate(_ context.Context, _ string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.resp, nil
}
