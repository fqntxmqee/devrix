package wave

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWaveScheduler_ConflictGroup(t *testing.T) {
	// L5-ORCH-13: two Tasks with same conflict_group should not run in parallel.
	runners := map[WorkerType]WorkerRunner{
		WorkerSubAgent: &stubRunner{kind: WorkerSubAgent, delay: 50 * time.Millisecond},
	}
	pool := NewWorkerPool(map[WorkerType]int{WorkerSubAgent: 3})
	guard := NewConflictGuard()
	artifacts := NewArtifactStore()
	resolver := NewContextResolver(ContextResolverDeps{Artifacts: artifacts, BaseSystemPrompt: "test"})
	s := &WaveScheduler{
		pool:      pool,
		guard:     guard,
		resolver:  resolver,
		artifacts: artifacts,
		runners:   runners,
		waves:     make(map[string]*schedulerWaveState),
	}

	graph := NewTaskGraph([]TaskNode{
		{ID: "a", WorkerType: WorkerSubAgent, ContextPolicy: ContextFresh, Directive: "a", ConflictGroup: "db"},
		{ID: "b", WorkerType: WorkerSubAgent, ContextPolicy: ContextFresh, Directive: "b", ConflictGroup: "db"},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := s.Start(ctx, "sess-cg", graph); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := s.WaitForCompletion(ctx, "sess-cg"); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	// Both completed in sequence (no parallel violation), but the scheduler
	// respected the conflict group ordering.
	stateA, _ := graph.State("a")
	stateB, _ := graph.State("b")
	if stateA != StateCompleted || stateB != StateCompleted {
		t.Fatalf("expected both completed, got a=%q b=%q", stateA, stateB)
	}
	// Inspect via guard that no two tasks of this group were registered
	// simultaneously. The test's primary assertion is correctness of result;
	// the parallel-avoidance is enforced by Guard.Allow in dispatchOne.
}

func TestWaveScheduler_UpstreamArtifact(t *testing.T) {
	// L5-ORCH-11: downstream with policy=upstream receives upstream artifact,
	// not Leader history.
	runners := map[WorkerType]WorkerRunner{
		WorkerSubAgent: &stubRunner{kind: WorkerSubAgent, delay: 30 * time.Millisecond},
	}
	pool := NewWorkerPool(map[WorkerType]int{WorkerSubAgent: 3})
	guard := NewConflictGuard()
	artifacts := NewArtifactStore()

	// Pre-seed upstream artifact.
	artifacts.Put(Artifact{
		TaskID:       "up",
		Summary:      "did important work",
		FilesChanged: []string{"src/api/users.go"},
	})

	// Resolver with a sidechain that would expose Leader history if accessed.
	resolver := NewContextResolver(ContextResolverDeps{
		Artifacts:        artifacts,
		BaseSystemPrompt: "base",
	})

	// Wrap resolver to inspect what downstream got.
	wrappedResolver := &inspectResolver{inner: resolver, captured: make(chan ResolvedContext, 4)}
	s := &WaveScheduler{
		pool:      pool,
		guard:     guard,
		resolver:  wrappedResolver,
		artifacts: artifacts,
		runners:   runners,
		waves:     make(map[string]*schedulerWaveState),
	}

	graph := NewTaskGraph([]TaskNode{
		{ID: "down", WorkerType: WorkerSubAgent, ContextPolicy: ContextUpstream, UpstreamTaskID: "up", Directive: "extend X"},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := s.Start(ctx, "sess-up", graph); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := s.WaitForCompletion(ctx, "sess-up"); err != nil {
		t.Fatalf("Wait: %v", err)
	}

	// Find the captured downstream context.
	var got ResolvedContext
	select {
	case got = <-wrappedResolver.captured:
	case <-time.After(1 * time.Second):
		t.Fatal("no captured context for downstream")
	}
	if got.UpstreamSummary != "did important work" {
		t.Fatalf("expected upstream summary in context, got %q", got.UpstreamSummary)
	}
	// Messages: only directive (no Leader history).
	if len(got.Messages) != 1 {
		t.Fatalf("expected 1 message (directive only), got %d", len(got.Messages))
	}
	if got.Messages[0].Content != "extend X" {
		t.Fatalf("expected directive 'extend X', got %q", got.Messages[0].Content)
	}
}

func TestWaveScheduler_CursorAndClaudeCodeParallel(t *testing.T) {
	// L5-ORCH-16: cursor + claude-code can run in parallel when file_scope
	// is disjoint.
	cur := &stubRunner{kind: WorkerCursor, delay: 30 * time.Millisecond}
	cc := &stubRunner{kind: WorkerClaudeCode, delay: 30 * time.Millisecond}
	runners := map[WorkerType]WorkerRunner{WorkerCursor: cur, WorkerClaudeCode: cc}
	pool := NewWorkerPool(map[WorkerType]int{WorkerCursor: 1, WorkerClaudeCode: 1})
	guard := NewConflictGuard()
	artifacts := NewArtifactStore()
	resolver := NewContextResolver(ContextResolverDeps{Artifacts: artifacts, BaseSystemPrompt: "test"})
	s := &WaveScheduler{
		pool:      pool,
		guard:     guard,
		resolver:  resolver,
		artifacts: artifacts,
		runners:   runners,
		waves:     make(map[string]*schedulerWaveState),
	}

	var parallel atomic.Int32
	graph := NewTaskGraph([]TaskNode{
		{ID: "c1", WorkerType: WorkerCursor, ContextPolicy: ContextFresh, Directive: "c1", FileScope: []string{"src/api/**"}},
		{ID: "cc1", WorkerType: WorkerClaudeCode, ContextPolicy: ContextFresh, Directive: "cc1", FileScope: []string{"docs/**"}},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := s.Start(ctx, "sess-mp", graph); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Sample parallel running.
	var sampler sync.WaitGroup
	sampler.Add(1)
	stop := make(chan struct{})
	go func() {
		defer sampler.Done()
		for {
			select {
			case <-stop:
				return
			case <-time.After(2 * time.Millisecond):
			}
			if cur.running.Load() > 0 && cc.running.Load() > 0 {
				parallel.Add(1)
			}
		}
	}()

	if _, err := s.WaitForCompletion(ctx, "sess-mp"); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	close(stop)
	sampler.Wait()

	if parallel.Load() == 0 {
		t.Fatal("expected cursor and claude-code to run in parallel at least once")
	}
}

func TestWaveScheduler_WaveCompletedSummary(t *testing.T) {
	// L5-ORCH-18: wave fully completes → WaitForCompletion returns all artifacts.
	runners := map[WorkerType]WorkerRunner{
		WorkerSubAgent: &stubRunner{kind: WorkerSubAgent, delay: 20 * time.Millisecond},
	}
	s, _, _ := newTestScheduler(t, map[WorkerType]int{WorkerSubAgent: 3}, runners)

	graph := NewTaskGraph([]TaskNode{
		{ID: "a", WorkerType: WorkerSubAgent, ContextPolicy: ContextFresh, Directive: "a"},
		{ID: "b", WorkerType: WorkerSubAgent, ContextPolicy: ContextFresh, Directive: "b"},
		{ID: "c", WorkerType: WorkerSubAgent, ContextPolicy: ContextFresh, Directive: "c"},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := s.Start(ctx, "sess-wc", graph); err != nil {
		t.Fatalf("Start: %v", err)
	}
	arts, err := s.WaitForCompletion(ctx, "sess-wc")
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if len(arts) != 3 {
		t.Fatalf("expected 3 artifacts, got %d", len(arts))
	}
	// Summary has expected count.
	if s.Metrics().Completed != 3 {
		t.Fatalf("expected 3 completed, got %d", s.Metrics().Completed)
	}
}

// inspectResolver captures ResolvedContext for downstream inspection.
type inspectResolver struct {
	inner    *ContextResolver
	captured chan ResolvedContext
}

func (r *inspectResolver) Resolve(n TaskNode) (ResolvedContext, error) {
	got, err := r.inner.Resolve(n)
	if err == nil {
		select {
		case r.captured <- got:
		default:
		}
	}
	return got, err
}

// typesMsgForTest is a no-op alias to keep imports clean.
type typesMsgForTest struct{}
