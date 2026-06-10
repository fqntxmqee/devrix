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
)

// Canonical Jaeger / OTLP operation names: {layer}.{module}.{action}
const (
	OpGatewayMessageReceive = "gateway.message.receive"
	OpGatewaySessionLifecycle = "gateway.session.lifecycle"
	OpGatewaySessionCreate = "gateway.session.create"
	OpGatewaySessionGet     = "gateway.session.get"
	OpGatewaySessionExpire  = "gateway.session.expire"
	OpGatewayStoreCreate    = "gateway.store.create"
	OpGatewayStoreGet       = "gateway.store.get"
	OpGatewayStoreUpdate    = "gateway.store.update"
	OpGatewayStoreDelete    = "gateway.store.delete"
	OpGatewayPermissionCheck = "gateway.permission.check"
	OpGatewayAgentCreate    = "gateway.agent.create"
	OpGatewayEngineEvent    = "gateway.engine_event.handle"

	OpContextProcess        = "context.process"
	OpContextSnapshotLoad   = "context.snapshot.load"
	OpContextSystemPromptLoad = "context.system_prompt.load"
	OpContextCompressionRun = "context.compression.run"
	OpContextPlanGenerate   = "context.plan.generate"
	OpContextMilestoneRun   = "context.milestone.run"
	OpContextLongTermRecall = "context.longterm.recall"
	OpContextLongTermStore  = "context.longterm.store"
	OpContextVerifyCommand  = "context.verify.command"
	OpContextToolsRegister   = "context.tools.register"
	OpContextMemorySnapshotSave = "context.memory.snapshot.save"

	OpContextHarnessBootstrapRun = "context.harness.bootstrap.run"
	OpContextHarnessBootstrapStage = "context.harness.bootstrap.stage"
	OpContextHarnessToolPool       = "context.harness.tool_pool"
	OpContextHarnessPreflight      = "context.harness.preflight"
	OpContextHarnessRoute          = "context.harness.route"
	OpContextSystemPromptBuild     = "context.system_prompt.build"

	OpContextPEVRun             = "context.pev.run"
	OpContextPEVLLMCall         = "context.pev.llm_call"
	OpContextPEVIteration       = "context.pev.iteration"
	OpContextPEVSynthesis       = "context.pev.synthesis"
	OpContextPEVToolExecute     = "context.pev.tool_execute"
	OpContextPEVPermissionCheck = "context.pev.permission_check"
	OpContextPEVVerify          = "context.pev.verify"

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
	case strings.HasPrefix(operation, "gateway."):
		return LayerCommunication, "gateway"
	case strings.HasPrefix(operation, "adapter."):
		return LayerCommunication, "adapter"
	case strings.HasPrefix(operation, "context.pev."):
		return LayerContext, "pev_engine"
	case strings.HasPrefix(operation, "context.milestone."):
		return LayerContext, "pev_engine"
	case strings.HasPrefix(operation, "context.plan."):
		return LayerContext, "context_engine"
	case strings.HasPrefix(operation, "context.longterm."):
		return LayerContext, "context_engine"
	case strings.HasPrefix(operation, "context.harness."):
		return LayerContext, "harness"
	case operation == OpContextSystemPromptBuild:
		return LayerContext, "harness"
	case strings.HasPrefix(operation, "context.verify."):
		return LayerContext, "context_engine"
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
