package coverage_test

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/observability/diagnose/coverage"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"

	// Import domain spans packages to trigger self-registration via init()
	_ "github.com/devrix/devrix/internal/layers/communication"
	_ "github.com/devrix/devrix/internal/layers/contextengine"
	_ "github.com/devrix/devrix/internal/layers/evolution"
	_ "github.com/devrix/devrix/internal/layers/llmgateway"
	_ "github.com/devrix/devrix/internal/layers/multiagent"
	// D7 orchestration spans were previously loaded via
	// `_ ".../orchestration/coordinator"` which transitively pulled in
	// sessionorchestrator + decisionplanning + workmodel + orchtypes init().
	// Now load sessionorchestrator directly (the canonical owner).
	_ "github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	_ "github.com/devrix/devrix/internal/layers/orchestration/decisionplanning"
	_ "github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	_ "github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
)

func TestAllOperations_should_match_telemetry_constants(t *testing.T) {
	t.Helper()
	expected := []string{
		// D1 Communication - Gateway / Capture (D1-S13)
		telemetry.OpD1_S13_Capture_Message_Receive,
		telemetry.OpD1_S13_Capture_Session_Lifecycle,
		telemetry.OpD1_S13_Capture_Session_Create,
		telemetry.OpD1_S13_Capture_Session_Get,
		telemetry.OpD1_S13_Capture_Session_Expire,
		telemetry.OpD1_S13_Capture_Store_Create,
		telemetry.OpD1_S13_Capture_Store_Get,
		telemetry.OpD1_S13_Capture_Store_Update,
		telemetry.OpD1_S13_Capture_Store_Delete,
		telemetry.OpD1_S13_Capture_Permission_Check,
		telemetry.OpD1_S13_Capture_Agent_Create,
		telemetry.OpD1_S13_Capture_EngineEvent_Handle,

		// D1 Communication - Signal (D1-S14~S16)
		telemetry.OpD1_S13_Capture_Persist,
		telemetry.OpD1_S13_Dispatch_Route,
		telemetry.OpD1_S14_Signal_Thinking,
		telemetry.OpD1_S15_Signal_Task,
		telemetry.OpD1_S16_Signal_Conclusion,
		telemetry.OpD1_S14_Signal_ChainIntegrity,
		telemetry.OpD1_S15_Signal_TaskWorkProof,
		telemetry.OpD1_S16_UserFeedback_ConclusionRejected,

		// D1 Communication - Adapter (D1-S17)
		telemetry.OpD1_S17_Adapter_Message_Receive,
		telemetry.OpD1_S17_Adapter_CLI_Send,
		telemetry.OpD1_S17_Adapter_Feishu_Outbound,

		// D2 Context Engine - Core (D2-S2)
		telemetry.OpD2_S2_Context_Process,
		telemetry.OpD2_S2_Context_Snapshot_Load,
		telemetry.OpD2_S2_Context_SystemPrompt_Load,
		telemetry.OpD2_S2_Context_Compression_Run,
		telemetry.OpD2_S2_Context_Compression_Step,
		telemetry.OpD2_S2_Context_Longterm_Recall,
		telemetry.OpD2_S2_Context_Longterm_Store,
		telemetry.OpD2_S2_Context_Tools_Register,
		telemetry.OpD2_S2_Context_Tools_List,
		telemetry.OpD2_S2_Context_Tools_Filter_Permission,
		telemetry.OpD2_S2_Context_Tools_Filter_AgentRole,
		telemetry.OpD2_S2_Context_Worker_Fork,
		telemetry.OpD2_S2_Context_Permission_Init,
		telemetry.OpD2_S2_Context_Tier_Resolve,
		telemetry.OpD2_S2_Context_Memory_Append,
		telemetry.OpD2_S2_Context_CompressedView_Set,
		telemetry.OpD2_S2_Context_Attachments_Collect,
		telemetry.OpD2_S2_Context_Queue_Drain,
		telemetry.OpD2_S2_Context_Memory_Snapshot_Save,

		// D2 Context Engine - Tool Execution (D2-S5)
		telemetry.OpD2_S5_Tool_Execute_Single,
		telemetry.OpD2_S5_Tool_Execute_Permission,

		// D2 Context Engine - Task / Plan (D2-S8)
		telemetry.OpD2_S8_Task_Plan_Generate,
		telemetry.OpD2_S8_Task_PlanMode_Enter,
		telemetry.OpD2_S8_Task_PlanMode_Execute,
		telemetry.OpD2_S8_Task_PlanMode_Approve,
		telemetry.OpD2_S8_Task_PlanMode_Reject,
		telemetry.OpD2_S8_Task_Manager_Create,
		telemetry.OpD2_S8_Task_Manager_Update,

		// D3 LLM Gateway (D3-S3)
		telemetry.OpD3_S3_LLM_Stream,
		telemetry.OpD3_S3_LLM_Provider_Route,
		telemetry.OpD3_S3_LLM_CircuitBreaker,
		telemetry.OpD3_S3_LLM_Retry,
		telemetry.OpD3_S3_LLM_Adapter_Stream,

		// D4 MultiAgent (D4-S4)
		telemetry.OpD4_S4_Agent_Run,
		telemetry.OpD4_S4_Agent_Tool_Call,
		telemetry.OpD4_S4_Agent_Fork,
		telemetry.OpD4_S4_Agent_Join,
		telemetry.OpD4_S4_Agent_Terminate,
		telemetry.OpD4_S4_Agent_State_Transition,

		// D7 Orchestration - Session / Turn (D7-S2)
		telemetry.OpD7_S2_Orchestration_Session_Process,
		telemetry.OpD7_S2_Orchestration_Intent_Classify,
		telemetry.OpD7_S2_Orchestration_Turn_Run,
		telemetry.OpD7_S2_Orchestration_Turn_Iteration,
		telemetry.OpD7_S2_Orchestration_LLM_Invoke,
		telemetry.OpD7_S2_Orchestration_Orchestrate_Run,

		// D7 Orchestration (D7-S3)
		telemetry.OpD7_S3_Orchestration_Wave_Schedule,
		telemetry.OpD7_S3_Orchestration_Wave_Task_Execute,
		telemetry.OpD7_S3_Orchestration_Flow_Event_Publish,

		// D7 Task Manager (D7-S1)
		telemetry.OpD7_S1_Task_Manager_Create,
		telemetry.OpD7_S1_Task_Manager_Update,

		// D6 Evolution - Runtime Validation (D6-S4)
		telemetry.OpD6_S4_Validation_Decision,
	}

	registry := coverage.AllOperations()
	if len(registry) != len(expected) {
		t.Fatalf("registry size %d, want %d", len(registry), len(expected))
	}

	byName := make(map[string]coverage.OperationMeta, len(registry))
	for _, op := range registry {
		byName[op.Name] = op
	}
	for _, name := range expected {
		meta, ok := byName[name]
		if !ok {
			t.Fatalf("missing operation %q in registry", name)
		}
		layer, component := telemetry.LayerAndComponent(name)
		if meta.Layer != layer || meta.Component != component {
			t.Fatalf("operation %q metadata layer=%q component=%q, want layer=%q component=%q",
				name, meta.Layer, meta.Component, layer, component)
		}
		if !meta.Instrumented {
			t.Fatalf("operation %q must be instrumented", name)
		}
	}
}
