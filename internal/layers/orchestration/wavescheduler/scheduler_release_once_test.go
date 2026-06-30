package wavescheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// T: D7-S3-A84-T01 (DM-20260630-013 RH-D7-02)
//
// OnReleaseOnce — repeated Start() MUST NOT append additional OnRelease
// hooks to the underlying WorkerPool. Before the fix, every dispatchLoop
// registered its own wakeup hook AND Start() appended a no-op, so the
// hook slice grew unbounded across waves and every Release fanned out to
// O(n) goroutines writing to defunct channels.
func TestWaveScheduler_OnRelease_HookCountInvariant(t *testing.T) {
	runners := map[WorkerType]WorkerRunner{
		WorkerSubAgent: &stubRunner{kind: WorkerSubAgent, delay: 5 * time.Millisecond},
	}
	pool := NewWorkerPool(map[WorkerType]int{WorkerSubAgent: 2})
	guard := NewConflictGuard()
	artifacts := NewArtifactStore()
	resolver := NewContextResolver(ContextResolverDeps{Artifacts: artifacts, BaseSystemPrompt: "test"})

	s := NewWaveScheduler(SchedulerDeps{
		Pool:      pool,
		Guard:     guard,
		Resolver:  resolver,
		Artifacts: artifacts,
		Runners:   runners,
	})

	// Baseline: NewWaveScheduler registers exactly one hook.
	if got := pool.HookCount(); got != 1 {
		t.Fatalf("HookCount after NewWaveScheduler = %d, want 1 (RH-D7-02 baseline)", got)
	}

	// Drive 100 Start cycles — same sessionID with cancel in between so the
	// dispatch loop actually tears down. Before the fix this grew the hook
	// slice to >1 (typically 1 from New + 1 from Start + 1 per dispatchLoop).
	for i := 0; i < 100; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		graph := NewTaskGraph([]TaskNode{
			{ID: "t", WorkerType: WorkerSubAgent, ContextPolicy: ContextFresh, Directive: "t"},
		})
		if err := s.Start(ctx, "sess-inv", graph); err != nil {
			cancel()
			t.Fatalf("Start#%d: %v", i, err)
		}
		if _, err := s.WaitForCompletion(ctx, "sess-inv"); err != nil {
			cancel()
			t.Fatalf("Wait#%d: %v", i, err)
		}
		cancel()
	}

	if got := pool.HookCount(); got != 1 {
		t.Errorf("HookCount after 100 Start cycles = %d, want 1 (D7-S3-A84 invariant)", got)
	}
}

// T: D7-S3-A84-T01 (DM-20260630-013 RH-D7-02)
//
// Distinct sessionIDs — confirms the invariant holds even when the
// dispatch loop spins up per-session with cancel in between.
func TestWaveScheduler_OnRelease_HookCount_DistinctSessions(t *testing.T) {
	runners := map[WorkerType]WorkerRunner{
		WorkerSubAgent: &stubRunner{kind: WorkerSubAgent, delay: 5 * time.Millisecond},
	}
	pool := NewWorkerPool(map[WorkerType]int{WorkerSubAgent: 2})
	guard := NewConflictGuard()
	artifacts := NewArtifactStore()
	resolver := NewContextResolver(ContextResolverDeps{Artifacts: artifacts, BaseSystemPrompt: "test"})

	s := NewWaveScheduler(SchedulerDeps{
		Pool:      pool,
		Guard:     guard,
		Resolver:  resolver,
		Artifacts: artifacts,
		Runners:   runners,
	})

	for i := 0; i < 50; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		graph := NewTaskGraph([]TaskNode{
			{ID: "t", WorkerType: WorkerSubAgent, ContextPolicy: ContextFresh, Directive: "t"},
		})
		sid := "sess-distinct-" + string(rune('A'+i%26)) + "-" + string(rune('0'+i%10))
		if err := s.Start(ctx, sid, graph); err != nil {
			cancel()
			t.Fatalf("Start#%d: %v", i, err)
		}
		if _, err := s.WaitForCompletion(ctx, sid); err != nil {
			cancel()
			t.Fatalf("Wait#%d: %v", i, err)
		}
		cancel()
	}

	if got := pool.HookCount(); got != 1 {
		t.Errorf("HookCount across 50 distinct sessions = %d, want 1", got)
	}
}

// T: D7-S3-A84-T01 (DM-20260630-013 RH-D7-02)
//
// Each slot Release triggers at most ONE wakeup signal. We can't observe
// the wakeupCh directly from outside the scheduler, but we can verify
// the goroutine fan-out: with N hooks registered, a single Release would
// spawn N goroutines. We hook an extra counter via a second OnRelease
// AFTER construction (legitimate use) and confirm that only 2 hooks fire
// per Release — not 3+ (which would indicate the production path
// re-registered itself).
func TestWaveScheduler_OnRelease_FiresOncePerRelease(t *testing.T) {
	runners := map[WorkerType]WorkerRunner{
		WorkerSubAgent: &stubRunner{kind: WorkerSubAgent, delay: 20 * time.Millisecond},
	}
	pool := NewWorkerPool(map[WorkerType]int{WorkerSubAgent: 1})
	guard := NewConflictGuard()
	artifacts := NewArtifactStore()
	resolver := NewContextResolver(ContextResolverDeps{Artifacts: artifacts, BaseSystemPrompt: "test"})

	s := NewWaveScheduler(SchedulerDeps{
		Pool:      pool,
		Guard:     guard,
		Resolver:  resolver,
		Artifacts: artifacts,
		Runners:   runners,
	})

	// Add a sentinel hook that counts releases. The production hook
	// (registered by NewWaveScheduler) and this sentinel should be the
	// ONLY hooks — total = 2.
	var releases atomic.Int64
	pool.OnRelease(func(_ SlotID) {
		releases.Add(1)
	})

	if got := pool.HookCount(); got != 2 {
		t.Fatalf("HookCount = %d, want 2 (production + sentinel)", got)
	}

	// Run a single wave to trigger one Release. The hook fires async, so
	// poll briefly.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	graph := NewTaskGraph([]TaskNode{
		{ID: "t", WorkerType: WorkerSubAgent, ContextPolicy: ContextFresh, Directive: "t"},
	})
	if err := s.Start(ctx, "sess-fires-once", graph); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := s.WaitForCompletion(ctx, "sess-fires-once"); err != nil {
		t.Fatalf("Wait: %v", err)
	}

	// Allow the async hooks to settle.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if releases.Load() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := releases.Load(); got != 1 {
		t.Errorf("sentinel release count = %d, want 1 (one Release → one hook fire)", got)
	}
}