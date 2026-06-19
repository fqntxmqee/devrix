package wavescheduler

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// stubRunner drives a single task. delay controls how long it blocks before
// emitting "complete". The runner can be cancelled via ctx.
type stubRunner struct {
	kind    WorkerType
	delay   time.Duration
	emitCh  chan WorkerEvent
	running atomic.Int32
}

func (s *stubRunner) Kind() WorkerType { return s.kind }

func (s *stubRunner) Run(ctx context.Context, spec WorkerRunSpec) error {
	s.running.Add(1)
	defer s.running.Add(-1)
	if s.emitCh != nil {
		go func() {
			for ev := range s.emitCh {
				if spec.Emit != nil {
					spec.Emit(ev)
				}
			}
		}()
	}
	select {
	case <-time.After(s.delay):
		if spec.Emit != nil {
			spec.Emit(WorkerEvent{Type: "complete", Content: "done"})
		}
		return nil
	case <-ctx.Done():
		if spec.Emit != nil {
			spec.Emit(WorkerEvent{Type: "cancelled", Content: "cancelled"})
		}
		return ctx.Err()
	}
}

func newTestScheduler(t *testing.T, cap map[WorkerType]int, runners map[WorkerType]WorkerRunner) (*WaveScheduler, *WorkerPool, *ConflictGuard) {
	t.Helper()
	pool := NewWorkerPool(cap)
	guard := NewConflictGuard()
	artifacts := NewArtifactStore()
	resolver := NewContextResolver(ContextResolverDeps{Artifacts: artifacts, BaseSystemPrompt: "test"})

	runnersCopy := make(map[WorkerType]WorkerRunner, len(runners))
	for k, v := range runners {
		runnersCopy[k] = v
	}

	s := &WaveScheduler{
		pool:      pool,
		guard:     guard,
		resolver:  resolver,
		artifacts: artifacts,
		runners:   runnersCopy,
		waves:     make(map[string]*schedulerWaveState),
	}
	return s, pool, guard
}

func TestWaveScheduler_StartDispatchesReady(t *testing.T) {
	// ORCH-S2-T17: only ready nodes are dispatched.
	runners := map[WorkerType]WorkerRunner{
		WorkerSubAgent: &stubRunner{kind: WorkerSubAgent, delay: 50 * time.Millisecond},
	}
	s, _, _ := newTestScheduler(t, map[WorkerType]int{WorkerSubAgent: 3}, runners)

	graph := NewTaskGraph([]TaskNode{
		{ID: "a", WorkerType: WorkerSubAgent, ContextPolicy: ContextFresh, Directive: "a"},
		{ID: "b", WorkerType: WorkerSubAgent, ContextPolicy: ContextFresh, Directive: "b", DependsOn: []string{"a"}},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := s.Start(ctx, "sess-1", graph); err != nil {
		t.Fatalf("Start: %v", err)
	}

	artifacts, err := s.WaitForCompletion(ctx, "sess-1")
	if err != nil {
		t.Fatalf("WaitForCompletion: %v", err)
	}
	if len(artifacts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(artifacts))
	}
}

func TestWaveScheduler_PeakConcurrency5(t *testing.T) {
	// ORCH-S2-T10: 6 ready subagent + 1 cursor → peak≤5 with caps respected.
	sub := &stubRunner{kind: WorkerSubAgent, delay: 80 * time.Millisecond}
	cur := &stubRunner{kind: WorkerCursor, delay: 80 * time.Millisecond}
	runners := map[WorkerType]WorkerRunner{WorkerSubAgent: sub, WorkerCursor: cur}
	s, _, _ := newTestScheduler(t, map[WorkerType]int{WorkerSubAgent: 3, WorkerCursor: 1}, runners)

	nodes := []TaskNode{
		{ID: "s1", WorkerType: WorkerSubAgent, ContextPolicy: ContextFresh, Directive: "s1"},
		{ID: "s2", WorkerType: WorkerSubAgent, ContextPolicy: ContextFresh, Directive: "s2"},
		{ID: "s3", WorkerType: WorkerSubAgent, ContextPolicy: ContextFresh, Directive: "s3"},
		{ID: "s4", WorkerType: WorkerSubAgent, ContextPolicy: ContextFresh, Directive: "s4"},
		{ID: "s5", WorkerType: WorkerSubAgent, ContextPolicy: ContextFresh, Directive: "s5"},
		{ID: "s6", WorkerType: WorkerSubAgent, ContextPolicy: ContextFresh, Directive: "s6"},
		{ID: "c1", WorkerType: WorkerCursor, ContextPolicy: ContextFresh, Directive: "c1"},
	}
	graph := NewTaskGraph(nodes)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Start(ctx, "sess-pc", graph); err != nil {
		t.Fatalf("Start: %v", err)
	}

	var peakSub, peakCur atomic.Int32
	stop := make(chan struct{})
	var sampler sync.WaitGroup
	sampler.Add(1)
	go func() {
		defer sampler.Done()
		for {
			select {
			case <-stop:
				return
			case <-time.After(5 * time.Millisecond):
			}
			subCount := sub.running.Load()
			curCount := cur.running.Load()
			if subCount > peakSub.Load() {
				peakSub.Store(subCount)
			}
			if curCount > peakCur.Load() {
				peakCur.Store(curCount)
			}
		}
	}()

	if _, err := s.WaitForCompletion(ctx, "sess-pc"); err != nil {
		t.Fatalf("WaitForCompletion: %v", err)
	}
	close(stop)
	sampler.Wait()

	if peakSub.Load() > 3 {
		t.Fatalf("subagent peak %d > 3", peakSub.Load())
	}
	if peakCur.Load() > 1 {
		t.Fatalf("cursor peak %d > 1", peakCur.Load())
	}
	// Combined peak ≤ 4 (3 sub + 1 cursor).
	if peakSub.Load()+peakCur.Load() > 4 {
		t.Fatalf("combined peak %d > 4", peakSub.Load()+peakCur.Load())
	}
}

func TestWaveScheduler_CancelWorker(t *testing.T) {
	// ORCH-S2-T19: CancelWorker releases slot and marks status=cancelled.
	runners := map[WorkerType]WorkerRunner{
		WorkerSubAgent: &stubRunner{kind: WorkerSubAgent, delay: 5 * time.Second},
	}
	s, pool, _ := newTestScheduler(t, map[WorkerType]int{WorkerSubAgent: 1}, runners)

	graph := NewTaskGraph([]TaskNode{
		{ID: "a", WorkerType: WorkerSubAgent, ContextPolicy: ContextFresh, Directive: "a"},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Start(ctx, "sess-cw", graph); err != nil {
		t.Fatalf("Start: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if pool.InUse(WorkerSubAgent) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pool.InUse(WorkerSubAgent) == 0 {
		t.Fatalf("expected subagent slot in use")
	}

	if err := s.CancelWorker("sess-cw", "a"); err != nil {
		t.Fatalf("CancelWorker: %v", err)
	}

	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if pool.InUse(WorkerSubAgent) == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pool.InUse(WorkerSubAgent) != 0 {
		t.Fatalf("expected slot released after cancel")
	}
	state, _ := graph.State("a")
	if state != StateCancelled {
		t.Fatalf("expected state=cancelled, got %q", state)
	}
}

func TestWaveScheduler_CancelAll(t *testing.T) {
	// ORCH-S2-T20: 5 running → CancelAll → all terminal.
	runners := map[WorkerType]WorkerRunner{
		WorkerSubAgent:   &stubRunner{kind: WorkerSubAgent, delay: 5 * time.Second},
		WorkerCursor:     &stubRunner{kind: WorkerCursor, delay: 5 * time.Second},
		WorkerClaudeCode: &stubRunner{kind: WorkerClaudeCode, delay: 5 * time.Second},
	}
	s, pool, _ := newTestScheduler(t,
		map[WorkerType]int{WorkerSubAgent: 3, WorkerCursor: 1, WorkerClaudeCode: 1},
		runners)

	nodes := []TaskNode{
		{ID: "s1", WorkerType: WorkerSubAgent, ContextPolicy: ContextFresh, Directive: "s1"},
		{ID: "s2", WorkerType: WorkerSubAgent, ContextPolicy: ContextFresh, Directive: "s2"},
		{ID: "s3", WorkerType: WorkerSubAgent, ContextPolicy: ContextFresh, Directive: "s3"},
		{ID: "c1", WorkerType: WorkerCursor, ContextPolicy: ContextFresh, Directive: "c1"},
		{ID: "cc1", WorkerType: WorkerClaudeCode, ContextPolicy: ContextFresh, Directive: "cc1"},
	}
	graph := NewTaskGraph(nodes)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Start(ctx, "sess-ca", graph); err != nil {
		t.Fatalf("Start: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		total := pool.InUse(WorkerSubAgent) + pool.InUse(WorkerCursor) + pool.InUse(WorkerClaudeCode)
		if total == 5 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := pool.InUse(WorkerSubAgent) + pool.InUse(WorkerCursor) + pool.InUse(WorkerClaudeCode); got != 5 {
		t.Fatalf("expected 5 in use, got %d", got)
	}

	cancelled := s.CancelAll("sess-ca")
	if cancelled != 5 {
		t.Fatalf("expected 5 cancelled, got %d", cancelled)
	}

	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		total := pool.InUse(WorkerSubAgent) + pool.InUse(WorkerCursor) + pool.InUse(WorkerClaudeCode)
		if total == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := pool.InUse(WorkerSubAgent) + pool.InUse(WorkerCursor) + pool.InUse(WorkerClaudeCode); got != 0 {
		t.Fatalf("expected 0 in use after cancel, got %d", got)
	}
}

func TestWaveScheduler_SlotReleaseDispatchesNext(t *testing.T) {
	// ORCH-S2-T15: slot release immediately dispatches next ready task.
	runners := map[WorkerType]WorkerRunner{
		WorkerSubAgent: &stubRunner{kind: WorkerSubAgent, delay: 30 * time.Millisecond},
	}
	s, _, _ := newTestScheduler(t, map[WorkerType]int{WorkerSubAgent: 1}, runners)

	graph := NewTaskGraph([]TaskNode{
		{ID: "a", WorkerType: WorkerSubAgent, ContextPolicy: ContextFresh, Directive: "a"},
		{ID: "b", WorkerType: WorkerSubAgent, ContextPolicy: ContextFresh, Directive: "b", DependsOn: []string{"a"}},
		{ID: "c", WorkerType: WorkerSubAgent, ContextPolicy: ContextFresh, Directive: "c", DependsOn: []string{"b"}},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Start(ctx, "sess-slot", graph); err != nil {
		t.Fatalf("Start: %v", err)
	}
	artifacts, err := s.WaitForCompletion(ctx, "sess-slot")
	if err != nil {
		t.Fatalf("WaitForCompletion: %v", err)
	}
	if len(artifacts) != 3 {
		t.Fatalf("expected 3 artifacts, got %d", len(artifacts))
	}
}
