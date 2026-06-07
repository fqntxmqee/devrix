package exporter

import (
	"context"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/observability/settings"
	"github.com/devrix/devrix/internal/layers/observability/tracer"
)

// consoleExporterAdapter adapts ConsoleExporter to SpanExporter.
type consoleExporterAdapter struct {
	inner *ConsoleExporter
}

func (a *consoleExporterAdapter) Export(_ context.Context, span tracer.ReadableSpan) error {
	return a.inner.Export(span)
}

func (a *consoleExporterAdapter) Shutdown(_ context.Context) error {
	return a.inner.Shutdown()
}

// NewConsoleExporterSpanExporter returns a console span exporter.
func NewConsoleExporterSpanExporter() tracer.SpanExporter {
	return &consoleExporterAdapter{inner: NewConsoleExporter()}
}

// ResolveOTLPEndpoint normalizes OTLP HTTP endpoint configuration.
func ResolveOTLPEndpoint(cfg settings.OTLPConfig) string {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		return "http://localhost:4318/v1/traces"
	}

	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		if !strings.Contains(endpoint, "/v1/traces") {
			return strings.TrimSuffix(endpoint, "/") + "/v1/traces"
		}
		return endpoint
	}

	if strings.HasSuffix(endpoint, ":4317") {
		endpoint = strings.TrimSuffix(endpoint, ":4317") + ":4318"
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "http://" + endpoint
	}
	if !strings.Contains(endpoint, "/v1/traces") {
		return strings.TrimSuffix(endpoint, "/") + "/v1/traces"
	}
	return endpoint
}

// NewTracingExporter creates a span exporter from tracing config.
func NewTracingExporter(cfg settings.TracingConfig) tracer.SpanExporter {
	switch cfg.Exporter {
	case "otlp":
		timeout := cfg.OTLP.Timeout
		if timeout == 0 {
			timeout = 5 * time.Second
		}
		return NewOTLPExporter(ResolveOTLPEndpoint(cfg.OTLP), cfg.ServiceName, timeout)
	case "null":
		return NewNullExporter()
	default:
		return NewConsoleExporterSpanExporter()
	}
}
