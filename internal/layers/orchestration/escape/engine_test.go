package escape

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
)

// engineFixture builds a fully-wired EscapeEngine for tests.
type engineFixture struct {
	tracker   *LoopDepthTracker
	chain     *ChainedArbitrator
	cbSet     *CircuitBreakerSet
	audit     *EscapeAuditLog
	resume    *HumanArbitrator
	notifier  *mockCLINotifier
	llmClient *mockLLMClient
}

func newEngineFixture(t *testing.T) *engineFixture {
	t.Helper()
	tracker, err := NewLoopDepthTracker(DefaultMaxDepth)
	if err != nil {
		t.Fatalf("NewLoopDepthTracker: %v", err)
	}

	// Share one mock between LLMArbitrator and the fixture so tests can
	// set llmClient.resp to steer chain output.
	llmMock := &mockLLMClient{}
	llm := NewLLMArbitrator(llmMock)
	rule := NewRuleArbitrator(nil)
	audit := NewEscapeAuditLog()
	store := NewInMemoryPendingResolutionStore()
	notifier := &mockCLINotifier{}
	human := NewHumanArbitrator(notifier, audit, store)
	human.SetTimeout(50 * time.Millisecond)

	chain := NewChainedArbitrator(llm, rule, human)
	cbSet := NewCircuitBreakerSet()

	return &engineFixture{
		tracker:   tracker,
		chain:     chain,
		cbSet:     cbSet,
		audit:     audit,
		resume:    human,
		notifier:  notifier,
		llmClient: llmMock,
	}
}

// --- TestEscapeEngine_AllContinue -------------------------------------------

func TestEscapeEngine_AllContinue(t *testing.T) {
	f := newEngineFixture(t)
	engine := NewEscapeEngine(f.tracker, f.chain, f.cbSet, f.audit, f.resume)

	loopCtx := LoopContext{
		SessionID: "sess-all-continue",
		PlanKind:  plan.ExplorationPlan,
	}

	d := engine.Evaluate(context.Background(), loopCtx)
	if d.Action != EscapeContinue {
		t.Errorf("all-continue: Action=%s, want continue", d.Action)
	}
	if d.Reason != "all_depth_limits_passed" {
		t.Errorf("Reason=%q, want all_depth_limits_passed", d.Reason)
	}
}

// --- TestEscapeEngine_LoopDepthExceeded --------------------------------------

func TestEscapeEngine_LoopDepthExceeded(t *testing.T) {
	f := newEngineFixture(t)
	// Mock LLM Continue so chain short-circuits (avoids async timeout).
	resp, _ := json.Marshal(map[string]string{"action": "Continue", "reason": "go"})
	f.llmClient.resp = string(resp)

	engine := NewEscapeEngine(f.tracker, f.chain, f.cbSet, f.audit, f.resume)
	loopCtx := LoopContext{
		SessionID:        "sess-depth",
		PlanKind:         plan.ExplorationPlan,
		ObservationKind:  1,
		FailureCriterion: "timeout",
		ArtifactType:     1,
	}

	// Force tracker to ForceExit (depth=3 → ForceExit per design §5.1)
	for i := 0; i < 3; i++ {
		f.tracker.ShouldContinue(loopCtx)
	}

	d := engine.Evaluate(context.Background(), loopCtx)
	if d.Action != EscapeForceExit {
		t.Errorf("depth exceeded: Action=%s, want force_exit", d.Action)
	}
	if !strings.Contains(d.Reason, "loop_depth_exceeded") && !strings.Contains(d.Reason, "user_continue") {
		t.Errorf("Reason=%q, want contains loop_depth_exceeded or user_continue", d.Reason)
	}

	// Audit should record the ForceExit
	if f.audit.Len() == 0 {
		t.Error("audit log should record ForceExit")
	}
}

// --- TestEscapeEngine_CircuitBreakerOpen --------------------------------------

func TestEscapeEngine_CircuitBreakerOpen(t *testing.T) {
	f := newEngineFixture(t)
	// Mock LLM Continue so chain short-circuits.
	resp, _ := json.Marshal(map[string]string{"action": "Continue", "reason": "go"})
	f.llmClient.resp = string(resp)

	engine := NewEscapeEngine(f.tracker, f.chain, f.cbSet, f.audit, f.resume)

	// Open L0 AnomalyDetectorCB
	for i := 0; i < 5; i++ {
		f.cbSet.L0.RecordFailure()
	}

	d := engine.Evaluate(context.Background(), LoopContext{SessionID: "sess-cb-open", PlanKind: plan.ProtocolPlan})

	if d.Action != EscapeForceExit {
		t.Errorf("CB open: Action=%s, want force_exit", d.Action)
	}
	if !strings.Contains(d.Reason, "circuit_breaker") {
		t.Errorf("Reason=%q, want contains circuit_breaker", d.Reason)
	}
}

// --- TestEscapeEngine_BothDepthAndCB ----------------------------------------

func TestEscapeEngine_BothDepthAndCB(t *testing.T) {
	f := newEngineFixture(t)
	resp, _ := json.Marshal(map[string]string{"action": "Continue", "reason": "go"})
	f.llmClient.resp = string(resp)

	engine := NewEscapeEngine(f.tracker, f.chain, f.cbSet, f.audit, f.resume)
	loopCtx := LoopContext{
		SessionID:        "sess-both",
		PlanKind:         plan.ExplorationPlan,
		ObservationKind:  1,
		FailureCriterion: "timeout",
		ArtifactType:     1,
	}

	// Open both depth (3 calls) AND CB L4 (1 panic)
	for i := 0; i < 3; i++ {
		f.tracker.ShouldContinue(loopCtx)
	}
	f.cbSet.L4.RecordFailure()

	d := engine.Evaluate(context.Background(), loopCtx)

	// 任一非 Continue → 仲裁 (最终决策由 LLM mock 决定 = Continue)
	if d.Action != EscapeContinue && d.Action != EscapeForceExit {
		t.Errorf("Action=%s, want continue or force_exit", d.Action)
	}
}

// --- TestEscapeEngine_PanicRecovery -----------------------------------------

func TestEscapeEngine_PanicRecovery(t *testing.T) {
	f := newEngineFixture(t)

	// Replace tracker with one that panics.
	panicTracker := &panickingTracker{}
	engine := NewEscapeEngine(panicTracker, f.chain, f.cbSet, f.audit, f.resume)

	d := engine.Evaluate(context.Background(), LoopContext{SessionID: "sess-panic"})

	if d.Action != EscapeContinue {
		t.Errorf("panic recovery: Action=%s, want continue", d.Action)
	}
	if d.Reason != "escape_engine_panic_recovered" {
		t.Errorf("Reason=%q, want escape_engine_panic_recovered", d.Reason)
	}
}

type panickingTracker struct{}

func (p *panickingTracker) ShouldContinue(ctx LoopContext) EscapeDecision {
	panic("simulated tracker panic")
}

// --- TestEscapeEngine_ResumeSession_Delegates --------------------------------

func TestEscapeEngine_ResumeSession_Delegates(t *testing.T) {
	f := newEngineFixture(t)
	engine := NewEscapeEngine(f.tracker, f.chain, f.cbSet, f.audit, f.resume)

	// No pending decision → not found
	_, found, err := engine.ResumeSession("sess-not-found")
	if err != nil {
		t.Fatalf("ResumeSession failed: %v", err)
	}
	if found {
		t.Error("ResumeSession on unknown session: found=true, want false")
	}
}

// --- TestEscapeEngine_NilEngine_DoesNotPanic --------------------------------

func TestEscapeEngine_NilEngine_NoCrash(t *testing.T) {
	// Pass nil fields; engine should still work (degraded).
	tracker, _ := NewLoopDepthTracker(DefaultMaxDepth)
	llm := NewLLMArbitrator(&mockLLMClient{})
	rule := NewRuleArbitrator(nil)
	human := NewHumanArbitrator(&mockCLINotifier{}, NewEscapeAuditLog(), NewInMemoryPendingResolutionStore())
	human.SetTimeout(50 * time.Millisecond)
	chain := NewChainedArbitrator(llm, rule, human)
	cbSet := NewCircuitBreakerSet()

	// audit = nil, resume = nil
	engine := NewEscapeEngine(tracker, chain, cbSet, nil, nil)
	d := engine.Evaluate(context.Background(), LoopContext{SessionID: "s1"})
	if d.Action != EscapeContinue {
		t.Errorf("nil-audit/resume: Action=%s, want continue", d.Action)
	}
}

// --- TestEscapeEngine_4DepthLimits_Coordination (L2 integration) -------------

func TestEscapeEngine_4DepthLimits_Coordination(t *testing.T) {
	f := newEngineFixture(t)
	// Mock LLM Continue (avoids async timeout)
	resp, _ := json.Marshal(map[string]string{"action": "Continue", "reason": "go"})
	f.llmClient.resp = string(resp)

	engine := NewEscapeEngine(f.tracker, f.chain, f.cbSet, f.audit, f.resume)
	loopCtx := LoopContext{
		SessionID:        "sess-4depth",
		PlanKind:         plan.ExplorationPlan,
		ObservationKind:  1,
		FailureCriterion: "timeout",
		ArtifactType:     1,
	}

	// 回路深度耗尽 + L0 open
	for i := 0; i < 3; i++ {
		f.tracker.ShouldContinue(loopCtx)
	}
	for i := 0; i < 5; i++ {
		f.cbSet.L0.RecordFailure()
	}

	d := engine.Evaluate(context.Background(), loopCtx)

	// Both non-Continue → chained to LLM. LLM Continue → return Continue.
	if d.Action != EscapeContinue {
		t.Errorf("4-depth coordination: Action=%s, want continue (LLM Continue)", d.Action)
	}

	// Verify audit captured the chain (upstream decisions recorded)
	if f.audit.Len() == 0 {
		t.Error("audit should record the 4-depth coordination chain")
	}
}

// --- helper to silence unused warning ----------------------------------------

var _ = errors.New

// --- PR-B (DM-20260629-008): NotifyPessimistic tests ------------------------

// TestEscapeEngine_NotifyPessimistic_NilGuard — nil guard = no-op pass-through.
func TestEscapeEngine_NotifyPessimistic_NilGuard(t *testing.T) {
	tracker, _ := NewLoopDepthTracker(DefaultMaxDepth)
	chain := NewChainedArbitrator(nil, nil, nil)
	engine := NewEscapeEngine(tracker, chain, NewCircuitBreakerSet(), nil, nil)
	// No SetPessimisticGuard call.
	report, _ := interfaces.NewTaskReport("ts_np_nil")
	updated, err := engine.NotifyPessimistic(nil, report)
	if err != nil {
		t.Fatalf("NotifyPessimistic: %v", err)
	}
	if updated != report {
		t.Errorf("nil-guard must return same report pointer")
	}
	if updated.FallbackUsed {
		t.Error("FallbackUsed should be false (guard not wired)")
	}
}

// TestEscapeEngine_NotifyPessimistic_DisabledGuard — wired but
// Feature Flag off = no-op pass-through.
func TestEscapeEngine_NotifyPessimistic_DisabledGuard(t *testing.T) {
	tracker, _ := NewLoopDepthTracker(DefaultMaxDepth)
	chain := NewChainedArbitrator(nil, nil, nil)
	engine := NewEscapeEngine(tracker, chain, NewCircuitBreakerSet(), nil, nil)
	guard := NewDefaultPessimisticCommitGuard() // Enabled = false
	engine.SetPessimisticGuard(guard)

	report, _ := interfaces.NewTaskReport("ts_np_off")
	updated, err := engine.NotifyPessimistic(nil, report)
	if err != nil {
		t.Fatalf("NotifyPessimistic: %v", err)
	}
	if updated.FallbackUsed {
		t.Error("FallbackUsed should be false (guard disabled)")
	}
	if updated.MVPArtifact != nil {
		t.Error("MVPArtifact should be nil (guard disabled)")
	}
}

// TestEscapeEngine_NotifyPessimistic_Enabled_TriggersCommit — guard
// enabled + resource exhausted → Pessimistic Commit fires, MVPArtifact
// populated, Result.Kind forced to Indeterminate.
func TestEscapeEngine_NotifyPessimistic_Enabled_TriggersCommit(t *testing.T) {
	tracker, _ := NewLoopDepthTracker(DefaultMaxDepth)
	chain := NewChainedArbitrator(nil, nil, nil)
	engine := NewEscapeEngine(tracker, chain, NewCircuitBreakerSet(), nil, nil)
	guard := NewDefaultPessimisticCommitGuard()
	guard.Enabled = true
	engine.SetPessimisticGuard(guard)

	report, _ := interfaces.NewTaskReport("ts_np_on")
	report, _ = report.WithResource(interfaces.Resource{
		TokensUsed:   990,
		TokensBudget: 1000, // only 10 left, < 10% reserve (100) → exhausted
	})
	updated, err := engine.NotifyPessimistic(nil, report)
	if err != nil {
		t.Fatalf("NotifyPessimistic: %v", err)
	}
	if !updated.FallbackUsed {
		t.Error("FallbackUsed should be true after trigger")
	}
	if updated.MVPArtifact == nil {
		t.Fatal("MVPArtifact should be populated")
	}
	if updated.MVPArtifact.Trigger != interfaces.TriggerResourceExhausted {
		t.Errorf("Trigger = %q, want %q", updated.MVPArtifact.Trigger, interfaces.TriggerResourceExhausted)
	}
	if updated.Result.Kind != interfaces.ResultKindIndeterminate {
		t.Errorf("Result.Kind = %v, want ResultKindIndeterminate", updated.Result.Kind)
	}
}

// TestEscapeEngine_NotifyPessimistic_NilReport — defensive: nil report
// is a no-op, no panic.
func TestEscapeEngine_NotifyPessimistic_NilReport(t *testing.T) {
	tracker, _ := NewLoopDepthTracker(DefaultMaxDepth)
	chain := NewChainedArbitrator(nil, nil, nil)
	engine := NewEscapeEngine(tracker, chain, NewCircuitBreakerSet(), nil, nil)
	guard := NewDefaultPessimisticCommitGuard()
	guard.Enabled = true
	engine.SetPessimisticGuard(guard)

	updated, err := engine.NotifyPessimistic(nil, nil)
	if err != nil {
		t.Fatalf("NotifyPessimistic(nil): %v", err)
	}
	if updated != nil {
		t.Error("nil report should stay nil")
	}
}