package runners

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// fakeSubAgentDeps returns a SubAgentDeps wired to a fake background task
// store. The store tracks tasks and allows cancel / terminal polling.
func fakeSubAgentDeps() (SubAgentDeps, *fakeBG) {
	bg := newFakeBG()
	var mu sync.Mutex
	deps := SubAgentDeps{
		Start: func(ctx context.Context, p SubAgentParams) (string, error) {
			mu.Lock()
			defer mu.Unlock()
			return bg.Start(p.AgentID)
		},
		Cancel: bg.Cancel,
		IsTerminal: func(id string) bool {
			return bg.IsTerminal(id)
		},
		TerminalResult: func(id string) (string, string, bool) {
			return bg.Result(id)
		},
		OnEvent: func(id string, ev wavescheduler.WorkerEvent) {},
	}
	return deps, bg
}

type fakeBG struct {
	mu     sync.Mutex
	nextID int
	tasks  map[string]*fakeTask
}

type fakeTask struct {
	id        string
	terminal  atomic.Bool
	cancelled atomic.Bool
	result    string
	errMsg    string
}

func newFakeBG() *fakeBG {
	return &fakeBG{tasks: make(map[string]*fakeTask)}
}

func (b *fakeBG) Start(agentID string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	id := "bg_test_" + itoa(b.nextID)
	b.tasks[id] = &fakeTask{id: id}
	return id, nil
}

func (b *fakeBG) Cancel(id string) bool {
	b.mu.Lock()
	t, ok := b.tasks[id]
	b.mu.Unlock()
	if !ok {
		return false
	}
	if t.terminal.Load() {
		return false
	}
	t.cancelled.Store(true)
	t.terminal.Store(true)
	return true
}

func (b *fakeBG) Complete(id, result, errMsg string) {
	b.mu.Lock()
	t, ok := b.tasks[id]
	b.mu.Unlock()
	if !ok {
		return
	}
	if t.terminal.Load() {
		return
	}
	t.result = result
	t.errMsg = errMsg
	t.terminal.Store(true)
}

func (b *fakeBG) IsTerminal(id string) bool {
	b.mu.Lock()
	t, ok := b.tasks[id]
	b.mu.Unlock()
	if !ok {
		return false
	}
	return t.terminal.Load()
}

func (b *fakeBG) Result(id string) (string, string, bool) {
	b.mu.Lock()
	t, ok := b.tasks[id]
	b.mu.Unlock()
	if !ok {
		return "", "", false
	}
	return t.result, t.errMsg, true
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var s []byte
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		s = append([]byte{byte('0' + n%10)}, s...)
		n /= 10
	}
	if neg {
		s = append([]byte{'-'}, s...)
	}
	return string(s)
}

func TestSubAgentRunner_HappyPath(t *testing.T) {
	deps, bg := fakeSubAgentDeps()
	r := NewSubAgentRunner(deps)

	var events []wavescheduler.WorkerEvent
	var mu sync.Mutex
	emit := func(ev wavescheduler.WorkerEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Mark terminal after a short delay.
	go func() {
		time.Sleep(50 * time.Millisecond)
		// Find the just-started task.
		bg.mu.Lock()
		var id string
		for k := range bg.tasks {
			id = k
		}
		bg.mu.Unlock()
		bg.Complete(id, "the result", "")
	}()

	spec := wavescheduler.WorkerRunSpec{
		SessionID: "s1",
		TaskID:    "t1",
		WorkDir:   "/tmp",
		Directive: "do something",
		Emit:      emit,
	}
	if err := r.Run(ctx, spec); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	// DM-20260626-002: the "started" thinking placeholder is gone
	// (real LLM thinking events now stream in via the Emit callback).
	// Expected sequence: text(result) → complete("done").
	hasText, hasComplete, hasStarted := false, false, false
	for _, e := range events {
		if e.Type == "text" && e.Content == "the result" {
			hasText = true
		}
		if e.Type == "complete" {
			hasComplete = true
		}
		if e.Type == "thinking" && e.Content == "started" {
			hasStarted = true
		}
	}
	if !hasText {
		t.Fatalf("expected text event with the result, got %+v", events)
	}
	if !hasComplete {
		t.Fatalf("expected complete event, got %+v", events)
	}
	if hasStarted {
		t.Fatalf("the 'started' thinking placeholder should be removed (DM-20260626-002), got %+v", events)
	}
}

func TestSubAgentRunner_Cancel(t *testing.T) {
	deps, _ := fakeSubAgentDeps()
	r := NewSubAgentRunner(deps)

	var events []wavescheduler.WorkerEvent
	var mu sync.Mutex
	emit := func(ev wavescheduler.WorkerEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	}

	ctx, cancel := context.WithCancel(context.Background())

	spec := wavescheduler.WorkerRunSpec{
		SessionID: "s1",
		TaskID:    "t1",
		WorkDir:   "/tmp",
		Directive: "do something",
		Emit:      emit,
	}
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := r.Run(ctx, spec)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	hasCancelled := false
	for _, e := range events {
		if e.Type == "cancelled" {
			hasCancelled = true
		}
	}
	if !hasCancelled {
		t.Fatalf("expected cancelled event, got %+v", events)
	}
}

// TestSubAgentRunner_StreamsEmitAsWorkerEvents verifies DM-20260626-002:
// every EngineEvent the SubQuery loop emits during a turn is translated
// to a WorkerEvent on the spec.Emit channel so the feishu card sees
// per-event streams (text/thinking/tool_use) instead of one big blob at
// the end. tool_call must map to tool_use (worker semantics).
func TestSubAgentRunner_StreamsEmitAsWorkerEvents(t *testing.T) {
	deps, bg := fakeSubAgentDeps()
	// Wrap the original Start so we can capture params.Emit and the
	// generated taskID, then drive events through it.
	var (
		capturedEmit contracts.EngineEmitFunc
		taskIDCh     = make(chan string, 1)
	)
	origStart := deps.Start
	deps.Start = func(ctx context.Context, p SubAgentParams) (string, error) {
		capturedEmit = p.Emit
		id, err := origStart(ctx, p)
		if err == nil {
			taskIDCh <- id
		}
		return id, err
	}
	r := NewSubAgentRunner(deps)

	var events []wavescheduler.WorkerEvent
	var mu sync.Mutex
	emit := func(ev wavescheduler.WorkerEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	doneCh := make(chan error, 1)
	go func() {
		doneCh <- r.Run(ctx, wavescheduler.WorkerRunSpec{
			SessionID: "s1",
			TaskID:    "t1",
			WorkDir:   "/tmp",
			Directive: "do something",
			Emit:      emit,
		})
	}()

	// Wait for the runner to wire Emit.
	var taskID string
	select {
	case taskID = <-taskIDCh:
	case <-time.After(2 * time.Second):
		t.Fatal("runner never wired SubAgentParams.Emit / returned taskID")
	}

	// Stream events through the captured Emit to simulate the LLM loop.
	capturedEmit(&contracts.EngineEvent{Type: "thinking", Content: "step 1"})
	capturedEmit(&contracts.EngineEvent{Type: "text", Content: "hello "})
	capturedEmit(&contracts.EngineEvent{Type: "text", Content: "world"})
	capturedEmit(&contracts.EngineEvent{Type: "tool_call", Content: "running ls", ToolName: "bash"})
	capturedEmit(&contracts.EngineEvent{Type: "info", Content: "should be skipped"})
	capturedEmit(&contracts.EngineEvent{Type: "complete", Content: "loop done — should be skipped"})

	// Mark the bg task terminal so the runner returns via TerminalResult.
	bg.Complete(taskID, "the result", "")

	select {
	case <-doneCh:
	case <-time.After(2 * time.Second):
		t.Fatal("runner did not return after terminal")
	}

	mu.Lock()
	defer mu.Unlock()
	got := map[string]int{}
	for _, e := range events {
		got[e.Type]++
	}
	// Expected mapping:
	//   thinking  → thinking  (1, streamed)
	//   text      → text      (2 streamed + 1 final from TerminalResult = 3)
	//   tool_call → tool_use  (1)
	//   info      → skipped
	//   complete  → skipped (the runner emits its own "complete" via TerminalResult)
	if got["thinking"] != 1 {
		t.Errorf("thinking count = %d, want 1", got["thinking"])
	}
	if got["text"] != 3 {
		t.Errorf("text count = %d, want 3 (2 streamed chunks + 1 final TerminalResult)", got["text"])
	}
	if got["tool_use"] != 1 {
		t.Errorf("tool_use count = %d, want 1 (tool_call must map to tool_use)", got["tool_use"])
	}
	if got["info"] != 0 {
		t.Errorf("info count = %d, want 0 (info should be skipped)", got["info"])
	}
	// The runner emits its own "complete" after TerminalResult. It must
	// NOT also pass through the loop's "complete" as a worker event.
	if got["complete"] != 1 {
		t.Errorf("complete count = %d, want exactly 1 (only the terminal marker, not the loop's complete)", got["complete"])
	}
}
