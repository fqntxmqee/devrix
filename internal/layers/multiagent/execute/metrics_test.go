package execute

import (
	"sync"
	"testing"
)

func TestExecutorMetrics_Snapshot_AtomicIncrement(t *testing.T) {
	m := &ExecutorMetrics{}
	const goroutines = 50
	const incsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < incsPerGoroutine; j++ {
				m.SandboxExitFailed.Add(1)
			}
		}()
	}
	wg.Wait()

	if got := m.SandboxExitFailed.Load(); got != int64(goroutines*incsPerGoroutine) {
		t.Fatalf("SandboxExitFailed = %d, want %d", got, goroutines*incsPerGoroutine)
	}

	snap := m.Snapshot()
	if snap.SandboxExitFailed != int64(goroutines*incsPerGoroutine) {
		t.Errorf("Snapshot.SandboxExitFailed = %d, want %d",
			snap.SandboxExitFailed, goroutines*incsPerGoroutine)
	}
}

func TestExecutorMetrics_NilSafe(t *testing.T) {
	var m *ExecutorMetrics
	snap := m.Snapshot()
	if snap != (ExecutorMetricsSnapshot{}) {
		t.Errorf("nil metrics snapshot should be zero value, got %+v", snap)
	}
}

func TestExecutor_WithMetrics_NilSafe(t *testing.T) {
	e := &Executor{}
	// recordSandboxExitFailed should not panic with nil metrics.
	e.recordSandboxExitFailed("test", "sess-A", "/tmp/x", nil)
	// no assertion needed — absence of panic is the contract.
}