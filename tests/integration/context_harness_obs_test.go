//go:build integration && d2

package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/layers/contextengine/registry"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/settings"
	"github.com/devrix/devrix/internal/layers/observability/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/tracer"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

func newHarnessObs(t *testing.T) (*observability.Observability, *observability.Bridge) {
	t.Helper()
	obs, err := observability.New(&observability.Config{
		Enabled: true,
		Tracing: settings.TracingConfig{
			Enabled:     true,
			ServiceName: "test-harness",
			Exporter:    "memory",
			Sampling:    settings.SamplingConfig{Type: "always_on", Rate: 1.0},
		},
		Metrics: observability.MetricsConfig{Enabled: false, Exporter: "null"},
		Logging: observability.LoggingConfig{Level: "info", Format: "text"},
	})
	if err != nil {
		t.Fatalf("observability: %v", err)
	}
	t.Cleanup(func() { _ = obs.Shutdown(context.Background()) })
	return obs, observability.NewBridge(obs)
}

// Covers: L5-2-9-11, L5-2-9-15
func TestIntegration_HarnessObs_enabled_span_tree(t *testing.T) {
	obs, bridge := newHarnessObs(t)

	ctxCfg := config.DefaultContextEngineConfig()
	ctxCfg.Harness.Enabled = true
	ctxCfg.Harness.Prefetch.Enabled = true
	ctxCfg.LongTerm.Enabled = false

	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM:        &mockctx.LLMGateway{Response: "obs ok"},
		Tools:      &mockctx.ToolRunner{},
		ToolsReg:   registry.NewBuiltinRegistry(),
		Permission: mockctx.AllowAllPermission{},
		Config:     ctxCfg,
		ObsBridge:  bridge,
	})

	session := types.NewSession("sess_obs_on", "cli", t.TempDir())
	ch := engine.Process(context.Background(), session, "trace harness")
	drainHarnessEvents(t, ch)

	mem := obs.MemoryExporter()
	if mem == nil {
		t.Fatal("memory exporter required")
	}
	spans := mem.Spans()
	counts := make(map[string]int)
	for _, s := range spans {
		counts[s.Name()]++
	}

	for _, name := range []string{
		telemetry.OpContextHarnessBootstrapRun,
		telemetry.OpContextSystemPromptBuild,
		telemetry.OpContextPEVRun,
	} {
		if counts[name] == 0 {
			t.Errorf("missing span %q", name)
		}
	}
	if counts[telemetry.OpContextHarnessBootstrapStage] == 0 {
		t.Fatal("expected bootstrap stage spans")
	}

	var runSpanID tracer.SpanID
	var runFound bool
	for _, s := range spans {
		if s.Name() == telemetry.OpContextHarnessBootstrapRun {
			runSpanID = s.SpanContext().SpanID
			runFound = true
		}
	}
	if !runFound {
		t.Fatal("bootstrap.run span not found")
	}

	stageParents := 0
	for _, s := range spans {
		if s.Name() != telemetry.OpContextHarnessBootstrapStage {
			continue
		}
		parent := s.Parent()
		if parent == nil {
			t.Logf("stage span missing parent, attrs=%v", s.Attributes())
			continue
		}
		if parent.SpanID == runSpanID {
			stageParents++
		}
	}
	if stageParents == 0 {
		t.Fatal("expected at least one bootstrap.stage span parented to bootstrap.run")
	}
}

// Covers: L5-2-9-11
func TestIntegration_HarnessObs_disabled_no_harness_spans(t *testing.T) {
	obs, bridge := newHarnessObs(t)

	ctxCfg := config.DefaultContextEngineConfig()
	ctxCfg.LongTerm.Enabled = false

	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM:        &mockctx.LLMGateway{Response: "legacy"},
		Tools:      &mockctx.ToolRunner{},
		ToolsReg:   registry.NewBuiltinRegistry(),
		Permission: mockctx.AllowAllPermission{},
		Config:     ctxCfg,
		ObsBridge:  bridge,
	})

	session := types.NewSession("sess_obs_off", "cli", t.TempDir())
	ch := engine.Process(context.Background(), session, "no harness")
	drainHarnessEvents(t, ch)

	for _, s := range obs.MemoryExporter().Spans() {
		name := s.Name()
		if strings.HasPrefix(name, "context.harness.") || name == telemetry.OpContextSystemPromptBuild {
			t.Fatalf("unexpected harness span when disabled: %s", name)
		}
	}
}

// Covers: L5-5-5-02
func TestIntegration_HarnessObs_coverage_hits_harness_operations(t *testing.T) {
	obs, bridge := newHarnessObs(t)

	ctxCfg := config.DefaultContextEngineConfig()
	ctxCfg.Harness.Enabled = true
	ctxCfg.LongTerm.Enabled = false

	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM:        &mockctx.LLMGateway{Response: "cov"},
		Tools:      &mockctx.ToolRunner{},
		ToolsReg:   registry.NewBuiltinRegistry(),
		Permission: mockctx.AllowAllPermission{},
		Config:     ctxCfg,
		ObsBridge:  bridge,
	})

	session := types.NewSession("sess_cov", "cli", t.TempDir())
	ch := engine.Process(context.Background(), session, "coverage")
	drainHarnessEvents(t, ch)

	report := obs.CoverageReport(true)
	for _, op := range []string{
		telemetry.OpContextHarnessBootstrapRun,
		telemetry.OpContextSystemPromptBuild,
	} {
		if report.Hits[op] == 0 {
			t.Errorf("expected coverage hit for %q, hits: %+v", op, report.Hits)
		}
	}
	t.Logf("coverage after harness process: hit=%d total=%d", report.OperationsHit, report.OperationsTotal)
}
