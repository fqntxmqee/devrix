package telemetry_test

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/observability/telemetry"
)

func TestLayerAndComponent_should_map_gateway_operation(t *testing.T) {
	layer, component := telemetry.LayerAndComponent(telemetry.OpGatewayMessageReceive)
	if layer != telemetry.LayerCommunication || component != "gateway" {
		t.Fatalf("got layer=%q component=%q", layer, component)
	}
}

func TestLayerAndComponent_should_map_pev_operation(t *testing.T) {
	layer, component := telemetry.LayerAndComponent(telemetry.OpContextPEVToolExecute)
	if layer != telemetry.LayerContext || component != "pev_engine" {
		t.Fatalf("got layer=%q component=%q", layer, component)
	}
}

func TestSpanAttrs_should_include_layer_and_component(t *testing.T) {
	attrs := telemetry.SpanAttrs(telemetry.OpLLMStream)
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
