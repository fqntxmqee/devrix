package freefork

import (
	"sync"
	"testing"
)

func TestForkerMetrics_Snapshot_AtomicIncrement(t *testing.T) {
	m := &ForkerMetrics{}
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
}

func TestForkerMetrics_NilSafe(t *testing.T) {
	var m *ForkerMetrics
	snap := m.Snapshot()
	if snap != (ForkerMetricsSnapshot{}) {
		t.Errorf("nil metrics snapshot should be zero value, got %+v", snap)
	}
}

func TestForkerMetrics_Snapshot_AllFields(t *testing.T) {
	m := &ForkerMetrics{}
	m.Spawned.Store(10)
	m.SpawnFailed.Store(2)
	m.SandboxEnterFailed.Store(1)
	m.SandboxExitFailed.Store(3)
	m.FactoryCreateFailed.Store(4)
	m.RollbackTriggered.Store(2)

	snap := m.Snapshot()
	if snap.Spawned != 10 {
		t.Errorf("Spawned = %d, want 10", snap.Spawned)
	}
	if snap.SpawnFailed != 2 {
		t.Errorf("SpawnFailed = %d, want 2", snap.SpawnFailed)
	}
	if snap.SandboxEnterFailed != 1 {
		t.Errorf("SandboxEnterFailed = %d, want 1", snap.SandboxEnterFailed)
	}
	if snap.SandboxExitFailed != 3 {
		t.Errorf("SandboxExitFailed = %d, want 3", snap.SandboxExitFailed)
	}
	if snap.FactoryCreateFailed != 4 {
		t.Errorf("FactoryCreateFailed = %d, want 4", snap.FactoryCreateFailed)
	}
	if snap.RollbackTriggered != 2 {
		t.Errorf("RollbackTriggered = %d, want 2", snap.RollbackTriggered)
	}
}