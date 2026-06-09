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
	OpContextCompressionRun = "context.compression.run"
	OpContextPlanGenerate   = "context.plan.generate"
	OpContextMilestoneRun   = "context.milestone.run"
	OpContextLongTermRecall = "context.longterm.recall"
	OpContextLongTermStore  = "context.longterm.store"
	OpContextVerifyCommand  = "context.verify.command"

	OpContextPEVRun             = "context.pev.run"
	OpContextPEVLLMCall         = "context.pev.llm_call"
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
