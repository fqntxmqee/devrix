// OrchestratePath is the D7-S2 orchtypes.IntentOrchestrate explicit-orchestration
// path.
//
//  1. decisionplanning.TaskDecomposer.SynthesizeTaskGraph (D7-S5-A02) — goal → TaskNode DAG
//  2. WaveScheduler.Start (D7-S3-A01) — DAG → 5-slot pool dispatch
//  3. WaveScheduler.WaitForCompletion — block until all nodes terminal
//  4. Stream FlowEvents back to the caller as EngineEvents
//
// Wiring: bootstrap injects decisionplanning.TaskDecomposer + WaveScheduler via
// NewOrchestratePath. ProcessMessage's orchtypes.IntentOrchestrate case calls
// Run directly.
package sessionorchestrator

import (
	"context"
	"fmt"
	"github.com/devrix/devrix/internal/layers/orchestration/decisionplanning"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"strings"

	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/contracts"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// WaveSchedulerRunner abstracts the WaveScheduler methods OrchestratePath
// needs. The concrete *wavescheduler.WaveScheduler satisfies this interface;
// tests can inject a fake.
type WaveSchedulerRunner interface {
	Start(ctx context.Context, sessionID string, graph *wavescheduler.TaskGraph) error
	WaitForCompletion(ctx context.Context, sessionID string) ([]wavescheduler.Artifact, error)
}

// OrchestratePath runs the D7-S5-A02 + D7-S3-A01 pipeline for
// orchtypes.IntentOrchestrate messages.
type OrchestratePath struct {
	decomposer  *decisionplanning.TaskDecomposer
	scheduler   WaveSchedulerRunner
	sink        EventPublisher
	obsBridge   *observability.Bridge
	taskManager *workmodel.TaskManager
}

// SetTaskManager wires WorkTree sync for wave nodes.
func (op *OrchestratePath) SetTaskManager(tm *workmodel.TaskManager) {
	if op != nil {
		op.taskManager = tm
	}
}

// SetObsBridge wires tracing for the orchestrate pipeline.
func (op *OrchestratePath) SetObsBridge(bridge *observability.Bridge) {
	if op == nil {
		return
	}
	op.obsBridge = bridge
	if ws, ok := op.scheduler.(*wavescheduler.WaveScheduler); ok {
		ws.SetObsBridge(bridge)
	}
}

// NewOrchestratePath builds the path. All args are required; nil →
// Run returns an error at the call site (no FastPath fallback).
func NewOrchestratePath(
	decomposer *decisionplanning.TaskDecomposer,
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
// Only decisionplanning.TaskDecomposer.SynthesizeTaskGraph may invoke an LLM (LLM-augmented
// decomposition with 5s timeout; rule-based fallback otherwise).
func (op *OrchestratePath) Run(ctx context.Context, req orchtypes.ProcessRequest, _ orchtypes.IntentClassification) (<-chan *contracts.EngineEvent, error) {
	if op == nil {
		return nil, fmt.Errorf("orchestrator: OrchestratePath is nil (bootstrap missing wiring)")
	}
	if op.decomposer == nil {
		return nil, fmt.Errorf("orchestrator: OrchestratePath.decomposer is nil (bootstrap missing decisionplanning.TaskDecomposer)")
	}
	if op.scheduler == nil {
		return nil, fmt.Errorf("orchestrator: OrchestratePath.scheduler is nil (bootstrap missing WaveScheduler)")
	}

	out := make(chan *contracts.EngineEvent, 16)
	go func() {
		defer close(out)

		ctx, orchSpan := startObsSpan(op.obsBridge, ctx, telemetry.OpD7_S2_Orchestration_Orchestrate_Run, tracer.SpanKindInternal,
			tracer.Attribute{Key: "session_id", Value: req.SessionID},
			tracer.Attribute{Key: "context.caller", Value: "d7"},
		)
		defer endSpan(orchSpan)

		// v6.0.0 5 节点 MUPS Pipeline root span (D7-S6-A50). Started AFTER Orchestrate_Run
		// so it shows up as a child of the orchestrate path root. The 4 sync 5-node spans
		// (taskgraph.synthesize / executor.select / channel.route / system.anomaly_detect)
		// inherit it as parent via ctx propagation. The async 5th node (memory.persist in
		// processAutoClose) is associated by sessionID rather than trace tree.
		//
		// 5 nodes mapped to spans:
		//   Observe → orchestrator.buildObserveRequest writes prior attrs to sessionSpan
		//             (no independent Span; shares Session_Process trace context)
		//   Plan    → decisionplanning/decomposer.go    : EmitTaskGraphSynthesize
		//   Wave    → wavescheduler/scheduler.go         : EmitExecutorSelect
		//   Execute → mups/execute/channel.go            : EmitChannelRoute
		//   Verify  → executionflow/verify/anomaly.go    : EmitSystemAnomalyDetect
		//   Learn   → mups/learn/memory.go               : EmitMemoryPersist (async)
		ctx, mupsSpan := startObsSpan(op.obsBridge, ctx, telemetry.OpD7_S6_MUPS_Pipeline, tracer.SpanKindInternal,
			tracer.Attribute{Key: "session_id", Value: req.SessionID},
			tracer.Attribute{Key: "pipeline.intent", Value: string(orchtypes.IntentOrchestrate)},
			tracer.Attribute{Key: "pipeline.nodes", Value: "observe,plan,wave,execute,verify,learn"},
		)
		defer endSpan(mupsSpan)

		// 1) SynthesizeTaskGraph (D7-S5-A02) — Plan node
		result, err := op.decomposer.SynthesizeTaskGraph(ctx, req.SessionID, req.Message)
		if err != nil {
			emitError(ctx, op.sink, out, req.SessionID, "synthesize_task_graph", err)
			return
		}
		if result.Validation != nil && !result.Validation.Valid {
			emitError(ctx, op.sink, out, req.SessionID, "validate_task_graph",
				fmt.Errorf("graph invalid: %d error(s): %s",
					len(result.Validation.Errors),
					strings.Join(result.Validation.Errors, "; ")))
			return
		}
		if orchSpan != nil {
			orchSpan.SetAttributes(
				tracer.Attribute{Key: "task.node_count", Value: fmt.Sprintf("%d", len(result.Nodes))},
				tracer.Attribute{Key: "task.validation_valid", Value: boolStr(result.Validation.Valid)},
				tracer.Attribute{Key: "task.validation_errors", Value: fmt.Sprintf("%d", len(result.Validation.Errors))},
			)
		}
		if op.taskManager != nil {
			batchRootID, err := op.taskManager.SyncWaveNodes(req.SessionID, result.Nodes)
			if err != nil {
				emitError(ctx, op.sink, out, req.SessionID, "sandbox_sync", err)
				return
			}
			if batchRootID != "" {
				result.Nodes = op.taskManager.WaveNodesFromSubtree(req.SessionID, batchRootID)
			}
		}
		emit(ctx, op.sink, out, &contracts.EngineEvent{
			Type:      "plan_formed",
			Content:   fmt.Sprintf("%d tasks synthesized", len(result.Nodes)),
			SessionID: req.SessionID,
		})

		// 2) Build TaskGraph (D7-S3)
		graph := buildTaskGraph(result.Nodes)

		if ws, ok := op.scheduler.(*wavescheduler.WaveScheduler); ok {
			ws.SetWorkerEventHandler(func(sessionID, taskID string, ev wavescheduler.WorkerEvent) {
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

		// 5) Emit terminal events. The per-task text/thinking/tool_use
		// streams were already fanned out by workerEventToEngine during
		// step 4 (DM-20260626-002) — DO NOT re-emit a consolidated
		// summary here, that produced duplicate content in the feishu
		// card (contentLen=61207 followed by 61212 = worker text +
		// outputParts-joined "result\ndone" + summarizeArtifacts).
		if len(artifacts) == 0 {
			emitError(ctx, op.sink, out, req.SessionID, "wave_complete",
				fmt.Errorf("wave finished with no task output (check worker runners)"))
			return
		}
		_ = summarizeArtifacts // retained for cross-package callers / tests
		emit(ctx, op.sink, out, &contracts.EngineEvent{
			Type:      "complete",
			SessionID: req.SessionID,
		})
	}()
	return out, nil
}

// buildTaskGraph constructs a *wavescheduler.TaskGraph from decomposed TaskNodes.
// It performs the light validation required by WaveScheduler.Start (the
// full DAG validation lives in decisionplanning.TaskDecomposer.validateGraph).
func buildTaskGraph(nodes []wavescheduler.TaskNode) *wavescheduler.TaskGraph {
	// The wave package exposes TaskGraph as a public type constructed by
	// the caller; the only exported constructor is NewTaskGraph(slice).
	return wavescheduler.NewTaskGraph(nodes)
}

// summarizeArtifacts produces user-facing text from wave artifacts for IM reply.
func summarizeArtifacts(artifacts []wavescheduler.Artifact) string {
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

func workerEventToEngine(sessionID, taskID string, ev wavescheduler.WorkerEvent) *contracts.EngineEvent {
	switch ev.Type {
	case "thinking", "tool_use", "text", "error":
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
		Content:   fmt.Sprintf("%s: %s", label, sharederrors.SanitizeForUser(err)),
		SessionID: sessionID,
	})
}

// newDefaultOrchestratePath builds an OrchestratePath bound to a fresh
// decisionplanning.TaskDecomposer and a functional WaveScheduler. It is the v1.1.0+ default
// for NewSessionOrchestrator when no WithOrchestratePath option is supplied.
//
// The default WaveScheduler has a real WorkerPool, ConflictGuard,
// ArtifactStore, and ContextResolver — so the dispatch loop will not
// deadlock or panic. Tasks that reach the dispatch phase will fail with
// "no runner for kind X" errors (clean failure) because no WorkerRunners
// are registered. Production callers that need real multi-agent execution
// MUST wire a WaveScheduler with proper runners via WithOrchestratePath.
func newDefaultOrchestratePath(sink EventPublisher, llmDecomp decisionplanning.LLMTaskDecomposer) *OrchestratePath {
	decomp := decisionplanning.NewTaskDecomposer()
	if llmDecomp != nil {
		decomp.SetLLMDecomposer(llmDecomp)
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
	})
	return NewOrchestratePath(decomp, sched, sink)
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
