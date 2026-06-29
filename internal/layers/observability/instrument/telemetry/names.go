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
	OpD2_S2_Context_Tools_List = "D2_Context_Tools_List"
	OpD2_S2_Context_Tools_Filter_Permission = "D2_Context_Tools_Filter_Permission"
	OpD2_S2_Context_Tools_Filter_AgentRole = "D2_Context_Tools_Filter_AgentRole"
	OpD2_S2_Context_Worker_Fork = "D2_Context_Worker_Fork"
	OpD2_S2_Context_Permission_Init = "D2_Context_Permission_Init"
	OpD2_S2_Context_Tier_Resolve = "D2_Context_Tier_Resolve"
	OpD2_S2_Context_Memory_Append = "D2_Context_Memory_Append"
	OpD2_S2_Context_CompressedView_Set = "D2_Context_CompressedView_Set"
	OpD2_S2_Context_Attachments_Collect = "D2_Context_Attachments_Collect"
	OpD2_S2_Context_Queue_Drain = "D2_Context_Queue_Drain"
	OpD2_S2_Context_Memory_Snapshot_Save = "D2_Context_Memory_Snapshot_Save"
	OpD2_S16_Context_Materialize = "D2_Context_Materialize"

	// D2 Context Engine - Harness (D2-S5, REMOVED v6.5.0; SystemPrompt_Build 在 prepare/adapters/ 复用)
	OpD2_S5_Context_Harness_SystemPrompt_Build = "D2_Context_Harness_SystemPrompt_Build"

	// D2 Context Engine - Tool Execution (D2-S5)
	OpD2_S5_Tool_Execute_Single = "D2_Tool_Execute_Single"

	// D2 Context Engine - Task / Plan (D2-S8)
	OpD2_S8_Task_Plan_Generate = "D2_Task_Plan_Generate"
	OpD2_S8_Task_PlanMode_Enter = "D2_Task_PlanMode_Enter"
	OpD2_S8_Task_PlanMode_Execute = "D2_Task_PlanMode_Execute"
	OpD2_S8_Task_PlanMode_Approve = "D2_Task_PlanMode_Approve"
	OpD2_S8_Task_PlanMode_Reject = "D2_Task_PlanMode_Reject"

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

	// D7 Orchestration - 6 S 精简 v6.0.0 新增 5 个 P0/P1 Span ops (2026-06-26)
	// D7-S3 WaveScheduler (executor.select P1)
	OpD7_S3_Executor_Select = "D7_Executor_Select"
	// D7-S4 ExecutionFlow + Verify (system.anomaly_detect P0)
	OpD7_S4_System_Anomaly_Detect = "D7_System_Anomaly_Detect"
	// D7-S5 DecisionPlanning + Observe (taskgraph.synthesize P1)
	OpD7_S5_TaskGraph_Synthesize = "D7_TaskGraph_Synthesize"
	// D7-S6 MUPS Pipeline (channel.route P0)
	OpD7_S6_Channel_Route = "D7_Channel_Route"
	// D7-S6 MUPS Pipeline (memory.persist P0)
	OpD7_S6_Memory_Persist = "D7_Memory_Persist"
	// D7-S6 MUPS Pipeline (5-node root span) — wraps the 5-node MUPS pipeline:
	// Observe (sessionSpan prior attrs) → Plan (taskgraph.synthesize) →
	// Wave (executor.select) → Execute (channel.route) → Verify (system.anomaly_detect) →
	// Learn (memory.persist, async via sessionCtx). Started in OrchestratePath.Run after
	// the outer Orchestrate_Run span; the 4 sync nodes inherit it as parent via ctx.
	OpD7_S6_MUPS_Pipeline = "D7_MUPS_Pipeline"

	// D7 Orchestration - inner observability spans (DM-20260626-009 follow-up,
	// 2026-06-26). The 5-node MUPS spans cover top-level pipeline nodes; the
	// three ops below cover the inner layers that were invisible in Jaeger:
	// the WorkItem task tree mutations (worktree), parallel-explore / child
	// WorkItem runs (subworktree), and the per-WorkItem ReAct loop iterations
	// (subturn). Without these, debugging a slow WorkItem meant reading the
	// code instead of inspecting traces.
	//
	// D7-S1 Worktree (worktree.op P1)
	OpD7_S1_Worktree_Op = "D7_Worktree_Op"
	// D7-S1 SubWorktree (subworktree.run P2)
	OpD7_S1_SubWorktree_Run = "D7_SubWorktree_Run"
	// D7-S5 SubTurn (subturn.iteration P1)
	OpD7_S5_SubTurn_Iteration = "D7_SubTurn_Iteration"

	// D7 Orchestration - t-span-coverage 5 ops (DM-20260629-001 PR-6, T35,
	// 2026-06-29). The 6 S inner spans above cover WorkItem-level mutations;
	// the five ops below cover the long-running reputation learning, anomaly
	// triggers, prior injection, resume decision paths, and the cross-domain
	// IM card render that closes the D7→D1 loop. Together they raise the
	// T↔Span coverage from ~38% to ≥80%.

	// D7-S2 SessionOrchestrator (resume.decision_path P0, ApplyResumeSession
	// 3 决策路由 A fall-through / B user_accept→ForceExit / C user_cancel→
	// AbortWithAudit).
	OpD7_S2_Resume_Decision_Path = "D7_Resume_Decision_Path"
	// D7-S5 DecisionPlanning + Observe (adaptive_prior.inject P0, buildObserveRequest
	// learner.Inject 注入路径, 跨 S5↔S6 数据契约).
	OpD7_S5_AdaptivePrior_Inject = "D7_AdaptivePrior_Inject"
	// D7-S4 ExecutionFlow + Verify (anomaly.trigger P0, SystemAnomalyDetector
	// 阈值触发, 高/中严重度路径).
	OpD7_S4_Anomaly_Trigger = "D7_Anomaly_Trigger"
	// D7-S6 MUPS Pipeline (longterm.reputation_update P0, BayesianUpdate 后
	// 长程信誉落盘, LP-1 闭环 acceptance).
	OpD7_S6_LongTerm_Reputation_Update = "D7_LongTerm_Reputation_Update"
	// D7 Orchestration × D1 Communication (feishu.card_render P0, 飞书卡片
	// finalizeReplyCardStreaming 渲染 span, D7→D1 跨域可观测).
	OpD7_Feishu_Card_Render = "D7_Feishu_Card_Render"

	// D6 Evolution - Runtime Validation (D6-S4)
	OpD6_S4_Validation_Decision = "D6_Validation_Decision"

	// --- DM-20260629-009 PR-C: TaskContract S18 spans ---
	// PR-C introduces 3 inner-layer Span ops for AC13/14/15 (CoW
	// VersionChain, Similarity Check, Hard Evidence). Names use the
	// D7_ prefix and live alongside the existing S1-S6 spans; the S18
	// sub-name encodes the change-id so dashboards can filter.
	OpD7_S18_Hard_Evidence_Reject         = "D7_Hard_Evidence_Reject"
	OpD7_S18_Worktree_VersionChain_Append = "D7_Worktree_VersionChain_Append"
	OpD7_S18_Similarity_Check_Intercept   = "D7_Similarity_Check_Intercept"
)

// SpanAttrs returns the extra attributes only. Layer/component are intentionally
// omitted because the canonical operation name (D{N}_* prefix) already encodes
// the layer and component — see LayerAndComponent for ad-hoc lookups.
func SpanAttrs(operation string, extra ...tracer.Attribute) []tracer.Attribute {
	_ = operation
	return extra
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
		// 仅 SystemPrompt_Build 仍在 prepare/adapters/assembler_adapter.go
		// 复用（S5 harness 主体 v6.5.0 已 REMOVED）。
		return LayerContext, "harness"
	case strings.HasPrefix(operation, "D2_Context_"):
		return LayerContext, "context_engine"
	case strings.HasPrefix(operation, "D2_Tool_"):
		return LayerContext, "tool_runner"
	case strings.HasPrefix(operation, "D2_Task_PlanMode_"):
		return LayerContext, "plan_mode"
	case strings.HasPrefix(operation, "D2_Task_Plan_"):
		return LayerContext, "plan_agent"

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
	case strings.HasPrefix(operation, "D7_Executor_Select"),
		strings.HasPrefix(operation, "D7_System_Anomaly_Detect"),
		strings.HasPrefix(operation, "D7_TaskGraph_Synthesize"),
		strings.HasPrefix(operation, "D7_Channel_Route"),
		strings.HasPrefix(operation, "D7_Memory_Persist"),
		strings.HasPrefix(operation, "D7_MUPS_Pipeline"):
		// v6.0.0 6 S 精简新增 5 ops (channel.kind / memory / system / taskgraph / executor)
		// + 1 5-node pipeline root span (D7_MUPS_Pipeline).
		return LayerOrchestration, "orchestrator"

	case strings.HasPrefix(operation, "D7_Worktree_Op"),
		strings.HasPrefix(operation, "D7_SubWorktree_Run"):
		// DM-20260626-009 follow-up inner observability spans (PR #254 + #257).
		// WorkItem task tree mutations + parallel-explore / child WorkItem
		// runs → worktree component (D7-S1).
		return LayerOrchestration, "worktree"

	case strings.HasPrefix(operation, "D7_SubTurn_Iteration"):
		// DM-20260626-009 follow-up inner observability spans (PR #254 + #257).
		// Per-WorkItem ReAct loop iterations (D7-S5).
		return LayerOrchestration, "executor"

	case strings.HasPrefix(operation, "D7_Resume_Decision_Path"),
		strings.HasPrefix(operation, "D7_AdaptivePrior_Inject"),
		strings.HasPrefix(operation, "D7_Anomaly_Trigger"),
		strings.HasPrefix(operation, "D7_LongTerm_Reputation_Update"):
		// DM-20260629-001 PR-6 t-span-coverage 5 ops (T35). Long-running
		// reputation learning, anomaly triggers, prior injection and resume
		// decision paths all live in the orchestrator component.
		return LayerOrchestration, "orchestrator"

	case strings.HasPrefix(operation, "D7_Feishu_Card_Render"):
		// DM-20260629-001 PR-6 (T35). Cross-domain D7→D1 finalise: still an
		// orchestration emit but tagged communication so Jaeger filters
		// match the D1_S17_Adapter_Feishu_Outbound lineage.
		return LayerOrchestration, "adapter"

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
