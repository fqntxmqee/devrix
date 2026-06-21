package telemetry_test

import (
	"fmt"
	"testing"

	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
)

func TestLayerAndComponent_should_map_gateway_operation(t *testing.T) {
	layer, component := telemetry.LayerAndComponent(telemetry.OpD1_S13_Capture_Message_Receive)
	if layer != telemetry.LayerCommunication || component != "gateway" {
		t.Fatalf("got layer=%q component=%q", layer, component)
	}
}


func TestLayerAndComponent_should_map_evolution_validation_operation(t *testing.T) {
	layer, component := telemetry.LayerAndComponent(telemetry.OpD6_S4_Validation_Decision)
	if layer != telemetry.LayerEvolution || component != "validation" {
		t.Fatalf("got layer=%q component=%q", layer, component)
	}
}

func TestSpanAttrs_should_not_inject_layer_or_component(t *testing.T) {
	attrs := telemetry.SpanAttrs(telemetry.OpD3_S3_LLM_Stream)
	for _, a := range attrs {
		if a.Key == "devrix.layer" || a.Key == "devrix.component" {
			t.Fatalf("SpanAttrs must not inject %q (encoded in operation name)", a.Key)
		}
	}
}

func TestSpanAttrs_should_pass_through_extras(t *testing.T) {
	extra := tracer.Attribute{Key: "tool.name", Value: "read_file"}
	attrs := telemetry.SpanAttrs(telemetry.OpD3_S3_LLM_Stream, extra)
	if len(attrs) != 1 || attrs[0].Key != "tool.name" || attrs[0].Value != "read_file" {
		t.Fatalf("extras passthrough failed: %+v", attrs)
	}
}

func TestLayerAndComponent_should_still_resolve_for_explicit_lookups(t *testing.T) {
	layer, component := telemetry.LayerAndComponent(telemetry.OpD3_S3_LLM_Stream)
	if layer != telemetry.LayerLLM || component != "llm_gateway" {
		t.Fatalf("LayerAndComponent regressed: layer=%q component=%q", layer, component)
	}
}

func TestGenAIUsageAttrs_should_include_otel_semantics(t *testing.T) {
	attrs := telemetry.GenAIUsageAttrs("deepseek-chat", "sess-1", 10, 20, 0, 0, "stop")
	keys := make(map[string]bool)
	for _, a := range attrs {
		keys[a.Key] = true
	}
	for _, want := range []string{
		"gen_ai.request.model", "gen_ai.usage.input_tokens", "gen_ai.usage.output_tokens",
		"gen_ai.agent.name", "gen_ai.conversation.id", "gen_ai.response.finish_reasons",
	} {
		if !keys[want] {
			t.Fatalf("missing attribute %q", want)
		}
	}
}

func TestGenAIUsageAttrs_should_include_breakdown_when_present(t *testing.T) {
	attrs := telemetry.GenAIUsageAttrs("o3", "sess-2", 100, 50, 80, 30, "stop")
	keys := make(map[string]string)
	for _, a := range attrs {
		keys[a.Key] = fmt.Sprintf("%v", a.Value)
	}
	if keys["gen_ai.usage.cache_read.input_tokens"] != "80" {
		t.Fatalf("cache_read attr: %+v", keys)
	}
	if keys["gen_ai.usage.reasoning.output_tokens"] != "30" {
		t.Fatalf("reasoning attr: %+v", keys)
	}
}
