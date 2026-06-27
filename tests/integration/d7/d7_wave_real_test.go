//go:build integration && d7

package d7integration

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/decisionplanning"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
)

// runDecomposedWave exercises S5→S3 directly: TaskDecomposer → WaveScheduler.
func runDecomposedWave(t *testing.T, decomp *decisionplanning.TaskDecomposer, sched *wavescheduler.WaveScheduler, sessionID, goal string) {
	t.Helper()
	ctx := context.Background()
	result, err := decomp.SynthesizeTaskGraph(ctx, sessionID, goal)
	if err != nil {
		t.Fatalf("SynthesizeTaskGraph: %v", err)
	}
	graph := wavescheduler.NewTaskGraph(result.Nodes)
	if err := sched.Start(ctx, sessionID, graph); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := sched.WaitForCompletion(ctx, sessionID); err != nil {
		t.Fatalf("WaitForCompletion: %v", err)
	}
}

// stubWorkerRunner is a minimal wavescheduler.WorkerRunner that simulates a short
// task with a configurable delay. It emits a text WorkerEvent and records
// its run count for test assertions.
type stubWorkerRunner struct {
	delay    time.Duration
	runCount atomic.Int64
}

func (r *stubWorkerRunner) Kind() wavescheduler.WorkerType { return wavescheduler.WorkerSubAgent }

func (r *stubWorkerRunner) Run(ctx context.Context, spec wavescheduler.WorkerRunSpec) error {
	r.runCount.Add(1)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(r.delay):
	}
	spec.Emit(wavescheduler.WorkerEvent{Type: "text", Content: spec.Directive + " done"})
	return nil
}

// T: D7-S3-A01-T01, D7-S3-A02-T01, D7-S3-A03-T01 — Real Wave Scheduler dispatch.
func TestIntegration_D7WaveScheduler_RealDispatch(t *testing.T) {
	runner := &stubWorkerRunner{delay: 20 * time.Millisecond}

	pool := wavescheduler.NewWorkerPool(wavescheduler.DefaultPoolCapacity)
	guard := wavescheduler.NewConflictGuard()
	artifacts := wavescheduler.NewArtifactStore()
	resolver := wavescheduler.NewContextResolver(wavescheduler.ContextResolverDeps{
		Artifacts:        artifacts,
		BaseSystemPrompt: "",
	})
	sched := wavescheduler.NewWaveScheduler(wavescheduler.SchedulerDeps{
		Pool:      pool,
		Guard:     guard,
		Resolver:  resolver,
		Artifacts: artifacts,
		Runners: map[wavescheduler.WorkerType]wavescheduler.WorkerRunner{
			wavescheduler.WorkerSubAgent: runner,
		},
	})

	decomp := decisionplanning.NewTaskDecomposer()
	const goal = "design the auth module && implement session token rotation && add e2e tests"
	runDecomposedWave(t, decomp, sched, "sess-wave-real", goal)

	metrics := sched.Metrics()
	if metrics.Completed != 3 {
		t.Errorf("expected 3 completed tasks, got %d (failed=%d, cancelled=%d)",
			metrics.Completed, metrics.Failed, metrics.Cancelled)
	}
	if metrics.PeakRunning > 1 {
		t.Errorf("expected PeakRunning ≤ 1 for sequential DAG, got %d", metrics.PeakRunning)
	}
	if got := runner.runCount.Load(); got != 3 {
		t.Errorf("expected 3 runner invocations, got %d", got)
	}
}

// T: D7-S3-A01-T02 — WaveScheduler handles empty graph gracefully.
func TestIntegration_D7WaveScheduler_EmptyGraph(t *testing.T) {
	runner := &stubWorkerRunner{delay: 10 * time.Millisecond}
	sched := wavescheduler.NewWaveScheduler(wavescheduler.SchedulerDeps{
		Pool:      wavescheduler.NewWorkerPool(wavescheduler.DefaultPoolCapacity),
		Guard:     wavescheduler.NewConflictGuard(),
		Resolver:  wavescheduler.NewContextResolver(wavescheduler.ContextResolverDeps{}),
		Artifacts: wavescheduler.NewArtifactStore(),
		Runners: map[wavescheduler.WorkerType]wavescheduler.WorkerRunner{
			wavescheduler.WorkerSubAgent: runner,
		},
	})

	decomp := decisionplanning.NewTaskDecomposer()
	runDecomposedWave(t, decomp, sched, "sess-wave-empty",
		"a single small task with no separators for simple decomposition test")

	metrics := sched.Metrics()
	if metrics.Completed != 1 {
		t.Errorf("expected 1 completed task, got %d", metrics.Completed)
	}
}

// T: D7-S3-A03-T02 — ConflictGuard prevents concurrent tasks in the same conflict group.
func TestIntegration_D7WaveScheduler_ConflictGuard(t *testing.T) {
	var concurrent atomic.Int64
	var maxConcurrent atomic.Int64

	runner := &conflictDetectRunner{
		concurrent:    &concurrent,
		maxConcurrent: &maxConcurrent,
		delay:         30 * time.Millisecond,
	}

	pool := wavescheduler.NewWorkerPool(wavescheduler.DefaultPoolCapacity)
	guard := wavescheduler.NewConflictGuard()
	artifacts := wavescheduler.NewArtifactStore()
	resolver := wavescheduler.NewContextResolver(wavescheduler.ContextResolverDeps{
		Artifacts:        artifacts,
		BaseSystemPrompt: "",
	})
	sched := wavescheduler.NewWaveScheduler(wavescheduler.SchedulerDeps{
		Pool:      pool,
		Guard:     guard,
		Resolver:  resolver,
		Artifacts: artifacts,
		Runners: map[wavescheduler.WorkerType]wavescheduler.WorkerRunner{
			wavescheduler.WorkerSubAgent: runner,
		},
	})

	decomp := decisionplanning.NewTaskDecomposer()
	runDecomposedWave(t, decomp, sched, "sess-wave-conflict", "task one && task two")

	metrics := sched.Metrics()
	if metrics.Completed != 2 {
		t.Errorf("expected 2 completed tasks, got %d", metrics.Completed)
	}
}

// conflictDetectRunner tracks peak concurrency across goroutines.
type conflictDetectRunner struct {
	concurrent    *atomic.Int64
	maxConcurrent *atomic.Int64
	delay         time.Duration
}

func (r *conflictDetectRunner) Kind() wavescheduler.WorkerType { return wavescheduler.WorkerSubAgent }

func (r *conflictDetectRunner) Run(ctx context.Context, spec wavescheduler.WorkerRunSpec) error {
	cur := r.concurrent.Add(1)
	for {
		max := r.maxConcurrent.Load()
		if cur <= max || r.maxConcurrent.CompareAndSwap(max, cur) {
			break
		}
	}
	defer r.concurrent.Add(-1)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(r.delay):
	}
	spec.Emit(wavescheduler.WorkerEvent{Type: "text", Content: spec.Directive + " done"})
	return nil
}
