package tracer_test

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/observability/diagnose/coverage"
	"github.com/devrix/devrix/internal/layers/observability/configure/settings"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
)

func TestTracer_should_record_coverage_when_sampling_off(t *testing.T) {
	coverage.InitGlobal(coverage.AllOperations())
	counter := coverage.Global()
	counter.Reset()

	cfg := &settings.TracingConfig{
		Sampling: settings.SamplingConfig{Type: "always_off", Rate: 0},
	}
	tp := tracer.NewTracerProvider(cfg, nil)
	tr := tp.Tracer("test")

	_, span := tr.Start(context.Background(), telemetry.OpD3_S3_LLM_Stream)
	span.End()

	report := counter.Report(coverage.AllOperations(), true)
	if report.Hits[telemetry.OpD3_S3_LLM_Stream] != 1 {
		t.Fatalf("hits: %+v", report.Hits)
	}
}
