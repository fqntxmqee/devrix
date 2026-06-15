package coordinator

import "github.com/devrix/devrix/internal/layers/observability/diagnose/coverage"

func init() {
	coverage.RegisterProvider(spansProvider{})
}

type spansProvider struct{}

func (spansProvider) Spans() []coverage.OperationMeta {
	return []coverage.OperationMeta{
		// D7 Orchestration (D7-S3)
		{Name: "D7_S3_Orchestration_Wave_Schedule", Layer: "orchestration", Component: "orchestrator", SinceVersion: "2.1.0", Instrumented: true},
		{Name: "D7_S3_Orchestration_Wave_Task_Execute", Layer: "orchestration", Component: "orchestrator", SinceVersion: "2.1.0", Instrumented: true},
		{Name: "D7_S3_Orchestration_Flow_Event_Publish", Layer: "orchestration", Component: "orchestrator", SinceVersion: "2.1.0", Instrumented: true},

		// D7 Task Manager (D7-S1)
		{Name: "D7_S1_Task_Manager_Create", Layer: "orchestration", Component: "task_manager", SinceVersion: "2.1.0", Instrumented: true},
		{Name: "D7_S1_Task_Manager_Update", Layer: "orchestration", Component: "task_manager", SinceVersion: "2.1.0", Instrumented: true},
	}
}
