package coverage_test

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/observability/coverage"
	"github.com/devrix/devrix/internal/layers/observability/telemetry"
)

func TestAllOperations_should_match_telemetry_constants(t *testing.T) {
	t.Helper()
	expected := []string{
		telemetry.OpAdapterCLISend,
		telemetry.OpAdapterFeishuOutbound,
		telemetry.OpAdapterMessageReceive,

		telemetry.OpAgentFork,
		telemetry.OpAgentJoin,
		telemetry.OpAgentRun,
		telemetry.OpAgentStateTransition,
		telemetry.OpAgentTerminate,
		telemetry.OpAgentToolCall,

		telemetry.OpContextCompressionRun,
	telemetry.OpContextCompressionStep,
		telemetry.OpContextHarnessBootstrapRun,
		telemetry.OpContextHarnessBootstrapStage,
		telemetry.OpContextHarnessPreflight,
		telemetry.OpContextHarnessRoute,
		telemetry.OpContextHarnessToolPool,
		telemetry.OpContextLongTermRecall,
		telemetry.OpContextLongTermStore,
	telemetry.OpContextMemorySnapshotSave,
		telemetry.OpContextProcess,
		telemetry.OpContextSnapshotLoad,
	telemetry.OpContextSystemPromptLoad,
		telemetry.OpContextSystemPromptBuild,
	telemetry.OpContextToolsRegister,

		telemetry.OpGatewayAgentCreate,
		telemetry.OpGatewayEngineEvent,
		telemetry.OpGatewayMessageReceive,
		telemetry.OpGatewayPermissionCheck,
		telemetry.OpGatewaySessionCreate,
		telemetry.OpGatewaySessionExpire,
		telemetry.OpGatewaySessionGet,
		telemetry.OpGatewaySessionLifecycle,
		telemetry.OpGatewayStoreCreate,
		telemetry.OpGatewayStoreDelete,
		telemetry.OpGatewayStoreGet,
		telemetry.OpGatewayStoreUpdate,

		telemetry.OpD1CapturePersist,
		telemetry.OpD1DispatchRoute,
		telemetry.OpD1SignalThinking,
		telemetry.OpD1SignalTask,
		telemetry.OpD1SignalConclusion,
		telemetry.OpD1SignalChainIntegrity,
		telemetry.OpD1SignalTaskWorkProof,
		telemetry.OpUserFeedbackConclusionRejected,

		telemetry.OpLLMAdapterStream,
		telemetry.OpLLMCircuitBreaker,
		telemetry.OpLLMProviderRoute,
		telemetry.OpLLMRetry,
		telemetry.OpLLMStream,

		telemetry.OpQueryLoopRun,
		telemetry.OpQueryLoopTurn,
		telemetry.OpQueryLoopLLMCall,

		telemetry.OpOrchFlowEventPublish,
	telemetry.OpOrchWaveSchedule,
	telemetry.OpOrchWaveTaskExecute,

	telemetry.OpToolExecuteSingle,
		telemetry.OpToolExecutePermission,

		telemetry.OpTaskPlanGenerate,
		telemetry.OpTaskPlanModeEnter,
		telemetry.OpTaskPlanModeExecute,
		telemetry.OpTaskPlanModeApprove,
		telemetry.OpTaskPlanModeReject,
		telemetry.OpTaskManagerCreate,
		telemetry.OpTaskManagerUpdate,
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
