package runners

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
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
	hasThinking := false
	hasComplete := false
	for _, e := range events {
		if e.Type == "thinking" {
			hasThinking = true
		}
		if e.Type == "complete" {
			hasComplete = true
		}
	}
	if !hasThinking || !hasComplete {
		t.Fatalf("expected thinking + complete events, got %+v", events)
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
