// OrchestratePath is the D7-S2 IntentOrchestrate explicit-orchestration
// path.
//
// v1.0 closure (2026-06-15) routed IntentOrchestrate to FastPath.Run with
// a "[orchestrate: please decompose and execute step by step]" system
// prompt, letting the LLM decompose the goal and execute in a single
// loop. That was a v1.0 simplification; v1.1+ dispatches to:
//
//  1. TaskDecomposer.SynthesizeTaskGraph (D7-S5-A02) — goal → TaskNode DAG
//  2. WaveScheduler.Start (D7-S3-A01) — DAG → 5-slot pool dispatch
//  3. WaveScheduler.WaitForCompletion — block until all nodes terminal
//  4. Stream FlowEvents back to the caller as EngineEvents
//
// Wiring: bootstrap injects TaskDecomposer + WaveScheduler via
// NewOrchestratePath. ProcessMessage's IntentOrchestrate case calls
// Run directly (no FastPath).
package coordinator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/wave"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// WaveSchedulerRunner abstracts the WaveScheduler methods OrchestratePath
// needs. The concrete *wave.WaveScheduler satisfies this interface;
// tests can inject a fake.
type WaveSchedulerRunner interface {
	Start(ctx context.Context, sessionID string, graph *wave.TaskGraph) error
	WaitForCompletion(ctx context.Context, sessionID string) ([]wave.Artifact, error)
}

// OrchestratePath runs the D7-S5-A02 + D7-S3-A01 pipeline for
// IntentOrchestrate messages.
type OrchestratePath struct {
	decomposer *TaskDecomposer
	scheduler  WaveSchedulerRunner
	sink       EventPublisher
}

// NewOrchestratePath builds the path. All args are required; nil →
// Run returns an error at the call site (no FastPath fallback).
func NewOrchestratePath(
	decomposer *TaskDecomposer,
	scheduler WaveSchedulerRunner,
	sink EventPublisher,
) *OrchestratePath {
	return &OrchestratePath{decomposer: decomposer, scheduler: scheduler, sink: sink}
}

// Run executes the orchestrate pipeline asynchronously. Returned channel
// emits:
//
//	plan_formed (count of nodes) → wave_started →
//	[per-node text events as FlowEvents arrive] → text (artifact summary)
//	→ complete
//
// Run does NOT call the LLM for execution; the Wave workers handle that.
// Only TaskDecomposer.SynthesizeTaskGraph may invoke an LLM (LLM-augmented
// decomposition with 5s timeout; rule-based fallback otherwise).
func (op *OrchestratePath) Run(ctx context.Context, req ProcessRequest, _ IntentClassification) (<-chan *contracts.EngineEvent, error) {
	if op == nil {
		return nil, fmt.Errorf("orchestrator: OrchestratePath is nil (bootstrap missing wiring)")
	}
	if op.decomposer == nil {
		return nil, fmt.Errorf("orchestrator: OrchestratePath.decomposer is nil (bootstrap missing TaskDecomposer)")
	}
	if op.scheduler == nil {
		return nil, fmt.Errorf("orchestrator: OrchestratePath.scheduler is nil (bootstrap missing WaveScheduler)")
	}

	out := make(chan *contracts.EngineEvent, 16)
	go func() {
		defer close(out)

		// 1) SynthesizeTaskGraph (D7-S5-A02)
		result, err := op.decomposer.SynthesizeTaskGraph(ctx, req.SessionID, req.Message)
		if err != nil {
			emitError(ctx, op.sink, out, req.SessionID, "synthesize_task_graph", err)
			return
		}
		if result.Validation != nil && !result.Validation.Valid {
			emitError(ctx, op.sink, out, req.SessionID, "validate_task_graph",
				fmt.Errorf("graph invalid: %v", result.Validation.Errors))
			return
		}
		emit(ctx, op.sink, out, &contracts.EngineEvent{
			Type:      "plan_formed",
			Content:   fmt.Sprintf("%d tasks synthesized", len(result.Nodes)),
			SessionID: req.SessionID,
		})

		// 2) Build TaskGraph (D7-S3)
		graph := buildTaskGraph(result.Nodes)

		// 3) WaveScheduler.Start
		if err := op.scheduler.Start(ctx, req.SessionID, graph); err != nil {
			emitError(ctx, op.sink, out, req.SessionID, "wave_start", err)
			return
		}
		emit(ctx, op.sink, out, &contracts.EngineEvent{
			Type:      "wave_started",
			Content:   fmt.Sprintf("wave dispatched (%d tasks)", len(result.Nodes)),
			SessionID: req.SessionID,
		})

		// 4) Wait for completion (D7-S3)
		artifacts, err := op.scheduler.WaitForCompletion(ctx, req.SessionID)
		if err != nil {
			emitError(ctx, op.sink, out, req.SessionID, "wave_wait", err)
			return
		}

		// 5) Summarize artifacts and emit terminal events
		summary := summarizeArtifacts(artifacts)
		emit(ctx, op.sink, out, &contracts.EngineEvent{
			Type:      "text",
			Content:   summary,
			SessionID: req.SessionID,
		})
		emit(ctx, op.sink, out, &contracts.EngineEvent{
			Type:      "complete",
			SessionID: req.SessionID,
		})
	}()
	return out, nil
}

// buildTaskGraph constructs a *wave.TaskGraph from decomposed TaskNodes.
// It performs the light validation required by WaveScheduler.Start (the
// full DAG validation lives in TaskDecomposer.validateGraph).
func buildTaskGraph(nodes []wave.TaskNode) *wave.TaskGraph {
	// The wave package exposes TaskGraph as a public type constructed by
	// the caller; the only exported constructor is NewTaskGraph(slice).
	return wave.NewTaskGraph(nodes)
}

// summarizeArtifacts produces a short text summary of wave artifacts,
// suitable for emitting as a single text EngineEvent.
func summarizeArtifacts(artifacts []wave.Artifact) string {
	if len(artifacts) == 0 {
		return "(no artifacts)"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Wave complete: %d task(s)\n", len(artifacts)))
	for i, a := range artifacts {
		// truncate summary to 120 chars
		s := a.Summary
		if len(s) > 120 {
			s = s[:117] + "..."
		}
		fmt.Fprintf(&sb, "- [%s] %s (exit=%d, %s)\n",
			a.TaskID, s, a.ExitCode, a.EndedAt.Sub(a.StartedAt).Round(time.Millisecond))
		_ = i
	}
	return sb.String()
}

// emit publishes an event to the sink (if any) and writes it to the
// channel, respecting ctx cancellation. Channel send is best-effort:
// on cancellation the goroutine returns and the channel will be closed
// by the deferred close above.
func emit(ctx context.Context, sink EventPublisher, out chan<- *contracts.EngineEvent, ev *contracts.EngineEvent) {
	if sink != nil {
		sink.Publish(ctx, ev)
	}
	select {
	case out <- ev:
	case <-ctx.Done():
	}
}

// emitError is a convenience for the error branches; emits a single
// error event and returns. Mirrors FastPath.Run error semantics: the
// channel is closed after the error event.
func emitError(ctx context.Context, sink EventPublisher, out chan<- *contracts.EngineEvent, sessionID, label string, err error) {
	emit(ctx, sink, out, &contracts.EngineEvent{
		Type:      "error",
		Content:   fmt.Sprintf("%s: %s", label, err.Error()),
		SessionID: sessionID,
	})
}

// newDefaultOrchestratePath builds an OrchestratePath bound to a fresh
// TaskDecomposer and a fresh WaveScheduler. It is the v1.1.0+ default
// for NewSessionOrchestrator when no WithOrchestratePath option is
// supplied.
//
// The default WaveScheduler is constructed with zero-value SchedulerDeps.
// wave.NewWaveScheduler is nil-safe for Pool / Guard / Resolver /
// Artifacts (only Runners is initialized to an empty map if nil).
// Production callers that need a real WorkerPool + WorkerRunner registry
// should still wire explicitly via WithOrchestratePath.
func newDefaultOrchestratePath(sink EventPublisher, llmDecomp LLMTaskDecomposer) *OrchestratePath {
	decomp := NewTaskDecomposer()
	if llmDecomp != nil {
		decomp.SetLLMDecomposer(llmDecomp)
	}
	sched := wave.NewWaveScheduler(wave.SchedulerDeps{})
	return NewOrchestratePath(decomp, sched, sink)
}
