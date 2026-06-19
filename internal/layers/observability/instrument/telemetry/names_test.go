package telemetry_test

import (
	"fmt"
	"testing"

	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
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

func TestSpanAttrs_should_include_layer_and_component(t *testing.T) {
	attrs := telemetry.SpanAttrs(telemetry.OpD3_S3_LLM_Stream)
	if len(attrs) < 2 {
		t.Fatalf("attrs: %+v", attrs)
	}
	if attrs[0].Key != "devrix.layer" || attrs[0].Value != telemetry.LayerLLM {
		t.Fatalf("layer attr: %+v", attrs[0])
	}
	if attrs[1].Key != "devrix.component" || attrs[1].Value != "llm_gateway" {
		t.Fatalf("component attr: %+v", attrs[1])
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
