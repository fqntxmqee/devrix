package adapters

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/wave"
)

// fakeCardkitClient is a stand-in for the real CardkitClient that records
// calls without touching Feishu.
type fakeCardkitClient struct {
	mu          sync.Mutex
	createCount int
	updateCount int
	streamCount int
	created     []string // card IDs returned in order
	cards       map[string]string
}

func newFakeCardkit() *fakeCardkitClient {
	return &fakeCardkitClient{cards: make(map[string]string)}
}

func (f *fakeCardkitClient) CreateCard(ctx context.Context, cardJSON string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createCount++
	id := "card_" + itoaInt(f.createCount)
	f.created = append(f.created, id)
	f.cards[id] = cardJSON
	return id, nil
}

func (f *fakeCardkitClient) StreamElementContent(ctx context.Context, cardID, elementID, content string, sequence int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.streamCount++
	return nil
}

func (f *fakeCardkitClient) UpdateCard(ctx context.Context, cardID, cardJSON string, sequence int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updateCount++
	f.cards[cardID] = cardJSON
	return nil
}

func (f *fakeCardkitClient) Stats() (created, streamed, updated int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.createCount, f.streamCount, f.updateCount
}

func itoaInt(n int) string {
	if n == 0 {
		return "0"
	}
	var s []byte
	for n > 0 {
		s = append([]byte{byte('0' + n%10)}, s...)
		n /= 10
	}
	return string(s)
}

func TestWorkerCardRenderer_StreamsTwoBlocksPerTask(t *testing.T) {
	// L5-ORCH-14: per-task independent card with two streaming blocks.
	fake := newFakeCardkit()
	r := NewWorkerCardRenderer(nil) // bypass public constructor
	r.cardkit = fake                // inject fake

	opts := WorkerCardOptions{
		SessionID:  "sess-A",
		TaskID:     "task-1",
		WorkerID:   "w1",
		WorkerType: wave.WorkerSubAgent,
		Title:      "build api",
	}

	// Emit thinking + text.
	if err := r.EmitWorkerEvent(context.Background(), opts, wave.WorkerEvent{Type: "thinking", Content: "hello "}); err != nil {
		t.Fatalf("emit thinking: %v", err)
	}
	if err := r.EmitWorkerEvent(context.Background(), opts, wave.WorkerEvent{Type: "text", Content: "world "}); err != nil {
		t.Fatalf("emit text: %v", err)
	}

	// Card should be created on first event.
	created, streamed, _ := fake.Stats()
	if created != 1 {
		t.Fatalf("expected 1 create, got %d", created)
	}
	if streamed < 2 {
		t.Fatalf("expected at least 2 stream calls, got %d", streamed)
	}

	// Snapshot shows accumulated buffers.
	th, ot, _, ok := r.Snapshot("sess-A", "task-1")
	if !ok {
		t.Fatal("expected session to be present")
	}
	if th != "hello " {
		t.Fatalf("expected thinking buffer 'hello ', got %q", th)
	}
	if ot != "world " {
		t.Fatalf("expected output buffer 'world ', got %q", ot)
	}
}

func TestWorkerCardRenderer_TerminalUpdate(t *testing.T) {
	fake := newFakeCardkit()
	r := NewWorkerCardRenderer(nil)
	r.cardkit = fake

	opts := WorkerCardOptions{
		SessionID:  "sess-A",
		TaskID:     "task-1",
		WorkerType: wave.WorkerCursor,
	}
	_ = r.EmitWorkerEvent(context.Background(), opts, wave.WorkerEvent{Type: "thinking", Content: "x"})
	_ = r.EmitWorkerEvent(context.Background(), opts, wave.WorkerEvent{Type: "complete", Content: "done"})

	_, _, updated := fake.Stats()
	if updated != 1 {
		t.Fatalf("expected 1 update on complete, got %d", updated)
	}
	_, _, status, _ := r.Snapshot("sess-A", "task-1")
	if status != "complete" {
		t.Fatalf("expected status 'complete', got %q", status)
	}
}

func TestWorkerCardRenderer_IndependentCardsPerTask(t *testing.T) {
	// Two tasks in the same session should produce two separate cards.
	fake := newFakeCardkit()
	r := NewWorkerCardRenderer(nil)
	r.cardkit = fake

	for _, tID := range []string{"task-1", "task-2"} {
		opts := WorkerCardOptions{
			SessionID:  "sess-A",
			TaskID:     tID,
			WorkerType: wave.WorkerSubAgent,
		}
		if err := r.EmitWorkerEvent(context.Background(), opts, wave.WorkerEvent{Type: "thinking", Content: tID}); err != nil {
			t.Fatalf("emit %s: %v", tID, err)
		}
	}
	created, _, _ := fake.Stats()
	if created != 2 {
		t.Fatalf("expected 2 distinct cards, got %d", created)
	}
	if r.ActiveSessions() != 2 {
		t.Fatalf("expected 2 sessions tracked, got %d", r.ActiveSessions())
	}
}

func TestWorkerCardRenderer_NoCardOnTerminalOnly(t *testing.T) {
	// A 'cancelled' arriving with no prior content should NOT create a card
	// (no thinking / output was streamed). The renderer treats this as
	// graceful no-op.
	fake := newFakeCardkit()
	r := NewWorkerCardRenderer(nil)
	r.cardkit = fake

	opts := WorkerCardOptions{SessionID: "sess-X", TaskID: "task-3", WorkerType: wave.WorkerSubAgent}
	if err := r.EmitWorkerEvent(context.Background(), opts, wave.WorkerEvent{Type: "cancelled"}); err != nil {
		t.Fatalf("emit cancelled: %v", err)
	}
	created, _, updated := fake.Stats()
	if created != 0 || updated != 0 {
		t.Fatalf("expected no card operations, got created=%d updated=%d", created, updated)
	}
}

func TestWorkerCardRenderer_CloseIsIdempotent(t *testing.T) {
	fake := newFakeCardkit()
	r := NewWorkerCardRenderer(fake)
	r.Close()
	r.Close() // no panic
	err := r.EmitWorkerEvent(context.Background(), WorkerCardOptions{
		SessionID:  "s",
		TaskID:     "t",
		WorkerType: wave.WorkerSubAgent,
	}, wave.WorkerEvent{Type: "thinking", Content: "x"})
	if !errors.Is(err, ErrWorkerCardClosed) {
		t.Fatalf("expected ErrWorkerCardClosed, got %v", err)
	}
}
