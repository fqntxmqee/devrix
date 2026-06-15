//go:build integration && d2

package integration

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/configure/settings"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
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
		Metrics: settings.MetricsConfig{Enabled: false, Exporter: "null"},
		Logging: observability.LoggingConfig{Level: "info", Format: "text"},
	})
	if err != nil {
		t.Fatalf("observability: %v", err)
	}
	t.Cleanup(func() { _ = obs.Shutdown(context.Background()) })
	return obs, observability.NewBridge(obs)
}

// T: D2-S9-A01-T11, D2-S9-A01-T15
func TestIntegration_HarnessObs_enabled_span_tree(t *testing.T) {
	obs, bridge := newHarnessObs(t)

	ctxCfg := config.DefaultContextEngineConfig()
	ctxCfg.Harness.Enabled = true
	ctxCfg.Harness.Prefetch.Enabled = true
	ctxCfg.LongTerm.Enabled = false

	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM:        &mockctx.LLMGateway{Response: "obs ok"},
		Tools:      &mockctx.ToolRunner{},
		ToolsReg:   mustBuiltinRegistry(t),
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
		telemetry.OpD2_S5_Context_Harness_Bootstrap_Run,
		telemetry.OpD2_S5_Context_Harness_SystemPrompt_Build,
	} {
		if counts[name] == 0 {
			t.Errorf("missing span %q", name)
		}
	}
	if counts[telemetry.OpD2_S5_Context_Harness_Bootstrap_Stage] == 0 {
		t.Fatal("expected bootstrap stage spans")
	}

	var runSpanID tracer.SpanID
	var runFound bool
	for _, s := range spans {
		if s.Name() == telemetry.OpD2_S5_Context_Harness_Bootstrap_Run {
			runSpanID = s.SpanContext().SpanID
			runFound = true
		}
	}
	if !runFound {
		t.Fatal("bootstrap.run span not found")
	}

	stageParents := 0
	for _, s := range spans {
		if s.Name() != telemetry.OpD2_S5_Context_Harness_Bootstrap_Stage {
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

// T: D2-S9-A01-T11
func TestIntegration_HarnessObs_disabled_no_harness_spans(t *testing.T) {
	obs, bridge := newHarnessObs(t)

	ctxCfg := config.DefaultContextEngineConfig()
	ctxCfg.LongTerm.Enabled = false

	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM:        &mockctx.LLMGateway{Response: "legacy"},
		Tools:      &mockctx.ToolRunner{},
		ToolsReg:   mustBuiltinRegistry(t),
		Permission: mockctx.AllowAllPermission{},
		Config:     ctxCfg,
		ObsBridge:  bridge,
	})

	session := types.NewSession("sess_obs_off", "cli", t.TempDir())
	ch := engine.Process(context.Background(), session, "no harness")
	drainHarnessEvents(t, ch)

	for _, s := range obs.MemoryExporter().Spans() {
		name := s.Name()
		switch name {
		case telemetry.OpD2_S5_Context_Harness_Bootstrap_Run,
			telemetry.OpD2_S5_Context_Harness_Bootstrap_Stage,
			telemetry.OpD2_S5_Context_Harness_ToolPool,
			telemetry.OpD2_S5_Context_Harness_Preflight,
			telemetry.OpD2_S5_Context_Harness_Route:
			t.Fatalf("unexpected harness span when disabled: %s", name)
		}
	}
}

// T: D5-S5-A01-T02
func TestIntegration_HarnessObs_coverage_hits_harness_operations(t *testing.T) {
	obs, bridge := newHarnessObs(t)

	ctxCfg := config.DefaultContextEngineConfig()
	ctxCfg.Harness.Enabled = true
	ctxCfg.LongTerm.Enabled = false

	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM:        &mockctx.LLMGateway{Response: "cov"},
		Tools:      &mockctx.ToolRunner{},
		ToolsReg:   mustBuiltinRegistry(t),
		Permission: mockctx.AllowAllPermission{},
		Config:     ctxCfg,
		ObsBridge:  bridge,
	})

	session := types.NewSession("sess_cov", "cli", t.TempDir())
	ch := engine.Process(context.Background(), session, "coverage")
	drainHarnessEvents(t, ch)

	report := obs.CoverageReport(true)
	for _, op := range []string{
		telemetry.OpD2_S5_Context_Harness_Bootstrap_Run,
		telemetry.OpD2_S5_Context_Harness_SystemPrompt_Build,
	} {
		if report.Hits[op] == 0 {
			t.Errorf("expected coverage hit for %q, hits: %+v", op, report.Hits)
		}
	}
	t.Logf("coverage after harness process: hit=%d total=%d", report.OperationsHit, report.OperationsTotal)
}
