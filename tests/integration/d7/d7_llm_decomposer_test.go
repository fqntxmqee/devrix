//go:build integration && d7

package d7integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/coordinator"
	"github.com/devrix/devrix/internal/layers/orchestration/turn"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/tests/testutil"
)

// stubLLMInvoker implements turn.LLMInvoker by returning a pre-canned JSON
// DAG string. Used to test the LLMDecomposer without a real LLM.
type stubLLMInvoker struct {
	jsonDAG string
}

func (s *stubLLMInvoker) InvokeStream(ctx context.Context, req turn.LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
	ch := make(chan llmgateway.Chunk, 2)
	if s.jsonDAG != "" {
		ch <- llmgateway.Chunk{Content: s.jsonDAG}
	}
	ch <- llmgateway.Chunk{Done: true, Usage: llmgateway.TokenUsage{PromptTokens: 50, CompletionTokens: 30}}
	close(ch)
	return ch, nil
}

// T: D7-S5-A02-T01, D7-S5-A03-T01 — LLM Decomposer end-to-end (DM-020 v1.0
// LLM-augmented decomposition → WaveScheduler).
//
// Verifies the S5→S3 critical chain: stub LLMInvoker returns a JSON DAG →
// LLMDecomposer.Decompose → parseDecomposedTasks → TaskDecomposer.SynthesizeTaskGraph
// → WaveScheduler dispatches and collects artifacts.
//
// The DAG is a chain: design → implement → test, which forces sequential
// execution (PeakRunning must be 1).
func TestIntegration_D7LLMDecomposer_EndToEnd(t *testing.T) {
	// JSON DAG: 3 tasks with chain dependency (design → implement → test).
	const jsonDAG = `[
		{
			"id": "design",
			"title": "Design auth module",
			"directive": "design the auth module architecture",
			"worker_type": "subagent",
			"context_policy": "fresh"
		},
		{
			"id": "implement",
			"title": "Implement auth module",
			"directive": "implement the auth module",
			"worker_type": "subagent",
			"context_policy": "fresh",
			"depends_on": ["design"]
		},
		{
			"id": "test",
			"title": "Add tests",
			"directive": "add e2e tests for auth",
			"worker_type": "subagent",
			"context_policy": "fresh",
			"depends_on": ["implement"]
		}
	]`

	llmInvoker := &stubLLMInvoker{jsonDAG: jsonDAG}
	llmDecomp := coordinator.NewLLMDecomposer(coordinator.LLMDecomposerDeps{
		LLM:         llmInvoker,
		DefaultTier: "default",
	})

	runner := &stubWorkerRunner{delay: 10 * time.Millisecond}
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

	decomp := coordinator.NewTaskDecomposer()
	decomp.SetLLMDecomposer(llmDecomp)

	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{
		RoutingMode: "rule_orchestrate",
		LLMStub: &testutil.D7LLMStub{Response: "should-not-be-called"},
		OverrideOrchestratePath: coordinator.NewOrchestratePath(decomp, sched, nil),
	})

	session, err := stack.Gateway.CreateSession("cli", stack.WorkDir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Any long message (> 32 chars, no fast pattern) → IntentOrchestrate →
	// LLMDecomposer (since the message itself won't match && separator,
	// the rule-based fallback is NOT the primary path — LLMDecomposer runs
	// first with a 5s timeout).
	routeAndWait(t, stack, session.SessionID, "build an auth module with comprehensive tests")

	// Verify: 3 tasks, LLM-decomposed, sequential execution.
	metrics := sched.Metrics()
	if metrics.Completed != 3 {
		t.Errorf("expected 3 completed tasks, got %d (failed=%d, cancelled=%d)",
			metrics.Completed, metrics.Failed, metrics.Cancelled)
	}
	// Chain DAG: design→implement→test must be strictly sequential.
	if metrics.PeakRunning != 1 {
		t.Errorf("expected PeakRunning == 1 for chain DAG, got %d", metrics.PeakRunning)
	}
	if got := runner.runCount.Load(); got != 3 {
		t.Errorf("expected 3 runner invocations, got %d", got)
	}

	// Verify artifact summaries appear in outbound.
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

// T: D7-S5-A02-T02 — LLMDecomposer fallback to rule-based when JSON is invalid.
//
// Verifies that when the LLM returns malformed JSON, the decomposer falls back
// to rule-based decomposition (TaskDecomposer.SynthesizeTaskGraph does this
// internally: LLM path fails → nodes=nil → rule-based fallback).
func TestIntegration_D7LLMDecomposer_FallbackOnInvalidJSON(t *testing.T) {
	// Return invalid JSON (missing closing bracket).
	llmInvoker := &stubLLMInvoker{jsonDAG: `[`} // deliberately broken
	llmDecomp := coordinator.NewLLMDecomposer(coordinator.LLMDecomposerDeps{
		LLM: llmInvoker,
	})

	runner := &stubWorkerRunner{delay: 10 * time.Millisecond}
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

	decomp := coordinator.NewTaskDecomposer()
	decomp.SetLLMDecomposer(llmDecomp)

	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{
		RoutingMode: "rule_orchestrate",
		LLMStub: &testutil.D7LLMStub{Response: "should-not-be-called"},
		OverrideOrchestratePath: coordinator.NewOrchestratePath(decomp, sched, nil),
	})

	session, err := stack.Gateway.CreateSession("cli", stack.WorkDir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Message with && separator → rule-based fallback after LLM JSON parse fails.
	routeAndWait(t, stack, session.SessionID, "task A && task B")

	metrics := sched.Metrics()
	if metrics.Completed != 2 {
		t.Errorf("expected 2 completed tasks from rule-based fallback, got %d (failed=%d)",
			metrics.Completed, metrics.Failed)
	}
}

// T: D7-S5-A02-T03 — LLM Decomposer handles empty task list.
func TestIntegration_D7LLMDecomposer_EmptyTaskList(t *testing.T) {
	// Return empty JSON array — parseDecomposedTasks returns error.
	llmInvoker := &stubLLMInvoker{jsonDAG: `[]`}
	llmDecomp := coordinator.NewLLMDecomposer(coordinator.LLMDecomposerDeps{
		LLM: llmInvoker,
	})

	runner := &stubWorkerRunner{delay: 10 * time.Millisecond}
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

	decomp := coordinator.NewTaskDecomposer()
	decomp.SetLLMDecomposer(llmDecomp)

	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{
		RoutingMode: "rule_orchestrate",
		LLMStub: &testutil.D7LLMStub{Response: "should-not-be-called"},
		OverrideOrchestratePath: coordinator.NewOrchestratePath(decomp, sched, nil),
	})

	session, err := stack.Gateway.CreateSession("cli", stack.WorkDir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Message with && → rule-based fallback since LLM returns empty array.
	routeAndWait(t, stack, session.SessionID, "task X && task Y")

	metrics := sched.Metrics()
	if metrics.Completed != 2 {
		t.Errorf("expected 2 completed tasks from rule-based fallback, got %d", metrics.Completed)
	}
}

// T: D7-S5-A02-T04 — LLMDecomposer splits text without JSON.
//
// Verifies that when the LLM returns prose without a JSON array, the
// decomposer correctly falls back to rule-based decomposition.
func TestIntegration_D7LLMDecomposer_NoJSONInResponse(t *testing.T) {
	// Return prose text without any JSON array.
	llmInvoker := &stubLLMInvoker{jsonDAG: `Sure, I'll help you break this down into tasks.

Here is my plan:
1. First, design the module
2. Then implement it
3. Finally, test everything`}
	llmDecomp := coordinator.NewLLMDecomposer(coordinator.LLMDecomposerDeps{
		LLM: llmInvoker,
	})

	runner := &stubWorkerRunner{delay: 10 * time.Millisecond}
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

	decomp := coordinator.NewTaskDecomposer()
	decomp.SetLLMDecomposer(llmDecomp)

	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{
		RoutingMode: "rule_orchestrate",
		LLMStub: &testutil.D7LLMStub{Response: "should-not-be-called"},
		OverrideOrchestratePath: coordinator.NewOrchestratePath(decomp, sched, nil),
	})

	session, err := stack.Gateway.CreateSession("cli", stack.WorkDir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	routeAndWait(t, stack, session.SessionID, "single task with no separators for json fallback verification")

	metrics := sched.Metrics()
	if metrics.Completed != 1 {
		t.Errorf("expected 1 completed task from rule-based fallback, got %d", metrics.Completed)
	}
}

// Compile-time guard: stubLLMInvoker must implement turn.LLMInvoker.
var _ turn.LLMInvoker = (*stubLLMInvoker)(nil)
