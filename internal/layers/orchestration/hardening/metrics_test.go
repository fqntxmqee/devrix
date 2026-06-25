package hardening

import (
	"sync"
	"testing"
)

func TestInterruptMetrics_Snapshot_AtomicIncrement(t *testing.T) {
	m := &InterruptMetrics{}
	const goroutines = 50
	const incsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < incsPerGoroutine; j++ {
				m.WaveCancelFailed.Add(1)
			}
		}()
	}
	wg.Wait()

	if got := m.WaveCancelFailed.Load(); got != int64(goroutines*incsPerGoroutine) {
		t.Fatalf("WaveCancelFailed = %d, want %d", got, goroutines*incsPerGoroutine)
	}

	snap := m.Snapshot()
	if snap.WaveCancelFailed != int64(goroutines*incsPerGoroutine) {
		t.Errorf("Snapshot.WaveCancelFailed = %d, want %d", snap.WaveCancelFailed, goroutines*incsPerGoroutine)
	}
	if snap.HandleCompleted != 0 {
		t.Errorf("Snapshot.HandleCompleted = %d, want 0", snap.HandleCompleted)
	}
}

func TestInterruptMetrics_NilSafe(t *testing.T) {
	var m *InterruptMetrics
	snap := m.Snapshot()
	if snap != (InterruptMetricsSnapshot{}) {
		t.Errorf("nil metrics snapshot should be zero value, got %+v", snap)
	}
	if got := m.TotalCancelFailures(); got != 0 {
		t.Errorf("nil metrics TotalCancelFailures = %d, want 0", got)
	}
}

func TestInterruptMetrics_TotalCancelFailures(t *testing.T) {
	m := &InterruptMetrics{}
	m.WaveCancelFailed.Store(3)
	m.D4CancelFailed.Store(5)
	m.ProcessCancelFailed.Store(2)

	if got := m.TotalCancelFailures(); got != 10 {
		t.Errorf("TotalCancelFailures = %d, want 10", got)
	}
}

func TestInterruptMetrics_Snapshot_AllFields(t *testing.T) {
	m := &InterruptMetrics{}
	m.WaveCancelFailed.Store(1)
	m.D4CancelFailed.Store(2)
	m.ProcessCancelFailed.Store(3)
	m.HandleCompleted.Store(10)
	m.HandleErrored.Store(4)

	snap := m.Snapshot()
	if snap.WaveCancelFailed != 1 {
		t.Errorf("WaveCancelFailed = %d, want 1", snap.WaveCancelFailed)
	}
	if snap.D4CancelFailed != 2 {
		t.Errorf("D4CancelFailed = %d, want 2", snap.D4CancelFailed)
	}
	if snap.ProcessCancelFailed != 3 {
		t.Errorf("ProcessCancelFailed = %d, want 3", snap.ProcessCancelFailed)
	}
	if snap.HandleCompleted != 10 {
		t.Errorf("HandleCompleted = %d, want 10", snap.HandleCompleted)
	}
	if snap.HandleErrored != 4 {
		t.Errorf("HandleErrored = %d, want 4", snap.HandleErrored)
	}
}