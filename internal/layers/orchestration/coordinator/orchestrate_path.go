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

	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
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
	obsBridge  *observability.Bridge
}

// SetObsBridge wires tracing for the orchestrate pipeline.
func (op *OrchestratePath) SetObsBridge(bridge *observability.Bridge) {
	if op == nil {
		return
	}
	op.obsBridge = bridge
	if ws, ok := op.scheduler.(*wave.WaveScheduler); ok {
		ws.SetObsBridge(bridge)
	}
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

		ctx, orchSpan := startObsSpan(op.obsBridge, ctx, telemetry.OpD7_S2_Orchestration_Orchestrate_Run, tracer.SpanKindInternal,
			tracer.Attribute{Key: "session_id", Value: req.SessionID},
		)
		defer endSpan(orchSpan)

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

		if ws, ok := op.scheduler.(*wave.WaveScheduler); ok {
			ws.SetWorkerEventHandler(func(sessionID, taskID string, ev wave.WorkerEvent) {
				if engineEv := workerEventToEngine(sessionID, taskID, ev); engineEv != nil {
					emit(ctx, op.sink, out, engineEv)
				}
			})
		}

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
		if len(artifacts) == 0 {
			emitError(ctx, op.sink, out, req.SessionID, "wave_complete",
				fmt.Errorf("wave finished with no task output (check worker runners)"))
			return
		}
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

// summarizeArtifacts produces user-facing text from wave artifacts for IM reply.
func summarizeArtifacts(artifacts []wave.Artifact) string {
	if len(artifacts) == 0 {
		return "(no artifacts)"
	}
	parts := make([]string, 0, len(artifacts))
	for _, a := range artifacts {
		if a.Error != "" {
			parts = append(parts, fmt.Sprintf("[%s] failed: %s", a.TaskID, a.Error))
			continue
		}
		content := strings.TrimSpace(a.Summary)
		if content == "" {
			continue
		}
		parts = append(parts, content)
	}
	if len(parts) == 0 {
		return fmt.Sprintf("Wave complete: %d task(s) finished with no text output", len(artifacts))
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return strings.Join(parts, "\n\n---\n\n")
}

func workerEventToEngine(sessionID, taskID string, ev wave.WorkerEvent) *contracts.EngineEvent {
	switch ev.Type {
	case "thinking", "text", "tool_use", "error":
		return &contracts.EngineEvent{
			Type:      ev.Type,
			Content:   ev.Content,
			SessionID: sessionID,
			Metadata:  map[string]string{"wave_task_id": taskID},
		}
	default:
		return nil
	}
}

// emit writes an event to the caller channel, respecting ctx cancellation.
// Events are not duplicated to sink — D1 gateway consumes the returned channel.
func emit(ctx context.Context, sink EventPublisher, out chan<- *contracts.EngineEvent, ev *contracts.EngineEvent) {
	_ = sink
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
// TaskDecomposer and a functional WaveScheduler. It is the v1.1.0+ default
// for NewSessionOrchestrator when no WithOrchestratePath option is supplied.
//
// The default WaveScheduler has a real WorkerPool, ConflictGuard,
// ArtifactStore, and ContextResolver — so the dispatch loop will not
// deadlock or panic. Tasks that reach the dispatch phase will fail with
// "no runner for kind X" errors (clean failure) because no WorkerRunners
// are registered. Production callers that need real multi-agent execution
// MUST wire a WaveScheduler with proper runners via WithOrchestratePath.
func newDefaultOrchestratePath(sink EventPublisher, llmDecomp LLMTaskDecomposer) *OrchestratePath {
	decomp := NewTaskDecomposer()
	if llmDecomp != nil {
		decomp.SetLLMDecomposer(llmDecomp)
	}
	pool := wave.NewWorkerPool(wave.DefaultPoolCapacity)
	guard := wave.NewConflictGuard()
	artifacts := wave.NewArtifactStore()
	resolver := wave.NewContextResolver(wave.ContextResolverDeps{
		Artifacts:        artifacts,
		BaseSystemPrompt: "",
	})
	sched := wave.NewWaveScheduler(wave.SchedulerDeps{
		Pool:      pool,
		Guard:     guard,
		Resolver:  resolver,
		Artifacts: artifacts,
	})
	return NewOrchestratePath(decomp, sched, sink)
}
