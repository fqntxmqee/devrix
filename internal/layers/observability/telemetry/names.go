package telemetry

import (
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/observability/tracer"
)

// Devrix layer identifiers (map to L1–L6 architecture).
const (
	LayerCommunication = "communication"
	LayerContext       = "context"
	LayerLLM           = "llm"
	LayerAgent         = "agent" // D4 multi-agent layer
	LayerOrchestration = "orchestration"
)

// Canonical Jaeger / OTLP operation names: {layer}.{module}.{action}
const (
	OpGatewayMessageReceive = "capture.message.receive"
	OpGatewaySessionLifecycle = "capture.session.lifecycle"
	OpGatewaySessionCreate = "capture.session.create"
	OpGatewaySessionGet     = "capture.session.get"
	OpGatewaySessionExpire  = "capture.session.expire"
	OpGatewayStoreCreate    = "capture.store.create"
	OpGatewayStoreGet       = "capture.store.get"
	OpGatewayStoreUpdate    = "capture.store.update"
	OpGatewayStoreDelete    = "capture.store.delete"
	OpGatewayPermissionCheck = "capture.permission.check"
	OpGatewayAgentCreate    = "capture.agent.create"
	OpGatewayEngineEvent    = "capture.engine_event.handle"

	OpD1CapturePersist           = "d1.capture.persist"
	OpD1DispatchRoute            = "d1.dispatch.route"
	OpD1SignalThinking           = "d1.signal.thinking"
	OpD1SignalTask               = "d1.signal.task"
	OpD1SignalConclusion         = "d1.signal.conclusion"
	OpD1SignalChainIntegrity     = "d1.signal.chain_integrity"
	OpD1SignalTaskWorkProof      = "d1.signal.task.work_proof"
	OpUserFeedbackConclusionRejected = "user.feedback.conclusion_rejected"

	OpContextProcess        = "context.process"
	OpContextSnapshotLoad   = "context.snapshot.load"
	OpContextSystemPromptLoad = "context.system_prompt.load"
	OpContextCompressionRun  = "context.compression.run"
	OpContextCompressionStep = "context.compression.step"
	OpContextLongTermRecall = "context.longterm.recall"
	OpContextLongTermStore  = "context.longterm.store"
	OpContextToolsRegister   = "context.tools.register"
	OpContextMemorySnapshotSave = "context.memory.snapshot.save"

	OpContextHarnessBootstrapRun = "context.harness.bootstrap.run"
	OpContextHarnessBootstrapStage = "context.harness.bootstrap.stage"
	OpContextHarnessToolPool       = "context.harness.tool_pool"
	OpContextHarnessPreflight      = "context.harness.preflight"
	OpContextHarnessRoute          = "context.harness.route"
	OpContextSystemPromptBuild     = "context.system_prompt.build"

	OpLLMStream         = "llm.stream"
	OpLLMProviderRoute  = "llm.provider.route"
	OpLLMCircuitBreaker = "llm.circuit_breaker"
	OpLLMRetry          = "llm.retry"
	OpLLMAdapterStream  = "llm.adapter.stream"

	OpAdapterMessageReceive = "adapter.message.receive"
	OpAdapterCLISend       = "adapter.cli.send"
	OpAdapterFeishuOutbound = "adapter.feishu.outbound"

	// D4 multi-agent layer
	OpAgentToolCall        = "agent.tool.call"
	OpAgentRun             = "agent.run"
	OpAgentFork            = "agent.fork"
	OpAgentJoin            = "agent.join"
	OpAgentTerminate       = "agent.terminate"
	OpAgentStateTransition = "agent.state.transition"

	// QueryLoop (D2-S10)
	OpQueryLoopRun     = "query.loop.run"
	OpQueryLoopTurn    = "query.loop.turn"
	OpQueryLoopLLMCall = "query.loop.llm.call"

	// Tool Execution (D2-S5)
	OpToolExecuteSingle     = "tool.execute.single"
	OpToolExecutePermission = "tool.execute.permission"

	// Task / Plan (D2-S8)
	OpTaskPlanGenerate    = "task.plan.generate"
	OpTaskPlanModeEnter   = "task.plan_mode.enter"
	OpTaskPlanModeExecute = "task.plan_mode.execute"
	OpTaskPlanModeApprove = "task.plan_mode.approve"
	OpTaskPlanModeReject  = "task.plan_mode.reject"
	OpTaskManagerCreate   = "task.manager.create"
	OpTaskManagerUpdate   = "task.manager.update"

	// Orchestration (D5 orchestration layer)
	OpOrchWaveSchedule   = "orchestration.wave.schedule"
	OpOrchWaveTaskExecute = "orchestration.wave.task.execute"
	OpOrchFlowEventPublish = "orchestration.flow.event.publish"
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
	case strings.HasPrefix(operation, "capture."):
		return LayerCommunication, "gateway"
	case strings.HasPrefix(operation, "d1."):
		return LayerCommunication, "gateway"
	case strings.HasPrefix(operation, "user.feedback."):
		return LayerCommunication, "gateway"
	case strings.HasPrefix(operation, "adapter."):
		return LayerCommunication, "adapter"
	case strings.HasPrefix(operation, "context.longterm."):
		return LayerContext, "context_engine"
	case strings.HasPrefix(operation, "context.harness."):
		return LayerContext, "harness"
	case operation == OpContextSystemPromptBuild:
		return LayerContext, "harness"
	case strings.HasPrefix(operation, "context."):
		return LayerContext, "context_engine"
	case strings.HasPrefix(operation, "llm.provider."):
		return LayerLLM, "llm_gateway"
	case strings.HasPrefix(operation, "llm.circuit_breaker"):
		return LayerLLM, "llm_gateway"
	case strings.HasPrefix(operation, "llm.retry"):
		return LayerLLM, "llm_gateway"
	case strings.HasPrefix(operation, "llm.adapter."):
		return LayerLLM, "llm_adapter"
	case strings.HasPrefix(operation, "llm."):
		return LayerLLM, "llm_gateway"
	case strings.HasPrefix(operation, "agent."):
		return LayerAgent, "agent_tool"
	case strings.HasPrefix(operation, "query."):
		return LayerContext, "query_loop"
	case strings.HasPrefix(operation, "tool.execute."):
		return LayerContext, "tool_runner"
	case strings.HasPrefix(operation, "tool."):
		return LayerContext, "tool_runner"
	case strings.HasPrefix(operation, "task.plan_mode."):
		return LayerContext, "plan_mode"
	case strings.HasPrefix(operation, "task.plan."):
		return LayerContext, "plan_agent"
	case strings.HasPrefix(operation, "task.manager."):
		return LayerContext, "task_manager"
	case strings.HasPrefix(operation, "task."):
		return LayerContext, "task_manager"
	case strings.HasPrefix(operation, "orchestration."):
		return LayerOrchestration, "orchestrator"
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
