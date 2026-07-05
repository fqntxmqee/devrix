package communication

import "github.com/devrix/devrix/internal/layers/observability/diagnose/coverage"

func init() {
	coverage.RegisterProvider(spansProvider{})
}

type spansProvider struct{}

func (spansProvider) Spans() []coverage.OperationMeta {
	return []coverage.OperationMeta{
		// D1 Communication - Gateway / Capture (D1-S13)
		{Name: "D1_Capture_Message_Receive", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D1_Capture_Session_Lifecycle", Layer: "communication", Component: "gateway", SinceVersion: "1.3.0", Instrumented: true},
		{Name: "D1_Capture_Session_Create", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D1_Capture_Session_Get", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D1_Capture_Session_Expire", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D1_Capture_Store_Create", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D1_Capture_Store_Get", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D1_Capture_Store_Update", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D1_Capture_Store_Delete", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D1_Capture_Permission_Check", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D1_Capture_Agent_Create", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D1_Capture_EngineEvent_Handle", Layer: "communication", Component: "gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D1_Capture_Transcript_Append", Layer: "communication", Component: "gateway", SinceVersion: "3.3.0", Instrumented: true},
		{Name: "D1_Capture_Dispatch", Layer: "communication", Component: "gateway", SinceVersion: "3.3.0", Instrumented: true},

		// D1 Communication - Signal (D1-S14~S16)
		{Name: "D1_Capture_Persist", Layer: "communication", Component: "gateway", SinceVersion: "3.1.0", Instrumented: true},
		{Name: "D1_Dispatch_Route", Layer: "communication", Component: "gateway", SinceVersion: "3.1.0", Instrumented: true},
		{Name: "D1_Signal_Thinking", Layer: "communication", Component: "gateway", SinceVersion: "3.1.0", Instrumented: true},
		{Name: "D1_Signal_Task", Layer: "communication", Component: "gateway", SinceVersion: "3.1.0", Instrumented: true},
		{Name: "D1_Signal_Conclusion", Layer: "communication", Component: "gateway", SinceVersion: "3.1.0", Instrumented: true},
		{Name: "D1_Signal_ChainIntegrity", Layer: "communication", Component: "gateway", SinceVersion: "3.1.0", Instrumented: true},
		{Name: "D1_Signal_TaskWorkProof", Layer: "communication", Component: "gateway", SinceVersion: "3.1.0", Instrumented: true},
		{Name: "D1_UserFeedback_ConclusionRejected", Layer: "communication", Component: "gateway", SinceVersion: "3.1.0", Instrumented: true},
		// D1-S16 emit.complete.fallback (P0, DM-20260630-011 devrix-session-conclusion-completeness).
		// Emitted when conclusion.EmitComplete falls back from `summary` to
		// event.Content or stats because the LLM last-turn text was too_short /
		// inconclusive per LastTextQualityGate. Surfaces the silent fallback
		// chain in Jaeger so dashboards can alert on abnormal fallback rate.
		{Name: "D1_EmitComplete_Fallback", Layer: "communication", Component: "gateway", SinceVersion: "3.2.0", Instrumented: true},

		// D1 Communication - Adapter (D1-S17)
		{Name: "D1_Adapter_Message_Receive", Layer: "communication", Component: "adapter", SinceVersion: "1.3.0", Instrumented: true},
		{Name: "D1_Adapter_CLI_Send", Layer: "communication", Component: "adapter", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D1_Adapter_Feishu_Outbound", Layer: "communication", Component: "adapter", SinceVersion: "2.0.0", Instrumented: true},
	}
}
