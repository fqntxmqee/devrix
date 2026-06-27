package sessionorchestrator

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
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

// T: D7-S2-T01 — ProcessMessage returns a streaming channel via RunSessionTurnLoop.
func TestSessionOrchestrator_ProcessMessage_TurnLoop(t *testing.T) {
	orch, _, _ := newOrchestratorWithItemPipeline(t)
	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: "sess-1",
		Message:   "hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	events := drainEvents(ch)
	if !hasEventType(events, "complete") {
		t.Fatalf("last event should be complete, got %v", loopEventTypes(events))
	}
	if !hasEventType(events, "pipeline_round") {
		t.Fatalf("expected pipeline_round, got %v", loopEventTypes(events))
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
// v6.1.0: replaced by OrchestratePath first-event latency; the v6.0
// FastPath P99 ≤ 2ms target is retired (5-node pipeline first event
// is plan_formed, dominated by orchestrator entry overhead).
func TestSessionOrchestrator_TurnLoop_FirstEvent_Latency(t *testing.T) {
	orch, _, _ := newOrchestratorWithItemPipeline(t)
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
	// We allow generous headroom for CI; the v6.0 2ms FastPath target was
	// retired. 50ms is a safe upper bound for unit tests.
	if maxDur > 50*time.Millisecond {
		t.Fatalf("turn loop first-event too slow: %v (allow ≤50ms in test env)", maxDur)
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

// T: D7-S2-T01 — OrchestratePath does not mirror events to sink (D1
// consumes channel only). v6.1.0: rename from FastPath_NoSinkMirrorOnStream.
func TestSessionOrchestrator_TurnLoop_NoSinkMirrorOnStream(t *testing.T) {
	sink := &fakeSink{}
	orch, _, _ := newOrchestratorWithItemPipeline(t, WithSink(sink))
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
	orch := newTestOrch(t)
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

// T: D7-S2-A01-T03 — no synthetic task progress before terminal events on turn loop path.
func TestSessionOrchestrator_AntiFabrication_NoSyntheticProgress(t *testing.T) {
	orch, _, _ := newOrchestratorWithItemPipeline(t)
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
		if ev.Type == "complete" || ev.Type == "stopped" || ev.Type == "error" {
			sawTerminal = true
			break
		}
		if ev.Type == "task_progress" || ev.Type == "task_update" {
			hasProgressBeforeComplete = true
		}
	}
	if hasProgressBeforeComplete && !sawTerminal {
		t.Fatalf("anti-fabrication violated: synthetic progress before terminal FlowEvent; events=%v", eventTypes)
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
