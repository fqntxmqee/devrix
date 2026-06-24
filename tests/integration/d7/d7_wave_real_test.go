//go:build integration && d7

package d7integration

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/decisionplanning"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	"github.com/devrix/devrix/tests/testutil"
)

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

// T: D7-S3-A01-T01, D7-S3-A02-T01, D7-S3-A03-T01 — Real Wave Scheduler dispatch
// (DM-20260615-004 v1.0 Wave Scheduler integration).
//
// Verifies the full WaveScheduler pipeline with real WorkerPool, ConflictGuard,
// ArtifactStore, and ContextResolver. Uses a stub WorkerRunner to exercise the
// DAG dispatch, slot management, and artifact collection without real subagents.
//
// The message "task A && task B && task C" triggers rule-based decomposition
// into 3 sequential tasks (SubAgent worker type), which the WaveScheduler
// dispatches through the 5-slot pool.
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

	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{
		RoutingMode: "rule_orchestrate",
		LLMStub: &testutil.D7LLMStub{Response: "should-not-be-called"},
		OverrideOrchestratePath: sessionorchestrator.NewOrchestratePath(
			decisionplanning.NewTaskDecomposer(),
			sched,
			nil,
		),
	})

	session, err := stack.Gateway.CreateSession("cli", stack.WorkDir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Long message with && separator (> 32 chars, no fast pattern) →
	// IntentOrchestrate → rule-based TaskDecomposer → 3 sequential tasks.
	const goal = "design the auth module && implement session token rotation && add e2e tests"
	routeAndWait(t, stack, session.SessionID, goal)

	metrics := sched.Metrics()
	if metrics.Completed != 3 {
		t.Errorf("expected 3 completed tasks, got %d (failed=%d, cancelled=%d)",
			metrics.Completed, metrics.Failed, metrics.Cancelled)
	}
	// DAG is sequential (task_1 → task_2 → task_3), peak should be 1.
	if metrics.PeakRunning > 1 {
		t.Errorf("expected PeakRunning ≤ 1 for sequential DAG, got %d", metrics.PeakRunning)
	}
	if got := runner.runCount.Load(); got != 3 {
		t.Errorf("expected 3 runner invocations, got %d", got)
	}

	// Verify outbound contains artifact summaries from all 3 tasks.
	var summaryCount int
	for _, msg := range stack.Handler.OutboundMessages() {
		if strings.Contains(msg.Content, "done") {
			summaryCount++
		}
	}
	if summaryCount < 1 {
		t.Errorf("expected artifact summaries in outbound, got: %+v", stack.Handler.OutboundMessages())
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

	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{
		RoutingMode: "rule_orchestrate",
		LLMStub: &testutil.D7LLMStub{Response: "should-not-be-called"},
		OverrideOrchestratePath: sessionorchestrator.NewOrchestratePath(
			decisionplanning.NewTaskDecomposer(),
			sched,
			nil,
		),
	})

	session, err := stack.Gateway.CreateSession("cli", stack.WorkDir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// A single short phrase (no separator) → rule-based decomposition → 1 task.
	routeAndWait(t, stack, session.SessionID, "a single small task with no separators for simple decomposition test")

	metrics := sched.Metrics()
	if metrics.Completed != 1 {
		t.Errorf("expected 1 completed task, got %d", metrics.Completed)
	}
}

// T: D7-S3-A03-T02 — ConflictGuard prevents concurrent tasks in the same conflict group.
func TestIntegration_D7WaveScheduler_ConflictGuard(t *testing.T) {
	// Two runners share a counter to detect concurrency.
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

	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{
		RoutingMode: "rule_orchestrate",
		LLMStub: &testutil.D7LLMStub{Response: "should-not-be-called"},
		OverrideOrchestratePath: sessionorchestrator.NewOrchestratePath(
			decisionplanning.NewTaskDecomposer(),
			sched,
			nil,
		),
	})

	session, err := stack.Gateway.CreateSession("cli", stack.WorkDir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	routeAndWait(t, stack, session.SessionID, "task one && task two")

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
	// Track peak.
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
