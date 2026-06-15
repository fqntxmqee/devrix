package communication

import "github.com/devrix/devrix/internal/layers/observability/diagnose/coverage"

func init() {
	coverage.RegisterProvider(spansProvider{})
}

type spansProvider struct{}

func (spansProvider) Spans() []coverage.OperationMeta {
	return []coverage.OperationMeta{
		// D1 Communication - Gateway / Capture (D1-S13)
		{Name: "D1_S13_Capture_Message_Receive", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D1_S13_Capture_Session_Lifecycle", Layer: "communication", Component: "gateway", SinceVersion: "1.3.0", Instrumented: true},
		{Name: "D1_S13_Capture_Session_Create", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D1_S13_Capture_Session_Get", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D1_S13_Capture_Session_Expire", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D1_S13_Capture_Store_Create", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D1_S13_Capture_Store_Get", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D1_S13_Capture_Store_Update", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D1_S13_Capture_Store_Delete", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D1_S13_Capture_Permission_Check", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D1_S13_Capture_Agent_Create", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D1_S13_Capture_EngineEvent_Handle", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},

		// D1 Communication - Signal (D1-S14~S16)
		{Name: "D1_S13_Capture_Persist", Layer: "communication", Component: "gateway", SinceVersion: "3.1.0", Instrumented: true},
		{Name: "D1_S13_Dispatch_Route", Layer: "communication", Component: "gateway", SinceVersion: "3.1.0", Instrumented: true},
		{Name: "D1_S14_Signal_Thinking", Layer: "communication", Component: "gateway", SinceVersion: "3.1.0", Instrumented: true},
		{Name: "D1_S15_Signal_Task", Layer: "communication", Component: "gateway", SinceVersion: "3.1.0", Instrumented: true},
		{Name: "D1_S16_Signal_Conclusion", Layer: "communication", Component: "gateway", SinceVersion: "3.1.0", Instrumented: true},
		{Name: "D1_S14_Signal_ChainIntegrity", Layer: "communication", Component: "gateway", SinceVersion: "3.1.0", Instrumented: true},
		{Name: "D1_S15_Signal_TaskWorkProof", Layer: "communication", Component: "gateway", SinceVersion: "3.1.0", Instrumented: true},
		{Name: "D1_S16_UserFeedback_ConclusionRejected", Layer: "communication", Component: "gateway", SinceVersion: "3.1.0", Instrumented: true},

		// D1 Communication - Adapter (D1-S17)
		{Name: "D1_S17_Adapter_Message_Receive", Layer: "communication", Component: "adapter", SinceVersion: "1.3.0", Instrumented: true},
		{Name: "D1_S17_Adapter_CLI_Send", Layer: "communication", Component: "adapter", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D1_S17_Adapter_Feishu_Outbound", Layer: "communication", Component: "adapter", SinceVersion: "2.0.0", Instrumented: true},
	}
}
