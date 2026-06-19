package telemetry

import (
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
)

// Devrix layer identifiers (map to L1–L6 architecture).
const (
	LayerCommunication = "communication"
	LayerContext       = "context"
	LayerLLM           = "llm"
	LayerAgent         = "agent" // D4 multi-agent layer
	LayerOrchestration = "orchestration"
	LayerEvolution     = "evolution"
)

// Canonical Jaeger / OTLP operation names: D{N}_{场景名称}_{动作}_{细节}（运行时字符串不含 S 编号）
const (
	// D1 Communication - Gateway / Capture (D1-S13)
	OpD1_S13_Capture_Message_Receive = "D1_Capture_Message_Receive"
	OpD1_S13_Capture_Session_Lifecycle = "D1_Capture_Session_Lifecycle"
	OpD1_S13_Capture_Session_Create = "D1_Capture_Session_Create"
	OpD1_S13_Capture_Session_Get = "D1_Capture_Session_Get"
	OpD1_S13_Capture_Session_Expire = "D1_Capture_Session_Expire"
	OpD1_S13_Capture_Store_Create = "D1_Capture_Store_Create"
	OpD1_S13_Capture_Store_Get = "D1_Capture_Store_Get"
	OpD1_S13_Capture_Store_Update = "D1_Capture_Store_Update"
	OpD1_S13_Capture_Store_Delete = "D1_Capture_Store_Delete"
	OpD1_S13_Capture_Permission_Check = "D1_Capture_Permission_Check"
	OpD1_S13_Capture_Agent_Create = "D1_Capture_Agent_Create"
	OpD1_S13_Capture_EngineEvent_Handle = "D1_Capture_EngineEvent_Handle"

	// D1 Communication - Signal (D1-S14~S16)
	OpD1_S13_Capture_Persist = "D1_Capture_Persist"
	OpD1_S13_Dispatch_Route = "D1_Dispatch_Route"
	OpD1_S14_Signal_Thinking = "D1_Signal_Thinking"
	OpD1_S15_Signal_Task = "D1_Signal_Task"
	OpD1_S16_Signal_Conclusion = "D1_Signal_Conclusion"
	OpD1_S14_Signal_ChainIntegrity = "D1_Signal_ChainIntegrity"
	OpD1_S15_Signal_TaskWorkProof = "D1_Signal_TaskWorkProof"
	OpD1_S16_UserFeedback_ConclusionRejected = "D1_UserFeedback_ConclusionRejected"

	// D1 Communication - Adapter (D1-S17)
	OpD1_S17_Adapter_Message_Receive = "D1_Adapter_Message_Receive"
	OpD1_S17_Adapter_CLI_Send = "D1_Adapter_CLI_Send"
	OpD1_S17_Adapter_Feishu_Outbound = "D1_Adapter_Feishu_Outbound"

	// D2 Context Engine - Core (D2-S2)
	OpD2_S2_Context_Process = "D2_Context_Process"
	OpD2_S2_Context_Snapshot_Load = "D2_Context_Snapshot_Load"
	OpD2_S2_Context_SystemPrompt_Load = "D2_Context_SystemPrompt_Load"
	OpD2_S2_Context_Compression_Run = "D2_Context_Compression_Run"
	OpD2_S2_Context_Compression_Step = "D2_Context_Compression_Step"
	OpD2_S2_Context_Longterm_Recall = "D2_Context_Longterm_Recall"
	OpD2_S2_Context_Longterm_Store = "D2_Context_Longterm_Store"
	OpD2_S2_Context_Tools_Register = "D2_Context_Tools_Register"
	OpD2_S2_Context_Memory_Snapshot_Save = "D2_Context_Memory_Snapshot_Save"

	// D2 Context Engine - Harness (D2-S5)
	OpD2_S5_Context_Harness_Bootstrap_Run = "D2_Context_Harness_Bootstrap_Run"
	OpD2_S5_Context_Harness_Bootstrap_Stage = "D2_Context_Harness_Bootstrap_Stage"
	OpD2_S5_Context_Harness_ToolPool = "D2_Context_Harness_ToolPool"
	OpD2_S5_Context_Harness_Preflight = "D2_Context_Harness_Preflight"
	OpD2_S5_Context_Harness_Route = "D2_Context_Harness_Route"
	OpD2_S5_Context_Harness_SystemPrompt_Build = "D2_Context_Harness_SystemPrompt_Build"

	// D2 Context Engine - Tool Execution (D2-S5)
	OpD2_S5_Tool_Execute_Single = "D2_Tool_Execute_Single"
	OpD2_S5_Tool_Execute_Permission = "D2_Tool_Execute_Permission"

	// D2 Context Engine - Task / Plan (D2-S8)
	OpD2_S8_Task_Plan_Generate = "D2_Task_Plan_Generate"
	OpD2_S8_Task_PlanMode_Enter = "D2_Task_PlanMode_Enter"
	OpD2_S8_Task_PlanMode_Execute = "D2_Task_PlanMode_Execute"
	OpD2_S8_Task_PlanMode_Approve = "D2_Task_PlanMode_Approve"
	OpD2_S8_Task_PlanMode_Reject = "D2_Task_PlanMode_Reject"
	OpD2_S8_Task_Manager_Create = "D2_Task_Manager_Create"
	OpD2_S8_Task_Manager_Update = "D2_Task_Manager_Update"

	// D3 LLM Gateway (D3-S3)
	OpD3_S3_LLM_Stream = "D3_LLM_Stream"
	OpD3_S3_LLM_Provider_Route = "D3_LLM_Provider_Route"
	OpD3_S3_LLM_CircuitBreaker = "D3_LLM_CircuitBreaker"
	OpD3_S3_LLM_Retry = "D3_LLM_Retry"
	OpD3_S3_LLM_Adapter_Stream = "D3_LLM_Adapter_Stream"

	// D4 MultiAgent (D4-S4)
	OpD4_S4_Agent_Run = "D4_Agent_Run"
	OpD4_S4_Agent_Tool_Call = "D4_Agent_Tool_Call"
	OpD4_S4_Agent_Fork = "D4_Agent_Fork"
	OpD4_S4_Agent_Join = "D4_Agent_Join"
	OpD4_S4_Agent_Terminate = "D4_Agent_Terminate"
	OpD4_S4_Agent_State_Transition = "D4_Agent_State_Transition"

	// D7 Orchestration - Session / Turn (D7-S2)
	OpD7_S2_Orchestration_Session_Process = "D7_Orchestration_Session_Process"
	OpD7_S2_Orchestration_Intent_Classify = "D7_Orchestration_Intent_Classify"
	OpD7_S2_Orchestration_Turn_Run        = "D7_Orchestration_Turn_Run"
	OpD7_S2_Orchestration_Turn_Iteration  = "D7_Orchestration_Turn_Iteration"
	OpD7_S2_Orchestration_LLM_Invoke      = "D7_Orchestration_LLM_Invoke"
	OpD7_S2_Orchestration_Orchestrate_Run = "D7_Orchestration_Orchestrate_Run"

	// D7 Orchestration (D7-S3)
	OpD7_S3_Orchestration_Wave_Schedule = "D7_Orchestration_Wave_Schedule"
	OpD7_S3_Orchestration_Wave_Task_Execute = "D7_Orchestration_Wave_Task_Execute"
	OpD7_S3_Orchestration_Flow_Event_Publish = "D7_Orchestration_Flow_Event_Publish"

	// D7 Task Manager (D7-S1)
	OpD7_S1_Task_Manager_Create = "D7_Task_Manager_Create"
	OpD7_S1_Task_Manager_Update = "D7_Task_Manager_Update"

	// D6 Evolution - Runtime Validation (D6-S4)
	OpD6_S4_Validation_Decision = "D6_Validation_Decision"
)

// SpanAttrs returns standard devrix.layer / devrix.component attributes plus extras.
func SpanAttrs(operation string, extra ...tracer.Attribute) []tracer.Attribute {
	layer, component := LayerAndComponent(operation)
	attrs := []tracer.Attribute{
		{Key: "devrix.layer", Value: layer},
		{Key: "devrix.component", Value: component},
	}
	return append(attrs, extra...)
}

// LayerAndComponent maps an operation name to Jaeger filter dimensions.
func LayerAndComponent(operation string) (layer, component string) {
	switch {
	// D1 Communication
	case strings.HasPrefix(operation, "D1_Adapter_"):
		return LayerCommunication, "adapter"
	case strings.HasPrefix(operation, "D1_"):
		return LayerCommunication, "gateway"

	// D2 Context Engine
	case strings.HasPrefix(operation, "D2_Context_Harness_"):
		return LayerContext, "harness"
	case strings.HasPrefix(operation, "D2_Context_"):
		return LayerContext, "context_engine"
	case strings.HasPrefix(operation, "D2_Tool_"):
		return LayerContext, "tool_runner"
	case strings.HasPrefix(operation, "D2_Task_PlanMode_"):
		return LayerContext, "plan_mode"
	case strings.HasPrefix(operation, "D2_Task_Plan_"):
		return LayerContext, "plan_agent"
	case strings.HasPrefix(operation, "D2_Task_Manager_"):
		return LayerContext, "task_manager"

	// D3 LLM Gateway
	case strings.HasPrefix(operation, "D3_LLM_Adapter_Stream"):
		return LayerLLM, "llm_adapter"
	case strings.HasPrefix(operation, "D3_LLM_"):
		return LayerLLM, "llm_gateway"

	// D4 MultiAgent
	case strings.HasPrefix(operation, "D4_Agent_"):
		return LayerAgent, "agent_tool"

	// D7 Orchestration
	case strings.HasPrefix(operation, "D7_Orchestration_"):
		return LayerOrchestration, "orchestrator"
	case strings.HasPrefix(operation, "D7_Task_Manager_"):
		return LayerOrchestration, "task_manager"

	// D6 Evolution
	case strings.HasPrefix(operation, "D6_Validation_"):
		return LayerEvolution, "validation"

	default:
		return LayerContext, "devrix"
	}
}

// LLMUsageAttrs formats token usage for span attributes.
func LLMUsageAttrs(promptTokens, completionTokens int, latencyMs int64) []tracer.Attribute {
	return []tracer.Attribute{
		{Key: "llm.tokens.prompt", Value: fmt.Sprintf("%d", promptTokens)},
		{Key: "llm.tokens.completion", Value: fmt.Sprintf("%d", completionTokens)},
		{Key: "llm.latency_ms", Value: fmt.Sprintf("%d", latencyMs)},
	}
}

// GenAIUsageAttrs returns OTel GenAI semantic convention attributes (dual-written alongside llm.*).
// GenAIPromptAttrs returns prompt version metadata for system prompt build spans.
func GenAIPromptAttrs(version, templateHash, agentsMDHash string) []tracer.Attribute {
	attrs := []tracer.Attribute{}
	if version != "" {
		attrs = append(attrs, tracer.Attribute{Key: "gen_ai.prompt.version", Value: version})
	}
	if templateHash != "" {
		attrs = append(attrs, tracer.Attribute{Key: "gen_ai.prompt.template_hash", Value: templateHash})
	}
	if agentsMDHash != "" {
		attrs = append(attrs, tracer.Attribute{Key: "gen_ai.prompt.agents_md_hash", Value: agentsMDHash})
	}
	return attrs
}

func GenAIUsageAttrs(model, sessionID string, promptTokens, completionTokens, cacheReadTokens, reasoningTokens int, finishReason string) []tracer.Attribute {
	attrs := []tracer.Attribute{
		{Key: "gen_ai.request.model", Value: model},
		{Key: "gen_ai.usage.input_tokens", Value: fmt.Sprintf("%d", promptTokens)},
		{Key: "gen_ai.usage.output_tokens", Value: fmt.Sprintf("%d", completionTokens)},
		{Key: "gen_ai.agent.name", Value: "devrix"},
	}
	if cacheReadTokens > 0 {
		attrs = append(attrs, tracer.Attribute{Key: "gen_ai.usage.cache_read.input_tokens", Value: fmt.Sprintf("%d", cacheReadTokens)})
	}
	if reasoningTokens > 0 {
		attrs = append(attrs, tracer.Attribute{Key: "gen_ai.usage.reasoning.output_tokens", Value: fmt.Sprintf("%d", reasoningTokens)})
	}
	if sessionID != "" {
		attrs = append(attrs, tracer.Attribute{Key: "gen_ai.conversation.id", Value: sessionID})
	}
	if finishReason != "" {
		attrs = append(attrs, tracer.Attribute{Key: "gen_ai.response.finish_reasons", Value: finishReason})
	}
	return attrs
}
