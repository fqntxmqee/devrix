package export

import (
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/observability/configure/settings"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
)

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
	case "memory":
		return NewMemoryExporter()
	default:
		return NewConsoleExporter()
	}
}
