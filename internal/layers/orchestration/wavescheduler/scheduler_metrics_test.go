package wavescheduler

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSchedulerMetrics_NewFields_ZeroValue(t *testing.T) {
	var m SchedulerMetrics
	if m.WorkerPanics != 0 || m.TaskCtxLeaked != 0 ||
		m.WaveReentryCancelled != 0 || m.DispatchLoopWakeups != 0 {
		t.Errorf("zero value should have all 4 new fields = 0, got %+v", m)
	}
}

func TestSchedulerMetrics_IncMetric_NewFields(t *testing.T) {
	s := &WaveScheduler{metrics: SchedulerMetrics{}}

	// Drive each new metric via incMetric.
	for i := 0; i < 3; i++ {
		s.incMetric("worker_panic")
	}
	for i := 0; i < 7; i++ {
		s.incMetric("task_ctx_leaked")
	}
	for i := 0; i < 2; i++ {
		s.incMetric("wave_reentry_cancelled")
	}
	for i := 0; i < 11; i++ {
		s.incMetric("dispatch_wakeup")
	}

	got := s.Metrics()
	if got.WorkerPanics != 3 {
		t.Errorf("WorkerPanics = %d, want 3", got.WorkerPanics)
	}
	if got.TaskCtxLeaked != 7 {
		t.Errorf("TaskCtxLeaked = %d, want 7", got.TaskCtxLeaked)
	}
	if got.WaveReentryCancelled != 2 {
		t.Errorf("WaveReentryCancelled = %d, want 2", got.WaveReentryCancelled)
	}
	if got.DispatchLoopWakeups != 11 {
		t.Errorf("DispatchLoopWakeups = %d, want 11", got.DispatchLoopWakeups)
	}
	// Existing fields still untouched.
	if got.Started != 0 || got.Completed != 0 || got.Failed != 0 ||
		got.Cancelled != 0 || got.PeakRunning != 0 || got.TotalDispatches != 0 {
		t.Errorf("existing fields should be untouched, got %+v", got)
	}
}

func TestSchedulerMetrics_IncMetric_ConcurrentNewFields(t *testing.T) {
	// Verifies metricsMu protects the new fields under concurrent updates.
	s := &WaveScheduler{metrics: SchedulerMetrics{}}
	const goroutines = 50
	const incsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < incsPerGoroutine; j++ {
				s.incMetric("worker_panic")
			}
		}()
	}
	wg.Wait()

	if got := s.Metrics().WorkerPanics; got != int(goroutines*incsPerGoroutine) {
		t.Fatalf("WorkerPanics = %d, want %d", got, goroutines*incsPerGoroutine)
	}
}

func TestWaveScheduler_Metrics_NilReceiver(t *testing.T) {
	var s *WaveScheduler
	got := s.Metrics()
	if got != (SchedulerMetrics{}) {
		t.Errorf("nil Metrics() should return zero value, got %+v", got)
	}
}

// --- End-to-end scenarios for the 4 new PR-B metrics ---

// panickingRunner panics on Run() to exercise the WorkerPanics metric.
type panickingRunner struct {
	kind    WorkerType
	paniced atomic.Bool
}

func (p *panickingRunner) Kind() WorkerType { return p.kind }

func (p *panickingRunner) Run(ctx context.Context, spec WorkerRunSpec) error {
	p.paniced.Store(true)
	panic("synthetic worker panic for metric test")
}

// TestWaveScheduler_WorkerPanicsMetric verifies that a panicking worker
// increments SchedulerMetrics.WorkerPanics.
func TestWaveScheduler_WorkerPanicsMetric(t *testing.T) {
	runner := &panickingRunner{kind: WorkerSubAgent}
	s, _, _ := newTestScheduler(t, map[WorkerType]int{WorkerSubAgent: 1}, map[WorkerType]WorkerRunner{
		WorkerSubAgent: runner,
	})
	graph := NewTaskGraph([]TaskNode{
		{ID: "boom", WorkerType: WorkerSubAgent, ContextPolicy: ContextFresh, Directive: "boom"},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := s.Start(ctx, "sess-panic", graph); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := s.WaitForCompletion(ctx, "sess-panic"); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	m := s.Metrics()
	if m.WorkerPanics != 1 {
		t.Errorf("WorkerPanics = %d, want 1", m.WorkerPanics)
	}
	if !runner.paniced.Load() {
		t.Error("runner should have panicked at least once")
	}
}

// TestWaveScheduler_WaveReentryCancelledMetric verifies that calling Start
// twice on the same sessionID increments WaveReentryCancelled.
func TestWaveScheduler_WaveReentryCancelledMetric(t *testing.T) {
	runners := map[WorkerType]WorkerRunner{
		WorkerSubAgent: &stubRunner{kind: WorkerSubAgent, delay: 80 * time.Millisecond},
	}
	s, _, _ := newTestScheduler(t, map[WorkerType]int{WorkerSubAgent: 2}, runners)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// First wave: 1 task that takes 80ms.
	g1 := NewTaskGraph([]TaskNode{
		{ID: "first", WorkerType: WorkerSubAgent, ContextPolicy: ContextFresh, Directive: "first"},
	})
	if err := s.Start(ctx, "sess-reentry", g1); err != nil {
		t.Fatalf("Start#1: %v", err)
	}
	// Second wave: 1 task — should cancel the first (reentry).
	g2 := NewTaskGraph([]TaskNode{
		{ID: "second", WorkerType: WorkerSubAgent, ContextPolicy: ContextFresh, Directive: "second"},
	})
	if err := s.Start(ctx, "sess-reentry", g2); err != nil {
		t.Fatalf("Start#2: %v", err)
	}
	if _, err := s.WaitForCompletion(ctx, "sess-reentry"); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if got := s.Metrics().WaveReentryCancelled; got != 1 {
		t.Errorf("WaveReentryCancelled = %d, want 1", got)
	}
}

// TestWaveScheduler_DispatchLoopWakeupsMetric verifies the dispatchLoop
// ticker (20ms) increments DispatchLoopWakeups at the expected rate.
func TestWaveScheduler_DispatchLoopWakeupsMetric(t *testing.T) {
	runners := map[WorkerType]WorkerRunner{
		WorkerSubAgent: &stubRunner{kind: WorkerSubAgent, delay: 5 * time.Second},
	}
	s, _, _ := newTestScheduler(t, map[WorkerType]int{WorkerSubAgent: 1}, runners)
	graph := NewTaskGraph([]TaskNode{
		{ID: "slow", WorkerType: WorkerSubAgent, ContextPolicy: ContextFresh, Directive: "slow"},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	if err := s.Start(ctx, "sess-wakeup", graph); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Wait for the wave to finish (or ctx to time out). We don't care about
	// the artifact; we just want to read DispatchLoopWakeups.
	_, _ = s.WaitForCompletion(ctx, "sess-wakeup")

	got := s.Metrics().DispatchLoopWakeups
	// 20ms ticker → expect ≥ ~25 wakeups over 1.5s (1.5s / 20ms = 75), but
	// we use a relaxed floor (≥ 10) to avoid CI flakiness on slow runners.
	if got < 10 {
		t.Errorf("DispatchLoopWakeups = %d, want ≥ 10 (ticker should fire ≥ 10 times in 1.5s)", got)
	}
}