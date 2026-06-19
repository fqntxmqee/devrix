package runners

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/multiagent/external"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
)

type fakeAgentTool struct {
	events []external.Event
	mu     sync.Mutex
	// blockUntilCancel makes Execute block on ctx.Done() after sending the
	// first event so the runner observes cancellation while still streaming.
	blockUntilCancel bool
}

func (f *fakeAgentTool) Info() external.Info { return external.Info{Name: "cursor", DisplayName: "Cursor"} }
func (f *fakeAgentTool) Execute(ctx context.Context, sessionID string, req external.Request) (<-chan external.Event, error) {
	ch := make(chan external.Event, 16)
	go func() {
		defer close(ch)
		for _, e := range f.events {
			select {
			case <-ctx.Done():
				return
			case ch <- e:
			}
		}
		if f.blockUntilCancel {
			<-ctx.Done()
		}
	}()
	return ch, nil
}

func TestAgentToolRunner_StreamEvents(t *testing.T) {
	fake := &fakeAgentTool{
		events: []external.Event{
			{Type: "thinking", Content: "thinking…"},
			{Type: "text", Content: "hello"},
			{Type: "complete", Content: "done"},
		},
	}
	reg := external.NewRegistry()
	if err := reg.Register(fake); err != nil {
		t.Fatalf("register: %v", err)
	}

	r := NewAgentToolRunner(wavescheduler.WorkerCursor, AgentToolDeps{Registry: reg})

	var got []wavescheduler.WorkerEvent
	var mu sync.Mutex
	emit := func(ev wavescheduler.WorkerEvent) {
		mu.Lock()
		got = append(got, ev)
		mu.Unlock()
	}
	spec := wavescheduler.WorkerRunSpec{
		SessionID: "s1",
		TaskID:    "t1",
		WorkDir:   "/tmp",
		Directive: "do x",
		Emit:      emit,
	}
	if err := r.Run(context.Background(), spec); err != nil {
		t.Fatalf("Run: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	hasThinking := false
	hasText := false
	hasComplete := false
	for _, e := range got {
		switch e.Type {
		case "thinking":
			hasThinking = true
		case "text":
			hasText = true
		case "complete":
			hasComplete = true
		}
	}
	if !hasThinking || !hasText || !hasComplete {
		t.Fatalf("expected all event types, got %+v", got)
	}
}

func TestAgentToolRunner_CancelEmitsCancelled(t *testing.T) {
	// ORCH-S2-T21 (stub): cancellation must produce a 'cancelled' event so the
	// IM card footer can show the reason. Full process-kill testing requires
	// OS-level integration; this test covers the runner-bridge half.
	fake := &fakeAgentTool{
		events: []external.Event{
			{Type: "thinking", Content: "start"},
		},
		blockUntilCancel: true,
	}
	reg := external.NewRegistry()
	_ = reg.Register(fake)

	r := NewAgentToolRunner(wavescheduler.WorkerCursor, AgentToolDeps{Registry: reg})

	var got []wavescheduler.WorkerEvent
	var mu sync.Mutex
	emit := func(ev wavescheduler.WorkerEvent) {
		mu.Lock()
		got = append(got, ev)
		mu.Unlock()
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	spec := wavescheduler.WorkerRunSpec{
		SessionID: "s1",
		TaskID:    "t1",
		WorkDir:   "/tmp",
		Directive: "do x",
		Emit:      emit,
	}
	err := r.Run(ctx, spec)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	hasCancelled := false
	for _, e := range got {
		if e.Type == "cancelled" {
			hasCancelled = true
		}
	}
	if !hasCancelled {
		t.Fatalf("expected cancelled event, got %+v", got)
	}
}
