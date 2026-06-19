package wavescheduler

import (
	"sync"
	"testing"
	"time"
)

func TestWorkerPool_AcquireRelease(t *testing.T) {
	pool := NewWorkerPool(map[WorkerType]int{
		WorkerCursor:     1,
		WorkerClaudeCode: 1,
		WorkerSubAgent:   3,
	})

	// Cursor can take 1 slot.
	s1, ok := pool.Acquire(WorkerCursor, "task-1")
	if !ok {
		t.Fatalf("expected to acquire cursor slot")
	}
	if s1 == "" {
		t.Fatalf("expected non-empty slot id")
	}

	// Second cursor acquire should fail.
	if _, ok := pool.Acquire(WorkerCursor, "task-2"); ok {
		t.Fatalf("expected second cursor acquire to fail (max=1)")
	}

	// SubAgent can take up to 3 slots.
	for i := 0; i < 3; i++ {
		if _, ok := pool.Acquire(WorkerSubAgent, "sub-"+string(rune('a'+i))); !ok {
			t.Fatalf("expected to acquire subagent slot %d", i)
		}
	}
	// 4th should fail.
	if _, ok := pool.Acquire(WorkerSubAgent, "sub-d"); ok {
		t.Fatalf("expected 4th subagent acquire to fail (max=3)")
	}

	// Release cursor → next cursor acquire succeeds.
	pool.Release(s1)
	if _, ok := pool.Acquire(WorkerCursor, "task-3"); !ok {
		t.Fatalf("expected to acquire cursor slot after release")
	}
}

func TestWorkerPool_UnknownType(t *testing.T) {
	pool := NewWorkerPool(map[WorkerType]int{WorkerCursor: 1})
	if _, ok := pool.Acquire(WorkerType("unknown"), "task-x"); ok {
		t.Fatalf("expected unknown worker type to fail")
	}
}

func TestWorkerPool_NotifyOnRelease(t *testing.T) {
	pool := NewWorkerPool(map[WorkerType]int{WorkerCursor: 1})

	// Acquire the only slot.
	s1, _ := pool.Acquire(WorkerCursor, "task-1")

	// Set up a notifier.
	got := make(chan SlotID, 1)
	pool.OnRelease(func(id SlotID) { got <- id })

	// Release asynchronously; the callback should fire on the pool's goroutine.
	pool.Release(s1)

	select {
	case id := <-got:
		if id != s1 {
			t.Fatalf("expected slot id %q, got %q", s1, id)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("timed out waiting for release notification")
	}
}

func TestWorkerPool_ConcurrentSafe(t *testing.T) {
	pool := NewWorkerPool(map[WorkerType]int{WorkerSubAgent: 3})
	var wg sync.WaitGroup
	acquired := 0
	var mu sync.Mutex

	// Spawn 20 goroutines racing for slots.
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if s, ok := pool.Acquire(WorkerSubAgent, "task-"+string(rune('a'+i%26))); ok {
				mu.Lock()
				acquired++
				mu.Unlock()
				time.Sleep(2 * time.Millisecond)
				pool.Release(s)
			}
		}(i)
	}
	wg.Wait()
	if acquired < 3 {
		t.Fatalf("expected at least 3 successful acquires, got %d", acquired)
	}
}

func TestWorkerPool_Available(t *testing.T) {
	pool := NewWorkerPool(map[WorkerType]int{WorkerSubAgent: 2})

	if pool.Available(WorkerSubAgent) != 2 {
		t.Fatalf("expected 2 available initially, got %d", pool.Available(WorkerSubAgent))
	}
	s1, _ := pool.Acquire(WorkerSubAgent, "t1")
	if pool.Available(WorkerSubAgent) != 1 {
		t.Fatalf("expected 1 available after 1 acquire, got %d", pool.Available(WorkerSubAgent))
	}
	pool.Release(s1)
	if pool.Available(WorkerSubAgent) != 2 {
		t.Fatalf("expected 2 available after release, got %d", pool.Available(WorkerSubAgent))
	}
}
