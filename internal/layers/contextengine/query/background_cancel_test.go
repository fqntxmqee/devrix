package query_test

import (
	"context"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/query"
	"github.com/devrix/devrix/internal/layers/contextengine/queue"
	"github.com/devrix/devrix/internal/shared/types"
)

// blockingLLM blocks on ctx.Done() then emits a final chunk.
type blockingLLM struct {
	query.SequentialLLM // not actually used; required to keep call site clean
	started             chan struct{}
	once                bool
}

func newBlockingLLM() *blockingLLM {
	return &blockingLLM{started: make(chan struct{})}
}

func (b *blockingLLM) Call(ctx context.Context, _ query.LLMRequest) (<-chan query.LLMChunk, error) {
	ch := make(chan query.LLMChunk, 1)
	go func() {
		defer close(ch)
		select {
		case <-ctx.Done():
			return
		default:
		}
		if !b.once {
			b.once = true
			close(b.started)
		}
		<-ctx.Done()
	}()
	return ch, nil
}

// D2-S9-A01-T16: stop running task → cancelled (idempotent).
func TestBackgroundRegistry_Cancel_running_task_marks_cancelled(t *testing.T) {
	reg := query.NewBackgroundRegistry()
	cancel, _ := reg.RegisterWithCancel("sess_a", "explore", "Explore", "explore_a")

	if reg.Cancel("bg_does_not_exist") {
		t.Fatal("Cancel on unknown id should return false")
	}
	if !reg.Cancel(cancel.ID) {
		t.Fatal("first Cancel should return true")
	}
	// idempotent: second Cancel returns false
	if reg.Cancel(cancel.ID) {
		t.Fatal("second Cancel on already-cancelled task should return false")
	}

	task, ok := reg.Get(cancel.ID)
	if !ok {
		t.Fatal("expected task to be present")
	}
	if task.Status != "cancelled" {
		t.Fatalf("expected status=cancelled, got %q", task.Status)
	}
	if task.EndedAt.IsZero() {
		t.Fatal("expected EndedAt to be set on cancel")
	}
}

// D2-S9-A01-T19: cancel 后 SessionQueue 不发 completed notification (tombstone 协议).
func TestRunBackground_cancel_suppresses_completed_notification(t *testing.T) {
	llm := newBlockingLLM()
	loop := &query.Loop{LLM: llm, Permission: query.AllowPermission{}}
	reg := query.NewBackgroundRegistry()
	sq := queue.NewSessionQueue()
	parent := &types.SessionContext{SessionID: "sess_cancel", Model: "test"}

	taskID, err := query.RunBackground(context.Background(), query.LoopDeps{Loop: loop}, query.SubQueryParams{
		ParentSC:       parent,
		AgentID:        "explore_slow",
		AgentName:      "Slow",
		SystemPrompt:   "slow",
		PromptMessages: []types.Message{{Role: types.MessageRoleUser, Content: "work"}},
		MaxTurns:       5,
	}, reg, sq)
	if err != nil {
		t.Fatal(err)
	}

	// wait for goroutine to start
	select {
	case <-llm.started:
	case <-time.After(2 * time.Second):
		t.Fatal("background goroutine never started")
	}
	if !reg.Cancel(taskID) {
		t.Fatal("Cancel should succeed while running")
	}

	// Give the goroutine a moment to wind down.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		task, _ := reg.Get(taskID)
		if task != nil && task.Status == "cancelled" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	task, _ := reg.Get(taskID)
	if task == nil || task.Status != "cancelled" {
		t.Fatalf("expected final status=cancelled, got %+v", task)
	}

	// Drain the queue — it must be empty (no completed notification).
	if drained := sq.Drain("sess_cancel", "explore_slow", false); len(drained) > 0 {
		t.Fatalf("expected no queue notifications after cancel, got %d: %+v", len(drained), drained)
	}
}

// D2-S9-A01-T17: output block=false on running/completed task.
func TestBackgroundRegistry_Get_returns_partial_for_block_false(t *testing.T) {
	reg := query.NewBackgroundRegistry()
	cancel, _ := reg.RegisterWithCancel("sess_b", "explore", "Explore", "explore_b")
	defer cancel.Cancel() // cleanup

	// running
	task, ok := reg.Get(cancel.ID)
	if !ok {
		t.Fatal("expected running task")
	}
	if task.Status != "running" {
		t.Fatalf("expected running, got %q", task.Status)
	}
	if task.Result != "" {
		t.Fatal("running task should have empty result")
	}

	// Complete via Complete path so the registry transitions normally.
	reg.Complete(cancel.ID, "final answer", "", nil)
	task, _ = reg.Get(cancel.ID)
	if task.Status != "completed" || task.Result != "final answer" {
		t.Fatalf("expected completed with result, got %+v", task)
	}
}

// D2-S9-A01-T18: output block=true waits until terminal or timeout.
func TestBackgroundRegistry_Wait_terminal_returns_immediately(t *testing.T) {
	reg := query.NewBackgroundRegistry()
	cancel, _ := reg.RegisterWithCancel("sess_c", "explore", "Explore", "explore_c")
	wait := query.NewBackgroundWaiter(reg)
	wait.Register(cancel.ID) // bind waiter to this task

	go func() {
		time.Sleep(30 * time.Millisecond)
		reg.Complete(cancel.ID, "done", "", nil)
	}()

	start := time.Now()
	got, ok := wait.Wait(cancel.ID, 2*time.Second)
	elapsed := time.Since(start)
	if !ok {
		t.Fatalf("Wait should succeed, got %+v", got)
	}
	if got.Status != "completed" {
		t.Fatalf("expected completed, got %q", got.Status)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("Wait should return soon after terminal, took %v", elapsed)
	}
}

func TestBackgroundRegistry_Wait_timeout_returns_running(t *testing.T) {
	reg := query.NewBackgroundRegistry()
	cancel, _ := reg.RegisterWithCancel("sess_d", "explore", "Explore", "explore_d")
	defer cancel.Cancel()
	wait := query.NewBackgroundWaiter(reg)
	wait.Register(cancel.ID)

	start := time.Now()
	got, ok := wait.Wait(cancel.ID, 100*time.Millisecond)
	elapsed := time.Since(start)
	if ok {
		t.Fatal("Wait should NOT succeed for still-running task within timeout")
	}
	if got == nil || got.Status != "running" {
		t.Fatalf("expected running snapshot, got %+v", got)
	}
	if elapsed < 80*time.Millisecond || elapsed > 500*time.Millisecond {
		t.Fatalf("Wait should respect timeout (~100ms), took %v", elapsed)
	}
}

func TestBackgroundRegistry_List_returns_session_tasks(t *testing.T) {
	reg := query.NewBackgroundRegistry()
	c1, _ := reg.RegisterWithCancel("sess_e", "explore", "Explore", "e1")
	c2, _ := reg.RegisterWithCancel("sess_e", "implement", "Implement", "i1")
	c3, _ := reg.RegisterWithCancel("sess_other", "explore", "Explore", "e2")
	defer c1.Cancel()
	defer c2.Cancel()
	defer c3.Cancel()

	tasks := reg.List("sess_e")
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks for sess_e, got %d", len(tasks))
	}
	for _, ts := range tasks {
		if ts.SessionID != "sess_e" {
			t.Fatalf("foreign session leaked: %+v", ts)
		}
	}
}
