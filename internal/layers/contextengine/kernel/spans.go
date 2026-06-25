package kernel

import "github.com/devrix/devrix/internal/layers/observability/diagnose/coverage"

func init() {
	coverage.RegisterProvider(spansProvider{})
}

type spansProvider struct{}

func (spansProvider) Spans() []coverage.OperationMeta {
	return []coverage.OperationMeta{
		// D2 Context Engine - Core (D2-S2)
		{Name: "D2_Context_Process", Layer: "context", Component: "context_engine", SinceVersion: "1.2.0", Instrumented: true},
		{Name: "D2_Context_Snapshot_Load", Layer: "context", Component: "context_engine", SinceVersion: "1.2.0", Instrumented: true},
		{Name: "D2_Context_SystemPrompt_Load", Layer: "context", Component: "context_engine", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D2_Context_Compression_Run", Layer: "context", Component: "context_engine", SinceVersion: "1.2.0", Instrumented: true},
		{Name: "D2_Context_Compression_Step", Layer: "context", Component: "context_engine", SinceVersion: "2.1.0", Instrumented: true},
		{Name: "D2_Context_Longterm_Recall", Layer: "context", Component: "context_engine", SinceVersion: "1.3.0", Instrumented: true},
		{Name: "D2_Context_Longterm_Store", Layer: "context", Component: "context_engine", SinceVersion: "1.3.0", Instrumented: true},
		{Name: "D2_Context_Tools_Register", Layer: "context", Component: "context_engine", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D2_Context_Tools_List", Layer: "context", Component: "context_engine", SinceVersion: "2.2.0", Instrumented: true},
		{Name: "D2_Context_Tools_Filter_Permission", Layer: "context", Component: "context_engine", SinceVersion: "2.2.0", Instrumented: true},
		{Name: "D2_Context_Tools_Filter_AgentRole", Layer: "context", Component: "context_engine", SinceVersion: "2.2.0", Instrumented: true},
		{Name: "D2_Context_Worker_Fork", Layer: "context", Component: "context_engine", SinceVersion: "2.2.0", Instrumented: true},
		{Name: "D2_Context_Permission_Init", Layer: "context", Component: "context_engine", SinceVersion: "2.2.0", Instrumented: true},
		{Name: "D2_Context_Tier_Resolve", Layer: "context", Component: "context_engine", SinceVersion: "2.2.0", Instrumented: true},
		{Name: "D2_Context_Memory_Append", Layer: "context", Component: "context_engine", SinceVersion: "2.2.0", Instrumented: true},
		{Name: "D2_Context_CompressedView_Set", Layer: "context", Component: "context_engine", SinceVersion: "2.2.0", Instrumented: true},
		{Name: "D2_Context_Attachments_Collect", Layer: "context", Component: "context_engine", SinceVersion: "2.2.0", Instrumented: true},
		{Name: "D2_Context_Queue_Drain", Layer: "context", Component: "context_engine", SinceVersion: "2.2.0", Instrumented: true},
		{Name: "D2_Context_Memory_Snapshot_Save", Layer: "context", Component: "context_engine", SinceVersion: "2.0.0", Instrumented: true},

		// D2 Context Engine - Tool Execution (D2-S5)
		{Name: "D2_Tool_Execute_Single", Layer: "context", Component: "tool_runner", SinceVersion: "2.1.0", Instrumented: true},
		{Name: "D2_Tool_Execute_Permission", Layer: "context", Component: "tool_runner", SinceVersion: "2.1.0", Instrumented: true},
		{Name: "D2_Context_Harness_SystemPrompt_Build", Layer: "context", Component: "harness", SinceVersion: "2.2.0", Instrumented: true},

		// D2 Context Engine - Task / Plan (D2-S8)
		{Name: "D2_Task_Plan_Generate", Layer: "context", Component: "plan_agent", SinceVersion: "2.1.0", Instrumented: true},
		{Name: "D2_Task_PlanMode_Enter", Layer: "context", Component: "plan_mode", SinceVersion: "2.1.0", Instrumented: true},
		{Name: "D2_Task_PlanMode_Execute", Layer: "context", Component: "plan_mode", SinceVersion: "2.1.0", Instrumented: true},
		{Name: "D2_Task_PlanMode_Approve", Layer: "context", Component: "plan_mode", SinceVersion: "2.1.0", Instrumented: true},
		{Name: "D2_Task_PlanMode_Reject", Layer: "context", Component: "plan_mode", SinceVersion: "2.1.0", Instrumented: true},
		{Name: "D2_Task_Manager_Create", Layer: "context", Component: "task_manager", SinceVersion: "2.1.0", Instrumented: true},
		{Name: "D2_Task_Manager_Update", Layer: "context", Component: "task_manager", SinceVersion: "2.1.0", Instrumented: true},
	}
}
