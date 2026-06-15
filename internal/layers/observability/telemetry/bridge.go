// Package telemetry is a backward-compatibility bridge.
//
// Deprecated: use github.com/devrix/devrix/internal/layers/observability/instrument/telemetry instead.
// This bridge will be removed in v2.1.
package telemetry

import "github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"

// Constants — layer identifiers

const (
	LayerCommunication = telemetry.LayerCommunication
	LayerContext       = telemetry.LayerContext
	LayerLLM           = telemetry.LayerLLM
	LayerAgent         = telemetry.LayerAgent
	LayerOrchestration = telemetry.LayerOrchestration
	LayerEvolution     = telemetry.LayerEvolution
)

// Constants — D1 Communication - Gateway / Capture (D1-S13)

const (
	OpD1_S13_Capture_Message_Receive    = telemetry.OpD1_S13_Capture_Message_Receive
	OpD1_S13_Capture_Session_Lifecycle  = telemetry.OpD1_S13_Capture_Session_Lifecycle
	OpD1_S13_Capture_Session_Create     = telemetry.OpD1_S13_Capture_Session_Create
	OpD1_S13_Capture_Session_Get        = telemetry.OpD1_S13_Capture_Session_Get
	OpD1_S13_Capture_Session_Expire     = telemetry.OpD1_S13_Capture_Session_Expire
	OpD1_S13_Capture_Store_Create       = telemetry.OpD1_S13_Capture_Store_Create
	OpD1_S13_Capture_Store_Get          = telemetry.OpD1_S13_Capture_Store_Get
	OpD1_S13_Capture_Store_Update       = telemetry.OpD1_S13_Capture_Store_Update
	OpD1_S13_Capture_Store_Delete       = telemetry.OpD1_S13_Capture_Store_Delete
	OpD1_S13_Capture_Permission_Check   = telemetry.OpD1_S13_Capture_Permission_Check
	OpD1_S13_Capture_Agent_Create       = telemetry.OpD1_S13_Capture_Agent_Create
	OpD1_S13_Capture_EngineEvent_Handle = telemetry.OpD1_S13_Capture_EngineEvent_Handle
)

// Constants — D1 Communication - Signal (D1-S14~S16)

const (
	OpD1_S13_Capture_Persist              = telemetry.OpD1_S13_Capture_Persist
	OpD1_S13_Dispatch_Route              = telemetry.OpD1_S13_Dispatch_Route
	OpD1_S14_Signal_Thinking             = telemetry.OpD1_S14_Signal_Thinking
	OpD1_S15_Signal_Task                 = telemetry.OpD1_S15_Signal_Task
	OpD1_S16_Signal_Conclusion           = telemetry.OpD1_S16_Signal_Conclusion
	OpD1_S14_Signal_ChainIntegrity       = telemetry.OpD1_S14_Signal_ChainIntegrity
	OpD1_S15_Signal_TaskWorkProof        = telemetry.OpD1_S15_Signal_TaskWorkProof
	OpD1_S16_UserFeedback_ConclusionRejected = telemetry.OpD1_S16_UserFeedback_ConclusionRejected
)

// Constants — D1 Communication - Adapter (D1-S17)

const (
	OpD1_S17_Adapter_Message_Receive  = telemetry.OpD1_S17_Adapter_Message_Receive
	OpD1_S17_Adapter_CLI_Send         = telemetry.OpD1_S17_Adapter_CLI_Send
	OpD1_S17_Adapter_Feishu_Outbound  = telemetry.OpD1_S17_Adapter_Feishu_Outbound
)

// Constants — D2 Context Engine - Core (D2-S2)

const (
	OpD2_S2_Context_Process             = telemetry.OpD2_S2_Context_Process
	OpD2_S2_Context_Snapshot_Load       = telemetry.OpD2_S2_Context_Snapshot_Load
	OpD2_S2_Context_SystemPrompt_Load   = telemetry.OpD2_S2_Context_SystemPrompt_Load
	OpD2_S2_Context_Compression_Run     = telemetry.OpD2_S2_Context_Compression_Run
	OpD2_S2_Context_Compression_Step    = telemetry.OpD2_S2_Context_Compression_Step
	OpD2_S2_Context_Longterm_Recall     = telemetry.OpD2_S2_Context_Longterm_Recall
	OpD2_S2_Context_Longterm_Store      = telemetry.OpD2_S2_Context_Longterm_Store
	OpD2_S2_Context_Tools_Register      = telemetry.OpD2_S2_Context_Tools_Register
	OpD2_S2_Context_Memory_Snapshot_Save = telemetry.OpD2_S2_Context_Memory_Snapshot_Save
)

// Constants — D2 Context Engine - Harness (D2-S5)

const (
	OpD2_S5_Context_Harness_Bootstrap_Run    = telemetry.OpD2_S5_Context_Harness_Bootstrap_Run
	OpD2_S5_Context_Harness_Bootstrap_Stage  = telemetry.OpD2_S5_Context_Harness_Bootstrap_Stage
	OpD2_S5_Context_Harness_ToolPool         = telemetry.OpD2_S5_Context_Harness_ToolPool
	OpD2_S5_Context_Harness_Preflight        = telemetry.OpD2_S5_Context_Harness_Preflight
	OpD2_S5_Context_Harness_Route            = telemetry.OpD2_S5_Context_Harness_Route
	OpD2_S5_Context_Harness_SystemPrompt_Build = telemetry.OpD2_S5_Context_Harness_SystemPrompt_Build
)

// Constants — D2 Context Engine - QueryLoop (D2-S10)

const (
	OpD2_S10_Query_Loop_Run     = telemetry.OpD2_S10_Query_Loop_Run
	OpD2_S10_Query_Loop_Turn    = telemetry.OpD2_S10_Query_Loop_Turn
	OpD2_S10_Query_Loop_LLM_Call = telemetry.OpD2_S10_Query_Loop_LLM_Call
)

// Constants — D2 Context Engine - Tool Execution (D2-S5)

const (
	OpD2_S5_Tool_Execute_Single     = telemetry.OpD2_S5_Tool_Execute_Single
	OpD2_S5_Tool_Execute_Permission = telemetry.OpD2_S5_Tool_Execute_Permission
)

// Constants — D2 Context Engine - Task / Plan (D2-S8)

const (
	OpD2_S8_Task_Plan_Generate    = telemetry.OpD2_S8_Task_Plan_Generate
	OpD2_S8_Task_PlanMode_Enter   = telemetry.OpD2_S8_Task_PlanMode_Enter
	OpD2_S8_Task_PlanMode_Execute = telemetry.OpD2_S8_Task_PlanMode_Execute
	OpD2_S8_Task_PlanMode_Approve = telemetry.OpD2_S8_Task_PlanMode_Approve
	OpD2_S8_Task_PlanMode_Reject  = telemetry.OpD2_S8_Task_PlanMode_Reject
	OpD2_S8_Task_Manager_Create   = telemetry.OpD2_S8_Task_Manager_Create
	OpD2_S8_Task_Manager_Update   = telemetry.OpD2_S8_Task_Manager_Update
)

// Constants — D3 LLM Gateway (D3-S3)

const (
	OpD3_S3_LLM_Stream           = telemetry.OpD3_S3_LLM_Stream
	OpD3_S3_LLM_Provider_Route   = telemetry.OpD3_S3_LLM_Provider_Route
	OpD3_S3_LLM_CircuitBreaker   = telemetry.OpD3_S3_LLM_CircuitBreaker
	OpD3_S3_LLM_Retry            = telemetry.OpD3_S3_LLM_Retry
	OpD3_S3_LLM_Adapter_Stream   = telemetry.OpD3_S3_LLM_Adapter_Stream
)

// Constants — D4 MultiAgent (D4-S4)

const (
	OpD4_S4_Agent_Run             = telemetry.OpD4_S4_Agent_Run
	OpD4_S4_Agent_Tool_Call       = telemetry.OpD4_S4_Agent_Tool_Call
	OpD4_S4_Agent_Fork            = telemetry.OpD4_S4_Agent_Fork
	OpD4_S4_Agent_Join            = telemetry.OpD4_S4_Agent_Join
	OpD4_S4_Agent_Terminate       = telemetry.OpD4_S4_Agent_Terminate
	OpD4_S4_Agent_State_Transition = telemetry.OpD4_S4_Agent_State_Transition
)

// Constants — D7 Orchestration (D7-S3)

const (
	OpD7_S3_Orchestration_Wave_Schedule     = telemetry.OpD7_S3_Orchestration_Wave_Schedule
	OpD7_S3_Orchestration_Wave_Task_Execute = telemetry.OpD7_S3_Orchestration_Wave_Task_Execute
	OpD7_S3_Orchestration_Flow_Event_Publish = telemetry.OpD7_S3_Orchestration_Flow_Event_Publish
)

// Constants — D7 Task Manager (D7-S1)

const (
	OpD7_S1_Task_Manager_Create = telemetry.OpD7_S1_Task_Manager_Create
	OpD7_S1_Task_Manager_Update = telemetry.OpD7_S1_Task_Manager_Update
)

// Constants — D6 Evolution - Runtime Validation (D6-S4)

const OpD6_S4_Validation_Decision = telemetry.OpD6_S4_Validation_Decision

// Functions

var (
	SpanAttrs         = telemetry.SpanAttrs
	LayerAndComponent = telemetry.LayerAndComponent
	LLMUsageAttrs     = telemetry.LLMUsageAttrs
	GenAIPromptAttrs  = telemetry.GenAIPromptAttrs
	GenAIUsageAttrs   = telemetry.GenAIUsageAttrs
	RecordSpanError   = telemetry.RecordSpanError
)
