package contextengine

import "github.com/devrix/devrix/internal/layers/observability/diagnose/coverage"

func init() {
	coverage.RegisterProvider(spansProvider{})
}

type spansProvider struct{}

func (spansProvider) Spans() []coverage.OperationMeta {
	return []coverage.OperationMeta{
		// D2 Context Engine - Core (D2-S2)
		{Name: "D2_S2_Context_Process", Layer: "context", Component: "context_engine", SinceVersion: "1.2.0", Instrumented: true},
		{Name: "D2_S2_Context_Snapshot_Load", Layer: "context", Component: "context_engine", SinceVersion: "1.2.0", Instrumented: true},
		{Name: "D2_S2_Context_SystemPrompt_Load", Layer: "context", Component: "context_engine", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D2_S2_Context_Compression_Run", Layer: "context", Component: "context_engine", SinceVersion: "1.2.0", Instrumented: true},
		{Name: "D2_S2_Context_Compression_Step", Layer: "context", Component: "context_engine", SinceVersion: "2.1.0", Instrumented: true},
		{Name: "D2_S2_Context_Longterm_Recall", Layer: "context", Component: "context_engine", SinceVersion: "1.3.0", Instrumented: true},
		{Name: "D2_S2_Context_Longterm_Store", Layer: "context", Component: "context_engine", SinceVersion: "1.3.0", Instrumented: true},
		{Name: "D2_S2_Context_Tools_Register", Layer: "context", Component: "context_engine", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D2_S2_Context_Memory_Snapshot_Save", Layer: "context", Component: "context_engine", SinceVersion: "2.0.0", Instrumented: true},

		// D2 Context Engine - Harness (D2-S5)
		{Name: "D2_S5_Context_Harness_Bootstrap_Run", Layer: "context", Component: "harness", SinceVersion: "5.0.0", Instrumented: true},
		{Name: "D2_S5_Context_Harness_Bootstrap_Stage", Layer: "context", Component: "harness", SinceVersion: "5.0.0", Instrumented: true},
		{Name: "D2_S5_Context_Harness_Preflight", Layer: "context", Component: "harness", SinceVersion: "5.0.0", Instrumented: true},
		{Name: "D2_S5_Context_Harness_Route", Layer: "context", Component: "harness", SinceVersion: "5.0.0", Instrumented: true},
		{Name: "D2_S5_Context_Harness_ToolPool", Layer: "context", Component: "harness", SinceVersion: "5.0.0", Instrumented: true},
		{Name: "D2_S5_Context_Harness_SystemPrompt_Build", Layer: "context", Component: "harness", SinceVersion: "5.0.0", Instrumented: true},

		// D2 Context Engine - QueryLoop (D2-S10)
		{Name: "D2_S10_Query_Loop_Run", Layer: "context", Component: "query_loop", SinceVersion: "2.1.0", Instrumented: true},
		{Name: "D2_S10_Query_Loop_Turn", Layer: "context", Component: "query_loop", SinceVersion: "2.1.0", Instrumented: true},
		{Name: "D2_S10_Query_Loop_LLM_Call", Layer: "context", Component: "query_loop", SinceVersion: "2.1.0", Instrumented: true},

		// D2 Context Engine - Tool Execution (D2-S5)
		{Name: "D2_S5_Tool_Execute_Single", Layer: "context", Component: "tool_runner", SinceVersion: "2.1.0", Instrumented: true},
		{Name: "D2_S5_Tool_Execute_Permission", Layer: "context", Component: "tool_runner", SinceVersion: "2.1.0", Instrumented: true},

		// D2 Context Engine - Task / Plan (D2-S8)
		{Name: "D2_S8_Task_Plan_Generate", Layer: "context", Component: "plan_agent", SinceVersion: "2.1.0", Instrumented: true},
		{Name: "D2_S8_Task_PlanMode_Enter", Layer: "context", Component: "plan_mode", SinceVersion: "2.1.0", Instrumented: true},
		{Name: "D2_S8_Task_PlanMode_Execute", Layer: "context", Component: "plan_mode", SinceVersion: "2.1.0", Instrumented: true},
		{Name: "D2_S8_Task_PlanMode_Approve", Layer: "context", Component: "plan_mode", SinceVersion: "2.1.0", Instrumented: true},
		{Name: "D2_S8_Task_PlanMode_Reject", Layer: "context", Component: "plan_mode", SinceVersion: "2.1.0", Instrumented: true},
		{Name: "D2_S8_Task_Manager_Create", Layer: "context", Component: "task_manager", SinceVersion: "2.1.0", Instrumented: true},
		{Name: "D2_S8_Task_Manager_Update", Layer: "context", Component: "task_manager", SinceVersion: "2.1.0", Instrumented: true},
	}
}
