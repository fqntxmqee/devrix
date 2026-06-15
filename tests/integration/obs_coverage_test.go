//go:build integration && d5

package integration

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
)

func TestCoverageReport_should_include_health_summary(t *testing.T) {
	cfg := observability.DefaultConfig()
	obs, err := observability.New(cfg)
	if err != nil {
		t.Fatalf("new observability: %v", err)
	}

	tr := obs.Tracer()
	_, span := tr.Start(context.Background(), telemetry.OpD2_S2_Context_Process)
	span.End()

	health := obs.HealthCheck()
	coverageRaw, ok := health["coverage"].(map[string]interface{})
	if !ok {
		t.Fatalf("health missing coverage: %+v", health)
	}
	if coverageRaw["operations_hit"].(int) < 1 {
		t.Fatalf("expected at least one hit, got %+v", coverageRaw)
	}
}

func TestCoverageReport_should_list_zero_hit_operations(t *testing.T) {
	cfg := observability.DefaultConfig()
	cfg.Tracing.Sampling.Type = "always_off"
	obs, err := observability.New(cfg)
	if err != nil {
		t.Fatalf("new observability: %v", err)
	}

	tr := obs.Tracer()
	_, span := tr.Start(context.Background(), telemetry.OpD2_S2_Context_Process)
	span.End()

	report := obs.CoverageReport(false)
	if report.OperationsHit < 1 {
		t.Fatalf("expected hit under always_off sampling, report: %+v", report)
	}
	if len(report.OperationsZeroHit) == 0 {
		t.Fatal("expected zero_hit entries for untouched operations")
	}
}
