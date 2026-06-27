//go:build integration && d7

package d7integration

import (
	"context"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/decisionplanning"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
)

// stubLLMInvoker implements orchtypes.LLMInvoker by returning a pre-canned JSON
// DAG string. Used to test the LLMDecomposer without a real LLM.
type stubLLMInvoker struct {
	jsonDAG string
}

func (s *stubLLMInvoker) InvokeStream(ctx context.Context, req orchtypes.LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
	ch := make(chan llmgateway.Chunk, 2)
	if s.jsonDAG != "" {
		ch <- llmgateway.Chunk{Content: s.jsonDAG}
	}
	ch <- llmgateway.Chunk{Done: true, Usage: llmgateway.TokenUsage{PromptTokens: 50, CompletionTokens: 30}}
	close(ch)
	return ch, nil
}

func newWaveSchedulerWithStubRunner(t *testing.T, delay time.Duration) (*wavescheduler.WaveScheduler, *stubWorkerRunner) {
	t.Helper()
	runner := &stubWorkerRunner{delay: delay}
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
	return sched, runner
}

// T: D7-S5-A02-T01, D7-S5-A03-T01 — LLM Decomposer end-to-end (S5→S3).
func TestIntegration_D7LLMDecomposer_EndToEnd(t *testing.T) {
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

	llmDecomp := decisionplanning.NewLLMDecomposer(decisionplanning.LLMDecomposerDeps{
		LLM:         &stubLLMInvoker{jsonDAG: jsonDAG},
		DefaultTier: "default",
	})
	sched, runner := newWaveSchedulerWithStubRunner(t, 10*time.Millisecond)
	decomp := decisionplanning.NewTaskDecomposer()
	decomp.SetLLMDecomposer(llmDecomp)

	runDecomposedWave(t, decomp, sched, "sess-llm-decomp", "build an auth module with comprehensive tests")

	metrics := sched.Metrics()
	if metrics.Completed != 3 {
		t.Errorf("expected 3 completed tasks, got %d (failed=%d, cancelled=%d)",
			metrics.Completed, metrics.Failed, metrics.Cancelled)
	}
	if metrics.PeakRunning != 1 {
		t.Errorf("expected PeakRunning == 1 for chain DAG, got %d", metrics.PeakRunning)
	}
	if got := runner.runCount.Load(); got != 3 {
		t.Errorf("expected 3 runner invocations, got %d", got)
	}
}

// T: D7-S5-A02-T02 — LLMDecomposer fallback to rule-based when JSON is invalid.
func TestIntegration_D7LLMDecomposer_FallbackOnInvalidJSON(t *testing.T) {
	llmDecomp := decisionplanning.NewLLMDecomposer(decisionplanning.LLMDecomposerDeps{
		LLM: &stubLLMInvoker{jsonDAG: `[`},
	})
	sched, _ := newWaveSchedulerWithStubRunner(t, 10*time.Millisecond)
	decomp := decisionplanning.NewTaskDecomposer()
	decomp.SetLLMDecomposer(llmDecomp)

	runDecomposedWave(t, decomp, sched, "sess-llm-fallback", "task A && task B")

	metrics := sched.Metrics()
	if metrics.Completed != 2 {
		t.Errorf("expected 2 completed tasks from rule-based fallback, got %d (failed=%d)",
			metrics.Completed, metrics.Failed)
	}
}

// T: D7-S5-A02-T03 — LLM Decomposer handles empty task list.
func TestIntegration_D7LLMDecomposer_EmptyTaskList(t *testing.T) {
	llmDecomp := decisionplanning.NewLLMDecomposer(decisionplanning.LLMDecomposerDeps{
		LLM: &stubLLMInvoker{jsonDAG: `[]`},
	})
	sched, _ := newWaveSchedulerWithStubRunner(t, 10*time.Millisecond)
	decomp := decisionplanning.NewTaskDecomposer()
	decomp.SetLLMDecomposer(llmDecomp)

	runDecomposedWave(t, decomp, sched, "sess-llm-empty", "task X && task Y")

	metrics := sched.Metrics()
	if metrics.Completed != 2 {
		t.Errorf("expected 2 completed tasks from rule-based fallback, got %d", metrics.Completed)
	}
}

// T: D7-S5-A02-T04 — LLMDecomposer splits text without JSON.
func TestIntegration_D7LLMDecomposer_NoJSONInResponse(t *testing.T) {
	llmDecomp := decisionplanning.NewLLMDecomposer(decisionplanning.LLMDecomposerDeps{
		LLM: &stubLLMInvoker{jsonDAG: `Sure, I'll help you break this down into tasks.`},
	})
	sched, _ := newWaveSchedulerWithStubRunner(t, 10*time.Millisecond)
	decomp := decisionplanning.NewTaskDecomposer()
	decomp.SetLLMDecomposer(llmDecomp)

	runDecomposedWave(t, decomp, sched, "sess-llm-nojson",
		"single task with no separators for json fallback verification")

	metrics := sched.Metrics()
	if metrics.Completed != 1 {
		t.Errorf("expected 1 completed task from rule-based fallback, got %d", metrics.Completed)
	}
}

// Compile-time guard: stubLLMInvoker must implement orchtypes.LLMInvoker.
var _ orchtypes.LLMInvoker = (*stubLLMInvoker)(nil)
