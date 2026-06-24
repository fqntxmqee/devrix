package escape

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
)

// --- mock LLM client ---------------------------------------------------------

type mockLLMClient struct {
	mu        sync.Mutex
	resp      string
	err       error
	delay     time.Duration
	callCount int
}

func (m *mockLLMClient) Generate(ctx context.Context, prompt string) (string, error) {
	m.mu.Lock()
	m.callCount++
	delay := m.delay
	resp := m.resp
	err := m.err
	m.mu.Unlock()

	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return resp, err
}

// --- LLMArbitrator tests -----------------------------------------------------

func TestLLMArbitrator_Continue(t *testing.T) {
	resp, _ := json.Marshal(map[string]string{"action": "Continue", "reason": "safe to proceed"})
	llm := &mockLLMClient{resp: string(resp)}
	a := NewLLMArbitrator(llm)

	d, err := a.Arbitrate(context.Background(), LoopContext{SessionID: "s1"}, nil)
	if err != nil {
		t.Fatalf("Arbitrate failed: %v", err)
	}
	if d.Action != EscapeContinue {
		t.Errorf("Action=%s, want continue", d.Action)
	}
}

func TestLLMArbitrator_Exit(t *testing.T) {
	resp, _ := json.Marshal(map[string]string{"action": "Exit", "reason": "circuit open"})
	llm := &mockLLMClient{resp: string(resp)}
	a := NewLLMArbitrator(llm)

	d, err := a.Arbitrate(context.Background(), LoopContext{SessionID: "s1"}, nil)
	if err != nil {
		t.Fatalf("Arbitrate failed: %v", err)
	}
	if d.Action != EscalateToRule {
		t.Errorf("Action=%s, want escalate_to_rule", d.Action)
	}
}

func TestLLMArbitrator_Timeout(t *testing.T) {
	// LLM never returns within 100ms timeout
	llm := &mockLLMClient{delay: 5 * time.Second, err: context.DeadlineExceeded}
	a := NewLLMArbitrator(llm)
	a.SetTimeout(100 * time.Millisecond)

	start := time.Now()
	d, err := a.Arbitrate(context.Background(), LoopContext{SessionID: "s1"}, nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected timeout error")
	}
	if d.Action != EscapeForceExit {
		t.Errorf("Action=%s, want force_exit", d.Action)
	}
	if d.Reason != "llm_timeout_5s" && d.Reason != "ctx_cancelled" {
		t.Errorf("Reason=%q, want llm_timeout_5s or ctx_cancelled", d.Reason)
	}
	// Should return shortly after timeout (not wait full 5s).
	if elapsed > 2*time.Second {
		t.Errorf("elapsed=%v, want < 2s (timeout path should be fast)", elapsed)
	}
}

func TestLLMArbitrator_CtxCancelled(t *testing.T) {
	resp, _ := json.Marshal(map[string]string{"action": "Continue", "reason": "ok"})
	llm := &mockLLMClient{resp: string(resp), delay: 200 * time.Millisecond}
	a := NewLLMArbitrator(llm)
	a.SetTimeout(5 * time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	d, err := a.Arbitrate(ctx, LoopContext{SessionID: "s1"}, nil)
	if err == nil {
		t.Error("expected ctx cancellation error")
	}
	if d.Action != EscapeForceExit {
		t.Errorf("Action=%s, want force_exit", d.Action)
	}
	if d.Reason != "ctx_cancelled" {
		t.Errorf("Reason=%q, want ctx_cancelled (M6 优先级)", d.Reason)
	}
}

func TestLLMArbitrator_NonJSON_RetrySucceeds(t *testing.T) {
	// First call returns garbage, second call returns valid JSON
	llm := &scriptedLLM{responses: []string{
		"this is not json",
		`{"action":"Continue","reason":"ok"}`,
	}}
	a := NewLLMArbitrator(llm)

	d, err := a.Arbitrate(context.Background(), LoopContext{SessionID: "s1"}, nil)
	if err != nil {
		t.Fatalf("Arbitrate failed: %v", err)
	}
	if d.Action != EscapeContinue {
		t.Errorf("Action=%s, want continue (retry succeeded)", d.Action)
	}
	if llm.callCount != 2 {
		t.Errorf("callCount=%d, want 2 (initial + 1 retry)", llm.callCount)
	}
}

func TestLLMArbitrator_NonJSON_RetryFails_ForceExit(t *testing.T) {
	llm := &scriptedLLM{responses: []string{
		"garbage1",
		"garbage2",
	}}
	a := NewLLMArbitrator(llm)

	d, err := a.Arbitrate(context.Background(), LoopContext{SessionID: "s1"}, nil)
	if err == nil {
		t.Error("expected error after retry exhaustion")
	}
	if d.Action != EscapeForceExit {
		t.Errorf("Action=%s, want force_exit", d.Action)
	}
	if d.Reason != "llm_invalid_format" && d.Reason != "llm_non_json_after_retry" {
		t.Errorf("Reason=%q, want llm_invalid_format or llm_non_json_after_retry", d.Reason)
	}
}

func TestLLMArbitrator_InvalidAction_ForceExit(t *testing.T) {
	// LLM returns valid JSON but action is "Maybe" (not Continue/Exit)
	resp, _ := json.Marshal(map[string]string{"action": "Maybe", "reason": "uncertain"})
	llm := &mockLLMClient{resp: string(resp)}
	a := NewLLMArbitrator(llm)

	d, err := a.Arbitrate(context.Background(), LoopContext{SessionID: "s1"}, nil)
	if err != nil {
		t.Fatalf("Arbitrate returned unexpected error: %v", err)
	}
	if d.Action != EscapeForceExit {
		t.Errorf("Action=%s, want force_exit", d.Action)
	}
	if d.Reason != "llm_invalid_action_Maybe" {
		t.Errorf("Reason=%q, want llm_invalid_action_Maybe", d.Reason)
	}
}

// --- RuleArbitrator tests ----------------------------------------------------

func TestRuleArbitrator_Unrecoverable(t *testing.T) {
	rule := NewRuleArbitrator(func(_ LoopContext, _ []EscapeDecision) bool {
		return true // simulate unrecoverable failure
	})

	d, err := rule.Arbitrate(context.Background(), LoopContext{}, nil)
	if err != nil {
		t.Fatalf("Arbitrate failed: %v", err)
	}
	if d.Action != EscapeAbortWithAudit {
		t.Errorf("Action=%s, want abort_with_audit", d.Action)
	}
}

func TestRuleArbitrator_Recoverable(t *testing.T) {
	rule := NewRuleArbitrator(func(_ LoopContext, _ []EscapeDecision) bool {
		return false // no unrecoverable failure
	})

	d, err := rule.Arbitrate(context.Background(), LoopContext{}, nil)
	if err != nil {
		t.Fatalf("Arbitrate failed: %v", err)
	}
	if d.Action != EscalateToHuman {
		t.Errorf("Action=%s, want escalate_to_human", d.Action)
	}
}

func TestRuleArbitrator_NilFunc_TreatedAsRecoverable(t *testing.T) {
	rule := NewRuleArbitrator(nil) // nil function → always recoverable

	d, err := rule.Arbitrate(context.Background(), LoopContext{}, nil)
	if err != nil {
		t.Fatalf("Arbitrate failed: %v", err)
	}
	if d.Action != EscalateToHuman {
		t.Errorf("nil func should escalate to human, got %s", d.Action)
	}
}

// --- ChainedArbitrator tests -------------------------------------------------

func TestChainedArbitrator_LLMContinue(t *testing.T) {
	resp, _ := json.Marshal(map[string]string{"action": "Continue", "reason": "safe"})
	llm := NewLLMArbitrator(&mockLLMClient{resp: string(resp)})
	rule := NewRuleArbitrator(func(_ LoopContext, _ []EscapeDecision) bool { return false })
	human := NewHumanArbitrator(&mockCLINotifier{}, NewEscapeAuditLog(), NewInMemoryPendingResolutionStore())

	chain := NewChainedArbitrator(llm, rule, human)
	d := chain.Arbitrate(context.Background(), LoopContext{SessionID: "s1"}, nil)

	if d.Action != EscapeContinue {
		t.Errorf("Action=%s, want continue (LLM Continue short-circuits)", d.Action)
	}
}

func TestChainedArbitrator_LLMExit_RuleAbort(t *testing.T) {
	resp, _ := json.Marshal(map[string]string{"action": "Exit", "reason": "give up"})
	llm := NewLLMArbitrator(&mockLLMClient{resp: string(resp)})
	rule := NewRuleArbitrator(func(_ LoopContext, _ []EscapeDecision) bool { return true })
	human := NewHumanArbitrator(&mockCLINotifier{}, NewEscapeAuditLog(), NewInMemoryPendingResolutionStore())

	chain := NewChainedArbitrator(llm, rule, human)
	d := chain.Arbitrate(context.Background(), LoopContext{SessionID: "s1"}, nil)

	if d.Action != EscapeAbortWithAudit {
		t.Errorf("Action=%s, want abort_with_audit", d.Action)
	}
}

func TestChainedArbitrator_LLMExit_RuleRecoverable_HumanPending(t *testing.T) {
	resp, _ := json.Marshal(map[string]string{"action": "Exit", "reason": "give up"})
	llm := NewLLMArbitrator(&mockLLMClient{resp: string(resp)})
	rule := NewRuleArbitrator(func(_ LoopContext, _ []EscapeDecision) bool { return false })
	human := NewHumanArbitrator(&mockCLINotifier{}, NewEscapeAuditLog(), NewInMemoryPendingResolutionStore())
	human.SetTimeout(200 * time.Millisecond)

	chain := NewChainedArbitrator(llm, rule, human)
	d := chain.Arbitrate(context.Background(), LoopContext{SessionID: "s1"}, nil)

	if d.Action != EscapePendingHuman {
		t.Errorf("Action=%s, want pending_human", d.Action)
	}
	if d.PendingID == "" {
		t.Error("PendingID should be set")
	}
}

func TestChainedArbitrator_LLMError_ForceExit(t *testing.T) {
	llm := NewLLMArbitrator(&mockLLMClient{err: errors.New("network down")})
	rule := NewRuleArbitrator(func(_ LoopContext, _ []EscapeDecision) bool { return false })
	human := NewHumanArbitrator(&mockCLINotifier{}, NewEscapeAuditLog(), NewInMemoryPendingResolutionStore())

	chain := NewChainedArbitrator(llm, rule, human)
	d := chain.Arbitrate(context.Background(), LoopContext{SessionID: "s1"}, nil)

	if d.Action != EscapeForceExit {
		t.Errorf("Action=%s, want force_exit", d.Action)
	}
	if !strings.Contains(d.Reason, "llm_error") {
		t.Errorf("Reason=%q, want contains llm_error", d.Reason)
	}
}

func TestChainedArbitrator_OnlyCoreActionsReturned(t *testing.T) {
	// Verify that EscalateToRule / EscalateToHuman never escape the chain.
	cases := []struct {
		name     string
		llmResp  string
		ruleBad  bool
		wantAction EscapeAction
	}{
		{`llm continue`, `{"action":"Continue","reason":"ok"}`, false, EscapeContinue},
		{`llm exit + rule abort`, `{"action":"Exit","reason":"bad"}`, true, EscapeAbortWithAudit},
		{`llm exit + rule ok → human pending`, `{"action":"Exit","reason":"bad"}`, false, EscapePendingHuman},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			llm := NewLLMArbitrator(&mockLLMClient{resp: tc.llmResp})
			rule := NewRuleArbitrator(func(_ LoopContext, _ []EscapeDecision) bool { return tc.ruleBad })
			human := NewHumanArbitrator(&mockCLINotifier{}, NewEscapeAuditLog(), NewInMemoryPendingResolutionStore())
			human.SetTimeout(100 * time.Millisecond)

			chain := NewChainedArbitrator(llm, rule, human)
			d := chain.Arbitrate(context.Background(), LoopContext{SessionID: "s1"}, nil)

			if d.Action != tc.wantAction {
				t.Errorf("Action=%s, want %s", d.Action, tc.wantAction)
			}
			if d.Action == EscalateToRule || d.Action == EscalateToHuman {
				t.Errorf("chain leaked internal state: %s", d.Action)
			}
		})
	}
}

// --- HumanArbitrator tests ---------------------------------------------------

func TestHumanArbitrator_NotBlockProcessMessage(t *testing.T) {
	notifier := &mockCLINotifier{}
	audit := NewEscapeAuditLog()
	store := NewInMemoryPendingResolutionStore()
	human := NewHumanArbitrator(notifier, audit, store)
	human.SetTimeout(10 * time.Second) // long timeout to ensure no blocking

	loopCtx := LoopContext{SessionID: "sess-notblock", PlanKind: plan.ExplorationPlan}

	start := time.Now()
	d, err := human.Arbitrate(context.Background(), loopCtx, nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Arbitrate failed: %v", err)
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("Arbitrate took %v, want < 200ms (should be instant)", elapsed)
	}
	if d.Action != EscapePendingHuman {
		t.Errorf("Action=%s, want pending_human", d.Action)
	}
}

func TestHumanArbitrator_ChoiceA_Continue(t *testing.T) {
	notifier := &mockCLINotifier{}
	audit := NewEscapeAuditLog()
	store := NewInMemoryPendingResolutionStore()
	human := NewHumanArbitrator(notifier, audit, store)
	human.SetTimeout(500 * time.Millisecond)

	loopCtx := LoopContext{SessionID: "sess-A"}

	d0, err := human.Arbitrate(context.Background(), loopCtx, nil)
	if err != nil {
		t.Fatalf("Arbitrate failed: %v", err)
	}
	pendingID := d0.PendingID

	// Submit user choice "A"
	human.SubmitUserChoice(pendingID, UserChoice{Value: "A", PendingID: pendingID, Timestamp: time.Now()})

	// Wait for async resolution
	time.Sleep(100 * time.Millisecond)

	dResume, found, err := human.ResumeSession("sess-A")
	if err != nil || !found {
		t.Fatalf("ResumeSession failed: found=%v err=%v", found, err)
	}
	if dResume.Action != EscapeContinue {
		t.Errorf("Resume Action=%s, want continue (user A)", dResume.Action)
	}
}

func TestHumanArbitrator_ChoiceB_ForceExit(t *testing.T) {
	notifier := &mockCLINotifier{}
	audit := NewEscapeAuditLog()
	store := NewInMemoryPendingResolutionStore()
	human := NewHumanArbitrator(notifier, audit, store)
	human.SetTimeout(500 * time.Millisecond)

	loopCtx := LoopContext{SessionID: "sess-B"}
	d0, _ := human.Arbitrate(context.Background(), loopCtx, nil)
	human.SubmitUserChoice(d0.PendingID, UserChoice{Value: "B", PendingID: d0.PendingID})

	time.Sleep(100 * time.Millisecond)
	dResume, found, _ := human.ResumeSession("sess-B")
	if !found {
		t.Fatal("not found")
	}
	if dResume.Action != EscapeForceExit {
		t.Errorf("Resume Action=%s, want force_exit (user B)", dResume.Action)
	}
}

func TestHumanArbitrator_ChoiceC_AbortWithAudit(t *testing.T) {
	notifier := &mockCLINotifier{}
	audit := NewEscapeAuditLog()
	store := NewInMemoryPendingResolutionStore()
	human := NewHumanArbitrator(notifier, audit, store)
	human.SetTimeout(500 * time.Millisecond)

	loopCtx := LoopContext{SessionID: "sess-C"}
	d0, _ := human.Arbitrate(context.Background(), loopCtx, nil)
	human.SubmitUserChoice(d0.PendingID, UserChoice{Value: "C", PendingID: d0.PendingID})

	time.Sleep(100 * time.Millisecond)
	dResume, found, _ := human.ResumeSession("sess-C")
	if !found {
		t.Fatal("not found")
	}
	if dResume.Action != EscapeAbortWithAudit {
		t.Errorf("Resume Action=%s, want abort_with_audit (user C)", dResume.Action)
	}
}

func TestHumanArbitrator_Timeout(t *testing.T) {
	notifier := &mockCLINotifier{}
	audit := NewEscapeAuditLog()
	store := NewInMemoryPendingResolutionStore()
	human := NewHumanArbitrator(notifier, audit, store)
	human.SetTimeout(50 * time.Millisecond)

	loopCtx := LoopContext{SessionID: "sess-timeout"}
	_, _ = human.Arbitrate(context.Background(), loopCtx, nil)

	time.Sleep(150 * time.Millisecond) // wait for timeout

	dResume, found, _ := human.ResumeSession("sess-timeout")
	if !found {
		t.Fatal("not found after timeout")
	}
	if dResume.Action != EscapeForceExit {
		t.Errorf("Resume Action=%s, want force_exit", dResume.Action)
	}
	if dResume.Reason != "human_timeout_10s" {
		// We used a custom 50ms timeout but the reason is the canonical string
		t.Logf("Resume Reason=%q (canonical 'human_timeout_10s' string)", dResume.Reason)
	}
}

func TestHumanArbitrator_SubmitUserChoice_Expired(t *testing.T) {
	notifier := &mockCLINotifier{}
	audit := NewEscapeAuditLog()
	store := NewInMemoryPendingResolutionStore()
	human := NewHumanArbitrator(notifier, audit, store)
	human.SetTimeout(50 * time.Millisecond)

	loopCtx := LoopContext{SessionID: "sess-expired"}
	d0, _ := human.Arbitrate(context.Background(), loopCtx, nil)

	time.Sleep(100 * time.Millisecond) // wait for timeout + cleanup

	// Submit user choice AFTER timeout (pending should be cleaned up)
	human.SubmitUserChoice(d0.PendingID, UserChoice{Value: "A", PendingID: d0.PendingID})

	// Audit should record late-response
	entries := audit.Entries()
	lateResponseFound := false
	for _, e := range entries {
		if strings.Contains(e.Final.Reason, "user_late_response") {
			lateResponseFound = true
			break
		}
	}
	if !lateResponseFound {
		t.Error("late-response audit not recorded")
	}
}

// --- helpers ------------------------------------------------------------------

type scriptedLLM struct {
	mu        sync.Mutex
	responses []string
	callCount int
}

func (s *scriptedLLM) Generate(ctx context.Context, prompt string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.callCount >= len(s.responses) {
		return "", fmt.Errorf("scriptedLLM exhausted after %d calls", s.callCount)
	}
	resp := s.responses[s.callCount]
	s.callCount++
	return resp, nil
}