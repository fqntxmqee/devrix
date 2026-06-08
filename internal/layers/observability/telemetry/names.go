package telemetry

import (
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/observability/tracer"
)

// Devrix layer identifiers (map to L1–L3 architecture).
const (
	LayerCommunication = "communication"
	LayerContext       = "context"
	LayerLLM           = "llm"
)

// Canonical Jaeger / OTLP operation names: {layer}.{module}.{action}
const (
	OpGatewayMessageReceive = "gateway.message.receive"

	OpContextProcess        = "context.process"
	OpContextSnapshotLoad   = "context.snapshot.load"
	OpContextCompressionRun = "context.compression.run"

	OpContextPEVRun             = "context.pev.run"
	OpContextPEVLLMCall         = "context.pev.llm_call"
	OpContextPEVToolExecute     = "context.pev.tool_execute"
	OpContextPEVPermissionCheck = "context.pev.permission_check"
	OpContextPEVVerify          = "context.pev.verify"

	OpLLMStream = "llm.stream"

	OpAdapterMessageReceive = "adapter.message.receive"

	OpContextPlanGenerate   = "context.plan.generate"
	OpContextMilestoneRun   = "context.milestone.run"
	OpContextLongTermRecall = "context.longterm.recall"
	OpContextLongTermStore  = "context.longterm.store"

	OpGatewaySessionLifecycle = "gateway.session.lifecycle"
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
	case strings.HasPrefix(operation, "context."):
		return LayerContext, "context_engine"
	case strings.HasPrefix(operation, "llm."):
		return LayerLLM, "llm_gateway"
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
