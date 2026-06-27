package sessionorchestrator

import "github.com/devrix/devrix/internal/layers/observability/diagnose/coverage"

func init() {
	coverage.RegisterProvider(spansProvider{})
}

type spansProvider struct{}

func (spansProvider) Spans() []coverage.OperationMeta {
	return []coverage.OperationMeta{
		// D7 Orchestration - Session / Turn (D7-S2)
		{Name: "D7_Orchestration_Session_Process", Layer: "orchestration", Component: "orchestrator", SinceVersion: "2.2.0", Instrumented: true},
		{Name: "D7_Orchestration_Intent_Classify", Layer: "orchestration", Component: "orchestrator", SinceVersion: "2.2.0", Instrumented: true},
		{Name: "D7_Orchestration_Turn_Run", Layer: "orchestration", Component: "orchestrator", SinceVersion: "2.2.0", Instrumented: true},
		{Name: "D7_Orchestration_Turn_Iteration", Layer: "orchestration", Component: "orchestrator", SinceVersion: "2.2.0", Instrumented: true},
		{Name: "D7_Orchestration_LLM_Invoke", Layer: "orchestration", Component: "orchestrator", SinceVersion: "2.2.0", Instrumented: true},
		{Name: "D7_Orchestration_Orchestrate_Run", Layer: "orchestration", Component: "orchestrator", SinceVersion: "2.2.0", Instrumented: true},

		// D7 Orchestration (D7-S3)
		{Name: "D7_Orchestration_Wave_Schedule", Layer: "orchestration", Component: "orchestrator", SinceVersion: "2.1.0", Instrumented: true},
		{Name: "D7_Orchestration_Wave_Task_Execute", Layer: "orchestration", Component: "orchestrator", SinceVersion: "2.1.0", Instrumented: true},
		{Name: "D7_Orchestration_Flow_Event_Publish", Layer: "orchestration", Component: "orchestrator", SinceVersion: "2.1.0", Instrumented: true},

		// D7 Task Manager (D7-S1)
		{Name: "D7_Task_Manager_Create", Layer: "orchestration", Component: "task_manager", SinceVersion: "2.1.0", Instrumented: true},
		{Name: "D7_Task_Manager_Update", Layer: "orchestration", Component: "task_manager", SinceVersion: "2.1.0", Instrumented: true},

		// v6.0.0 6 S 精简 5 节点 P0/P1 Span ops (2026-06-26, see hardening/emitter.go).
		// Layer/component match telemetry.LayerAndComponent (orchestrator/).
		// 5 节点分别是: Plan (taskgraph.synthesize S5) → Wave (executor.select S3) →
		// Execute (channel.route S6) → Verify (system.anomaly_detect S4) → Learn (memory.persist S6).
		// Observe 节点作为 sessionSpan 的 prior attributes 写入（见 orchestrator.go:330-332），
		// 没有独立 Span operation，因为它与 Session_Process 共享 trace context。
		{Name: "D7_Executor_Select", Layer: "orchestration", Component: "orchestrator", SinceVersion: "2.2.0", Instrumented: true},
		{Name: "D7_System_Anomaly_Detect", Layer: "orchestration", Component: "orchestrator", SinceVersion: "2.2.0", Instrumented: true},
		{Name: "D7_TaskGraph_Synthesize", Layer: "orchestration", Component: "orchestrator", SinceVersion: "2.2.0", Instrumented: true},
		{Name: "D7_Channel_Route", Layer: "orchestration", Component: "orchestrator", SinceVersion: "2.2.0", Instrumented: true},
		{Name: "D7_Memory_Persist", Layer: "orchestration", Component: "orchestrator", SinceVersion: "2.2.0", Instrumented: true},

		// v6.0.0 5 节点 pipeline root span (D7-S6). Parent of the 4 sync 5-node spans.
		// Started in OrchestratePath.Run; async Learn node (memory.persist) is associated
		// by sessionID rather than trace tree.
		{Name: "D7_MUPS_Pipeline", Layer: "orchestration", Component: "orchestrator", SinceVersion: "2.2.0", Instrumented: true},

		// DM-20260626-009 follow-up inner observability spans (2026-06-26).
		// The 5-node MUPS spans above cover the top-level pipeline; these
		// three cover the inner layers that were invisible in Jaeger: the
		// WorkItem task tree mutations, parallel-explore / child WorkItem
		// runs, and the per-WorkItem ReAct loop iterations. Without these,
		// debugging a slow WorkItem meant reading the code instead of
		// inspecting traces. Names mirror telemetry/names.go (OpD7_S1_* /
		// OpD7_S5_*) so coverage.IsKnown returns true and tracer.Start no
		// longer WARNs "unknown operation".
		{Name: "D7_Worktree_Op", Layer: "orchestration", Component: "worktree", SinceVersion: "2.2.0", Instrumented: true},
		{Name: "D7_SubWorktree_Run", Layer: "orchestration", Component: "worktree", SinceVersion: "2.2.0", Instrumented: true},
		{Name: "D7_SubTurn_Iteration", Layer: "orchestration", Component: "executor", SinceVersion: "2.2.0", Instrumented: true},
	}
}
