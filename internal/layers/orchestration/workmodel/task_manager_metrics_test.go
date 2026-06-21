package workmodel

import (
	"sync"
	"testing"
)

func TestTaskManagerMetrics_Snapshot_AtomicIncrement(t *testing.T) {
	m := &TaskManagerMetrics{}
	const goroutines = 50
	const incsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < incsPerGoroutine; j++ {
				m.PublishCompletionPanics.Add(1)
			}
		}()
	}
	wg.Wait()

	if got := m.PublishCompletionPanics.Load(); got != int64(goroutines*incsPerGoroutine) {
		t.Fatalf("PublishCompletionPanics = %d, want %d", got, goroutines*incsPerGoroutine)
	}

	snap := m.Snapshot()
	if snap.PublishCompletionPanics != int64(goroutines*incsPerGoroutine) {
		t.Errorf("Snapshot.PublishCompletionPanics = %d, want %d",
			snap.PublishCompletionPanics, goroutines*incsPerGoroutine)
	}
}

func TestTaskManagerMetrics_NilSafe(t *testing.T) {
	var m *TaskManagerMetrics
	snap := m.Snapshot()
	if snap != (TaskManagerMetricsSnapshot{}) {
		t.Errorf("nil metrics snapshot should be zero value, got %+v", snap)
	}
}

func TestTaskManager_SetMetrics_GetMetrics(t *testing.T) {
	tm := NewTaskManager()
	// PR-B: NewTaskManager now initializes a default *TaskManagerMetrics so
	// publishCompletion panics are always counted, even when callers don't
	// explicitly SetMetrics.
	if tm.Metrics() == nil {
		t.Error("default Metrics() should be non-nil after PR-B (publishCompletion panics always counted)")
	}
	// nil setter should still be honored (caller can disable recording).
	tm.SetMetrics(nil)
	if tm.Metrics() != nil {
		t.Errorf("SetMetrics(nil) should disable, got %v", tm.Metrics())
	}

	m := &TaskManagerMetrics{}
	got := tm.SetMetrics(m).Metrics()
	if got != m {
		t.Errorf("SetMetrics should chain and return same pointer, got %p want %p", got, m)
	}
}