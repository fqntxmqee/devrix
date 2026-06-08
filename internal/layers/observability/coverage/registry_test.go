package coverage_test

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/observability/coverage"
	"github.com/devrix/devrix/internal/layers/observability/telemetry"
)

func TestAllOperations_should_match_telemetry_constants(t *testing.T) {
	t.Helper()
	expected := []string{
		telemetry.OpGatewayMessageReceive,
		telemetry.OpGatewaySessionLifecycle,
		telemetry.OpAdapterMessageReceive,
		telemetry.OpContextProcess,
		telemetry.OpContextSnapshotLoad,
		telemetry.OpContextCompressionRun,
		telemetry.OpContextPlanGenerate,
		telemetry.OpContextMilestoneRun,
		telemetry.OpContextLongTermRecall,
		telemetry.OpContextLongTermStore,
		telemetry.OpContextPEVRun,
		telemetry.OpContextPEVLLMCall,
		telemetry.OpContextPEVToolExecute,
		telemetry.OpContextPEVPermissionCheck,
		telemetry.OpContextPEVVerify,
		telemetry.OpLLMStream,
	}

	registry := coverage.AllOperations()
	if len(registry) != len(expected) {
		t.Fatalf("registry size %d, want %d", len(registry), len(expected))
	}

	byName := make(map[string]coverage.OperationMeta, len(registry))
	for _, op := range registry {
		byName[op.Name] = op
	}
	for _, name := range expected {
		meta, ok := byName[name]
		if !ok {
			t.Fatalf("missing operation %q in registry", name)
		}
		layer, component := telemetry.LayerAndComponent(name)
		if meta.Layer != layer || meta.Component != component {
			t.Fatalf("operation %q metadata layer=%q component=%q, want layer=%q component=%q",
				name, meta.Layer, meta.Component, layer, component)
		}
		if !meta.Instrumented {
			t.Fatalf("operation %q must be instrumented", name)
		}
	}
}
