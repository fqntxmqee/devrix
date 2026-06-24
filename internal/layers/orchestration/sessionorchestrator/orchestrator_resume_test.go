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
	"sync"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
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
// Test error injection uses a real HumanArbitrator backed by a custom
// errStore (defined below) — this exercises the real Load→Delete path
// instead of a mock, avoiding mock drift.

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

// --- H-4 fixtures --------------------------------------------------------

// recordingExecutor wraps completingExecutor but records RunTurn call count,
// so H-4 integration tests can assert that short-circuit path never invokes
// the underlying TurnExecutor (5 节点 pipeline fully skipped).
type recordingExecutor struct {
	eventType    string
	eventContent string
	calls        int
}

func (e *recordingExecutor) RunTurn(_ context.Context, _ QueryRequest) (<-chan *contracts.EngineEvent, error) {
	e.calls++
	out := make(chan *contracts.EngineEvent, 1)
	out <- &contracts.EngineEvent{Type: e.eventType, Content: e.eventContent}
	close(out)
	return out, nil
}

// recordingSpan implements tracer.Span by capturing SetAttributes calls.
// Used by H-3 sub-tests to assert which attributes applyResumeSession sets
// along each decision path. Non-essential methods are no-ops.
type recordingSpan struct {
	mu    sync.Mutex
	attrs map[string]interface{}
}

func newRecordingSpan() *recordingSpan {
	return &recordingSpan{attrs: map[string]interface{}{}}
}

func (s *recordingSpan) End(_ ...tracer.SpanEndOption)                         {}
func (s *recordingSpan) SetStatus(_ tracer.SpanStatusCode, _ string)          {}
func (s *recordingSpan) RecordError(_ error, _ ...tracer.RecordErrorOption)   {}
func (s *recordingSpan) AddEvent(_ string, _ ...tracer.EventOption)           {}
func (s *recordingSpan) SpanContext() tracer.SpanContext                       { return tracer.SpanContext{} }
func (s *recordingSpan) IsRecording() bool                                    { return true }

func (s *recordingSpan) SetAttributes(kv ...tracer.Attribute) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range kv {
		s.attrs[a.Key] = a.Value
	}
}

func (s *recordingSpan) Get(key string) (interface{}, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.attrs[key]
	return v, ok
}

// --- M-1: empty SessionID → silent fall through (no slog.Warn) ------------

// DM-20260625-004 review-fixes: empty SessionID is a contract violation
// (ProcessRequest constructor requires non-empty), not a transient error —
// must silently fall through without slog.Warn to avoid log noise / misdiagnosis.
// 验证: span 仍收到 attempted='false' (虽然没尝试),不调用 engine.ResumeSession
// (用 errStore 注入即可:若误调了 Load 会 panic;事实上不会 panic 即说明未调)。
func TestApplyResumeSession_EmptySessionID_FallThrough(t *testing.T) {
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
	span := newRecordingSpan()

	ch, short, err := orch.applyResumeSession(context.Background(),
		orchtypes.ProcessRequest{SessionID: ""}, span)
	if err != nil {
		t.Fatalf("err: want nil, got %v", err)
	}
	if short {
		t.Error("shortCircuit: want false (empty SessionID → fall through), got true")
	}
	if ch != nil {
		t.Error("ch: want nil (fall through), got non-nil")
	}
	// span must record attempted='false' (we did NOT call engine.ResumeSession)
	if v, ok := span.Get("escape.resume.attempted"); !ok || v != "false" {
		t.Errorf("span[escape.resume.attempted]: want false, got %v (ok=%v)", v, ok)
	}
	// decision_action and decision_pending_id must NOT be set
	if _, ok := span.Get("escape.resume.decision_action"); ok {
		t.Error("span[escape.resume.decision_action]: should NOT be set on empty SessionID")
	}
}

// --- H-3: 4 类 SetAttributes 路径全覆盖 -----------------------------------

// TestApplyResumeSession_SessionSpanAttrs 守护 4 类 SetAttributes 路径:
//   1. nil engine              → attempted='false' (且不设 decision_action)
//   2. err failsafe            → attempted='true', decision_action='error_failsafe'
//   3. not found (TTL 过期)     → attempted='true' (且不设 decision_action)
//   4. found terminal decision → attempted='true', decision_action=<action>, decision_pending_id=<id>
func TestApplyResumeSession_SessionSpanAttrs(t *testing.T) {
	t.Run("nil_engine", func(t *testing.T) {
		orch := &SessionOrchestrator{escapeEngine: nil}
		span := newRecordingSpan()
		_, short, err := orch.applyResumeSession(context.Background(),
			orchtypes.ProcessRequest{SessionID: "sess-attr-1"}, span)
		if err != nil || short {
			t.Fatalf("fall through expected, got err=%v short=%v", err, short)
		}
		if v, _ := span.Get("escape.resume.attempted"); v != "false" {
			t.Errorf("attempted: want false, got %v", v)
		}
		if _, ok := span.Get("escape.resume.decision_action"); ok {
			t.Error("decision_action: should NOT be set on nil engine")
		}
	})
	t.Run("err_failsafe", func(t *testing.T) {
		ha := escape.NewHumanArbitrator(nil, nil, errStore{})
		engine := escape.NewEscapeEngine(
			&stubDepthChecker{decision: escape.EscapeDecision{
				Action: escape.EscapeContinue, Reason: "no_op",
			}},
			nil,
			escape.NewCircuitBreakerSet(),
			nil, ha,
		)
		orch := &SessionOrchestrator{escapeEngine: engine}
		span := newRecordingSpan()
		_, short, err := orch.applyResumeSession(context.Background(),
			orchtypes.ProcessRequest{SessionID: "sess-attr-2"}, span)
		if err != nil || short {
			t.Fatalf("fall through expected, got err=%v short=%v", err, short)
		}
		if v, _ := span.Get("escape.resume.attempted"); v != "true" {
			t.Errorf("attempted: want true, got %v", v)
		}
		if v, _ := span.Get("escape.resume.decision_action"); v != "error_failsafe" {
			t.Errorf("decision_action: want error_failsafe, got %v", v)
		}
	})
	t.Run("not_found", func(t *testing.T) {
		store := escape.NewInMemoryPendingResolutionStore()
		engine := newResumeEngine(t, store) // 空 store → Load 返回 (zero,false,nil)
		orch := &SessionOrchestrator{escapeEngine: engine}
		span := newRecordingSpan()
		_, short, err := orch.applyResumeSession(context.Background(),
			orchtypes.ProcessRequest{SessionID: "sess-attr-3"}, span)
		if err != nil || short {
			t.Fatalf("fall through expected, got err=%v short=%v", err, short)
		}
		if v, _ := span.Get("escape.resume.attempted"); v != "true" {
			t.Errorf("attempted: want true, got %v", v)
		}
		// not_found: 故意不设 decision_action
		if _, ok := span.Get("escape.resume.decision_action"); ok {
			t.Error("decision_action: should NOT be set on not_found (TTL expired)")
		}
	})
	t.Run("found_terminal", func(t *testing.T) {
		store := escape.NewInMemoryPendingResolutionStore()
		engine := newResumeEngine(t, store)
		orch := &SessionOrchestrator{escapeEngine: engine}
		saveDecision(t, store, "sess-attr-4", escape.EscapeDecision{
			Action:     escape.EscapeForceExit,
			Reason:     "user_accept",
			AuditLevel: 1,
			PendingID:  "p-attr-4",
			SessionID:  "sess-attr-4",
			CreatedAt:  time.Now(),
		})
		span := newRecordingSpan()
		_, short, err := orch.applyResumeSession(context.Background(),
			orchtypes.ProcessRequest{SessionID: "sess-attr-4"}, span)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if !short {
			t.Fatal("shortCircuit: want true, got false")
		}
		if v, _ := span.Get("escape.resume.attempted"); v != "true" {
			t.Errorf("attempted: want true, got %v", v)
		}
		if v, _ := span.Get("escape.resume.decision_action"); v != "force_exit" {
			t.Errorf("decision_action: want force_exit, got %v", v)
		}
		if v, _ := span.Get("escape.resume.decision_pending_id"); v != "p-attr-4" {
			t.Errorf("decision_pending_id: want p-attr-4, got %v", v)
		}
	})
}

// --- H-4 增强: 端到端短路 → TurnExecutor 完全未被调用 + 5 字段 Metadata ---

// TestProcessMessage_WithResume_UserAccept_EarlyClose_NoExecutorCall
// 守护: 短路早退路径完全不进入 5 节点 (TurnExecutor.RunTurn 永远不应被调)。
// H-4 (DM-20260625-004): recordingExecutor.calls 必须严格 == 0。
func TestProcessMessage_WithResume_UserAccept_EarlyClose_NoExecutorCall(t *testing.T) {
	store := escape.NewInMemoryPendingResolutionStore()
	engine := newResumeEngine(t, store)
	saveDecision(t, store, "sess-e2e-rec-b", escape.EscapeDecision{
		Action:     escape.EscapeForceExit,
		Reason:     "user_accept",
		AuditLevel: 1,
		PendingID:  "p-rec-b",
		SessionID:  "sess-e2e-rec-b",
		CreatedAt:  time.Now(),
	})
	rec := &recordingExecutor{eventType: "complete"}
	orch := NewSessionOrchestrator(
		orchtypes.DefaultConfig(),
		rec,
		WithEscapeEngine(engine),
	)
	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-e2e-rec-b",
		Message:   "hi",
	})
	if err != nil {
		t.Fatalf("ProcessMessage: %v", err)
	}
	events := []*contracts.EngineEvent{}
	for ev := range ch {
		events = append(events, ev)
	}
	// H-4 key assertion: RunTurn 一次都不应被调
	if rec.calls != 0 {
		t.Errorf("recordingExecutor.calls: want 0 (short-circuit path), got %d", rec.calls)
	}
	// 1 个 complete event + 5 字段 metadata
	if len(events) != 1 {
		t.Fatalf("events: want 1, got %d", len(events))
	}
	ev := events[0]
	if ev.Type != "complete" {
		t.Errorf("Type: want complete, got %q", ev.Type)
	}
	wantMeta := map[string]string{
		"escape.resume":      "true",
		"escape.action":      "force_exit",
		"escape.reason":      "user_accept",
		"escape.pending_id":  "p-rec-b",
		"exit_reason_source": "user_resume",
	}
	for k, want := range wantMeta {
		if got := ev.Metadata[k]; got != want {
			t.Errorf("Metadata[%s]: want %q, got %q", k, want, got)
		}
	}
	if ev.SessionID != "sess-e2e-rec-b" {
		t.Errorf("SessionID: want sess-e2e-rec-b, got %q", ev.SessionID)
	}
}

// TestProcessMessage_WithResume_UserCancel_EarlyClose_NoExecutorCall 同 H-4
// 用例,守护 user_cancel 短路早退路径 + 5 字段 metadata。
func TestProcessMessage_WithResume_UserCancel_EarlyClose_NoExecutorCall(t *testing.T) {
	store := escape.NewInMemoryPendingResolutionStore()
	engine := newResumeEngine(t, store)
	saveDecision(t, store, "sess-e2e-rec-c", escape.EscapeDecision{
		Action:     escape.EscapeAbortWithAudit,
		Reason:     "user_cancel",
		AuditLevel: 2,
		PendingID:  "p-rec-c",
		SessionID:  "sess-e2e-rec-c",
		CreatedAt:  time.Now(),
	})
	rec := &recordingExecutor{eventType: "complete"}
	orch := NewSessionOrchestrator(
		orchtypes.DefaultConfig(),
		rec,
		WithEscapeEngine(engine),
	)
	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-e2e-rec-c",
		Message:   "hi",
	})
	if err != nil {
		t.Fatalf("ProcessMessage: %v", err)
	}
	events := []*contracts.EngineEvent{}
	for ev := range ch {
		events = append(events, ev)
	}
	if rec.calls != 0 {
		t.Errorf("recordingExecutor.calls: want 0 (short-circuit path), got %d", rec.calls)
	}
	if len(events) != 1 {
		t.Fatalf("events: want 1, got %d", len(events))
	}
	ev := events[0]
	if ev.Type != "complete" {
		t.Errorf("Type: want complete, got %q", ev.Type)
	}
	wantMeta := map[string]string{
		"escape.resume":      "true",
		"escape.action":      "abort_with_audit",
		"escape.reason":      "user_cancel",
		"escape.pending_id":  "p-rec-c",
		"exit_reason_source": "user_resume",
	}
	for k, want := range wantMeta {
		if got := ev.Metadata[k]; got != want {
			t.Errorf("Metadata[%s]: want %q, got %q", k, want, got)
		}
	}
}
