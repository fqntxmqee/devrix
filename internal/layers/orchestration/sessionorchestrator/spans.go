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
	}
}
