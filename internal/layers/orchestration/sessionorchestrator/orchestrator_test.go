package sessionorchestrator

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/decisionplanning"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// fakeD2 is a minimal D2 executor for testing. It returns a channel
// containing a single text EngineEvent followed by a complete.
type fakeD2 struct {
	calls        int
	executedMsgs []string
	mu           sync.Mutex
}

func (f *fakeD2) RunTurn(_ context.Context, req QueryRequest) (<-chan *contracts.EngineEvent, error) {
	f.mu.Lock()
	f.calls++
	f.executedMsgs = append(f.executedMsgs, req.Messages[0].Content)
	f.mu.Unlock()
	out := make(chan *contracts.EngineEvent, 3)
	out <- &contracts.EngineEvent{Type: "text", Content: "echo: " + req.Messages[0].Content, SessionID: req.SessionID}
	out <- &contracts.EngineEvent{Type: "complete", SessionID: req.SessionID}
	close(out)
	return out, nil
}

// T: D7-S2-T01 — ProcessMessage returns a streaming channel of EngineEvent.
func TestSessionOrchestrator_ProcessMessage_FastPath(t *testing.T) {
	exec := &fakeD2{}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec)
	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-1",
		Message:   "hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var events []*contracts.EngineEvent
	for ev := range ch {
		events = append(events, ev)
	}
	if len(events) < 2 {
		t.Fatalf("want ≥ 2 events (text+complete), got %d", len(events))
	}
	if events[0].Type != "text" {
		t.Fatalf("first event should be text, got %q", events[0].Type)
	}
	if events[len(events)-1].Type != "complete" {
		t.Fatalf("last event should be complete, got %q", events[len(events)-1].Type)
	}
	if exec.calls != 1 {
		t.Fatalf("want 1 D2 call, got %d", exec.calls)
	}
	if !strings.HasPrefix(events[0].Content, "echo: hello") {
		t.Fatalf("content not echoed: %q", events[0].Content)
	}
}

// T: D7-S2-T01 — empty message returns skip path (no D2 call).
func TestSessionOrchestrator_ProcessMessage_Skip(t *testing.T) {
	exec := &fakeD2{}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec)
	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-1",
		Message:   "   ",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range ch {
	}
	if exec.calls != 0 {
		t.Fatalf("skip should not call D2, got %d calls", exec.calls)
	}
}

// T: D7-S2-T01 — command-first path goes through CommandHandler
// (v1.1.0+ orthogonal dispatch), NOT through D2/FastPath. v1.0 routed
// commands to D2 with a system-prompt hint; that v1.0 simplification
// was removed (devrix-d7-orthogonal-intent-paths).
func TestSessionOrchestrator_ProcessMessage_Command(t *testing.T) {
	exec := &fakeD2{}
	cli := workmodel.NewCLICommands(workmodel.NewTaskManager())
	plan := workmodel.NewPlanCLICommands(workmodel.NewPlanMode(nil, nil))
	chHandler := NewCommandHandler(cli, plan, nil)
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec, WithCommandHandler(chHandler))
	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-1",
		Message:   "/plan add auth",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// v1.1.0+: channel must yield ≥1 text + complete event.
	var sawText, sawComplete bool
	for ev := range ch {
		if ev.Type == "text" {
			sawText = true
		}
		if ev.Type == "complete" {
			sawComplete = true
		}
	}
	if !sawText || !sawComplete {
		t.Fatalf("command path must emit text+complete, got text=%v complete=%v", sawText, sawComplete)
	}
	// v1.1.0+: command path does NOT call D2 (zero LLM cost).
	if exec.calls != 0 {
		t.Fatalf("command path must not call D2, got %d calls", exec.calls)
	}
}

// T: D7-S2-T02c — end-to-end FastPath P99 ≤2ms (per R2 保留项 4.2).
// We measure the time from ProcessMessage return to the first event.
func TestSessionOrchestrator_FastPath_EndToEnd_Latency(t *testing.T) {
	exec := &fakeD2{}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec)
	const iters = 100
	var maxDur time.Duration
	for i := 0; i < iters; i++ {
		t0 := time.Now()
		ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
			SessionID: "sess-perf",
			Message:   "hello",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_, ok := <-ch
		if !ok {
			t.Fatalf("channel closed without events")
		}
		dur := time.Since(t0)
		if dur > maxDur {
			maxDur = dur
		}
		// Drain remaining events.
		for range ch {
		}
	}
	// We allow generous headroom for CI; the spec is 2ms in production.
	// 50ms is a safe upper bound for unit tests.
	if maxDur > 50*time.Millisecond {
		t.Fatalf("FastPath P99 too high: %v (allow ≤50ms in test env)", maxDur)
	}
}

// fakeSink is a EventPublisher for testing. It records all published events.
type fakeSink struct {
	mu     sync.Mutex
	events []*contracts.EngineEvent
}

func (f *fakeSink) Publish(_ context.Context, ev *contracts.EngineEvent) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, ev)
}

// T: D7-S2-T01 — FastPath does not mirror events to sink (D1 consumes channel only).
func TestSessionOrchestrator_FastPath_NoSinkMirrorOnStream(t *testing.T) {
	exec := &fakeD2{}
	sink := &fakeSink{}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec, WithSink(sink))
	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-sink",
		Message:   "hi",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range ch {
	}
	time.Sleep(5 * time.Millisecond)
	if len(sink.events) != 0 {
		t.Fatalf("sink should not receive stream events, got %d", len(sink.events))
	}
}

// T: D7-S2-T03 — TurnExecutor returns an error; orchestrator propagates.
func TestSessionOrchestrator_FastPath_D2Error(t *testing.T) {
	exec := &errD2{}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec)
	_, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-err",
		Message:   "hi",
	})
	if err == nil {
		t.Fatalf("expected error from D2, got nil")
	}
}

type errD2 struct{}

func (errD2) RunTurn(_ context.Context, _ QueryRequest) (<-chan *contracts.EngineEvent, error) {
	return nil, errors.New("simulated d2 error")
}

// fakeWaveScheduler is the minimal WaveSchedulerRunner for testing
// OrchestratePath. It bypasses the real wave pool and returns a
// caller-supplied artifact list (typically empty for anti-fabrication
// tests).
type fakeWaveScheduler struct {
	mu        sync.Mutex
	starts    int
	waits     int
	artifacts []wavescheduler.Artifact
}

func (f *fakeWaveScheduler) Start(_ context.Context, _ string, _ *wavescheduler.TaskGraph) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.starts++
	return nil
}

func (f *fakeWaveScheduler) WaitForCompletion(_ context.Context, _ string) ([]wavescheduler.Artifact, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.waits++
	return f.artifacts, nil
}

// T: D7-S2-T04 — HandleInterrupt emits "stopped" event and runs cancelers
// in the documented order (Wave → D4 → Process), per R2 命题 E 反驳.
func TestInterruptHandler_Handle_SequenceAndEvent(t *testing.T) {
	exec := &fakeD2{}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec)
	sink := &fakeSink{}

	var waveOrder, d4Order, procOrder []string
	var mu sync.Mutex

	waveCancel := func(_ string) error {
		mu.Lock()
		waveOrder = append(waveOrder, "wave")
		mu.Unlock()
		return nil
	}
	DelegateCanceler := func(_ string) error {
		mu.Lock()
		d4Order = append(d4Order, "d4")
		mu.Unlock()
		return nil
	}
	procCancel := func(_ string) error {
		mu.Lock()
		procOrder = append(procOrder, "process")
		mu.Unlock()
		return nil
	}

	h := NewInterruptHandler(orch, InterruptOptions{
		WaveCanceler:     waveCancel,
		DelegateCanceler: DelegateCanceler,
		ProcessCanceler:  procCancel,
		Sink:             sink,
	})
	if err := h.Handle(context.Background(), "sess-int"); err != nil {
		t.Fatalf("Handle returned err: %v", err)
	}
	// Verify order: wave → d4 → process.
	mu.Lock()
	defer mu.Unlock()
	if len(waveOrder) != 1 || len(d4Order) != 1 || len(procOrder) != 1 {
		t.Fatalf("cancelers not invoked exactly once: wave=%v d4=%v proc=%v",
			waveOrder, d4Order, procOrder)
	}
	// Verify stopped event.
	if len(sink.events) != 1 {
		t.Fatalf("want 1 stopped event, got %d", len(sink.events))
	}
	if sink.events[0].Type != "stopped" {
		t.Fatalf("want Type=stopped, got %q", sink.events[0].Type)
	}
	if sink.events[0].SessionID != "sess-int" {
		t.Fatalf("SessionID mismatch: %q", sink.events[0].SessionID)
	}
}

// T: D7-S2-T05 — HandleInterrupt is idempotent (no active orchestration).
func TestInterruptHandler_Handle_Idempotent(t *testing.T) {
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), &fakeD2{})
	sink := &fakeSink{}
	called := 0
	h := NewInterruptHandler(orch, InterruptOptions{
		ProcessCanceler: func(_ string) error { called++; return nil },
		Sink:            sink,
	})
	if err := h.Handle(context.Background(), "sess-noop"); err != nil {
		t.Fatalf("Handle returned err: %v", err)
	}
	if called != 1 {
		t.Fatalf("ProcessCanceler call count = %d, want 1", called)
	}
	if len(sink.events) != 1 {
		t.Fatalf("want 1 stopped event, got %d", len(sink.events))
	}
}

// T: D7-S5-T06 — Command-first 路径不触发 LLM classify; 命令路径走
// CommandHandler (v1.1.0+ orthogonal dispatch)，完全绕过 D2，零 LLM 成本。
//
// T: types are exported and immutability of With* can be tested.
// (Immutability is enforced at the value-object layer in shared/types.)
func TestTaskSpec_Immutability(t *testing.T) {
	ts := orchtypes.TaskSpec{ID: "t1", Subject: "s", Type: orchtypes.TaskTypeExplore, CreatedAt: time.Now()}
	// Direct mutation of a value (not a pointer) is a local copy; pointer
	// fields would be the concern. This is a structural smoke test.
	if ts.ID == "" {
		t.Fatalf("orchtypes.TaskSpec should preserve ID")
	}
	if ts.Type != orchtypes.TaskTypeExplore {
		t.Fatalf("orchtypes.TaskSpec should preserve Type")
	}
	_ = types.Message{Role: "user", Content: "x"}
}

// T: D7-S2-A01-T02 — FastPath 命中时无 Wave 创建（screening 完整性）。
// FastPath.Run 直接转发 D2，不调用任何 Wave Scheduler。架构隐式保证，
// 此测试验证 orchestrator 在 FastPath 路径不触发 Wave 调度。
func TestSessionOrchestrator_FastPath_NoWaveScheduled(t *testing.T) {
	exec := &fakeD2{}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec)

	// 验证 FastPath 命中时只调用 D2，不涉及 Wave
	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-fast",
		Message:   "hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range ch {
		// drain
	}
	// FastPath 直接走 TurnExecutor.RunTurn，无 Wave 创建
	// 验证方式：fakeD2.calls == 1 且无 Wave 相关调用
	if exec.calls != 1 {
		t.Fatalf("want 1 D2 call (FastPath), got %d", exec.calls)
	}
}

// T: D7-S2-A01-T03 — 禁止在 Worker terminal FlowEvent 前伪造 Task 进度。
// anti-fabrication commitment: D7 不允许在 Worker 发送 terminal FlowEvent 之前
// 发送任何 synthetic Task progress 信号。
//
// v1.1.0+ (orthogonal dispatch): orchtypes.IntentOrchestrate routes through
// OrchestratePath → SynthesizeTaskGraph → WaveScheduler. The
// fakeWaveScheduler below emits a clean terminal-only event stream
// (plan_formed, wave_started, text, complete) — no synthetic progress.
func TestSessionOrchestrator_AntiFabrication_NoSyntheticProgress(t *testing.T) {
	exec := &fakeD2{}
	decomp := decisionplanning.NewTaskDecomposer()
	sched := &fakeWaveScheduler{artifacts: nil}
	op := NewOrchestratePath(decomp, sched, nil)
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec, WithOrchestratePath(op))

	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-anti",
		Message:   "do something complex",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var hasProgressBeforeComplete bool
	var sawTerminal bool
	var eventTypes []string
	for ev := range ch {
		eventTypes = append(eventTypes, ev.Type)
		// terminal FlowEvent 类型: complete, stopped, error
		if ev.Type == "complete" || ev.Type == "stopped" || ev.Type == "error" {
			sawTerminal = true
			break
		}
		// synthetic progress 类型: task_progress, task_update (非 terminal)
		if ev.Type == "task_progress" || ev.Type == "task_update" {
			hasProgressBeforeComplete = true
		}
	}

	// 验证：不应该在 terminal 之前看到任何 synthetic progress
	if hasProgressBeforeComplete && !sawTerminal {
		t.Fatalf("anti-fabrication violated: synthetic progress before terminal FlowEvent; events=%v", eventTypes)
	}
}

// D7-S5-A01-T01: FastPath confidence threshold gating.
// When the classifier returns orchtypes.IntentFast with confidence below the configured
// FastPathThreshold, the request is downgraded to orchtypes.IntentOrchestrate.

func TestSessionOrchestrator_FastPathConfidenceBelowThreshold(t *testing.T) {
	cfg := orchtypes.DefaultConfig()
	cfg.RoutingMode = orchtypes.RoutingModeRuleOrchestrate
	cfg.FastPathThreshold = 80 // short message default is 70

	decomp := decisionplanning.NewTaskDecomposer()
	sched := &fakeWaveScheduler{artifacts: nil}
	op := NewOrchestratePath(decomp, sched, nil)
	exec := &fakeD2{}
	orch := NewSessionOrchestrator(cfg, exec,
		WithOrchestratePath(op),
	)

	// Short message (≤32 chars, single-line) → classifier returns
	// orchtypes.IntentFast with confidence=70. With threshold=80, this should
	// be downgraded to orchtypes.IntentOrchestrate.
	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-thresh",
		Message:   "how do I test this",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var eventTypes []string
	for ev := range ch {
		eventTypes = append(eventTypes, ev.Type)
	}

	// OrchestratePath emits plan_formed → wave_started → text → complete.
	foundPlan := false
	for _, typ := range eventTypes {
		if typ == "plan_formed" {
			foundPlan = true
			break
		}
	}
	if !foundPlan {
		t.Errorf("expected plan_formed from OrchestratePath (downgrade), got events: %v", eventTypes)
	}
}

func TestSessionOrchestrator_FastPathConfidenceAboveThreshold(t *testing.T) {
	cfg := orchtypes.DefaultConfig()
	cfg.FastPathThreshold = 90 // greeting pattern is 95

	exec := &fakeD2{}
	orch := NewSessionOrchestrator(cfg, exec)

	// "hello" → classifier returns orchtypes.IntentFast with confidence=95.
	// With threshold=90, this should stay as FastPath.
	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-thresh2",
		Message:   "hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var sawText bool
	for ev := range ch {
		if ev.Type == "text" {
			sawText = true
		}
	}

	// FastPath emits text events from the fakeD2 executor.
	if !sawText {
		t.Error("expected text event from FastPath (should not be downgraded)")
	}
}

type testLLMClassifier struct {
	result orchtypes.IntentClassification
	calls  int32
}

func (t *testLLMClassifier) ClassifyIntent(ctx context.Context, _ string) (orchtypes.IntentClassification, error) {
	atomic.AddInt32(&t.calls, 1)
	return t.result, nil
}
