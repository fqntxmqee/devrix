package exporter

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/observability/settings"
	"github.com/devrix/devrix/internal/layers/observability/tracer"
)

func TestConsoleExporter(t *testing.T) {
	exporter := NewConsoleExporter()

	tp := tracer.NewTracerProvider(nil, nil)
	tr := tp.Tracer("test")
	ctx := context.Background()
	_, span := tr.Start(ctx, "test.span")
	span.End()

	err := exporter.Export(span)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConsoleExporterNilSpan(t *testing.T) {
	exporter := NewConsoleExporter()

	err := exporter.Export(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNullExporter(t *testing.T) {
	exporter := NewNullExporter()

	err := exporter.Export(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = exporter.ExportBatch(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = exporter.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOTLPExporterCreation(t *testing.T) {
	exporter := NewOTLPExporter("http://localhost:4318/v1/traces", "devrix", 0)
	if exporter == nil {
		t.Fatal("expected non-nil exporter")
	}
}

func TestResolveOTLPEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     string
	}{
		{name: "grpc shorthand", endpoint: "localhost:4317", want: "http://localhost:4318/v1/traces"},
		{name: "http host", endpoint: "http://localhost:4318", want: "http://localhost:4318/v1/traces"},
		{name: "full path", endpoint: "http://localhost:4318/v1/traces", want: "http://localhost:4318/v1/traces"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveOTLPEndpoint(settingsOTLP(tt.endpoint))
			if got != tt.want {
				t.Fatalf("ResolveOTLPEndpoint() = %q, want %q", got, tt.want)
			}
		})
	}
}

func settingsOTLP(endpoint string) settings.OTLPConfig {
	return settings.OTLPConfig{Endpoint: endpoint}
}
