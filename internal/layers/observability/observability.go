package observability

import (
	"context"
	"fmt"
	"sync"

	"github.com/devrix/devrix/internal/layers/observability/coverage"
	"github.com/devrix/devrix/internal/layers/observability/exporter"
	"github.com/devrix/devrix/internal/layers/observability/logger"
	"github.com/devrix/devrix/internal/layers/observability/metrics"
	"github.com/devrix/devrix/internal/layers/observability/tracer"
)

// Observability is the main facade for the observability layer
type Observability struct {
	config *Config

	tracerProvider *tracer.TracerProvider
	tracerInst     *tracer.Tracer
	meterProvider  *metrics.MeterProvider
	meterInst      *metrics.Meter
	log            *logger.StructuredLogger

	mu     sync.RWMutex
	status ComponentStatus
}

// ComponentStatus tracks the status of each component
type ComponentStatus struct {
	Tracer  string `json:"tracer"`
	Metrics string `json:"metrics"`
	Logging string `json:"logging"`
}

// New creates a new Observability instance
func New(cfg *Config) (*Observability, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	obs := &Observability{
		config: cfg,
	}

	coverage.InitGlobal(coverage.AllOperations())

	// Initialize tracer
	if cfg.IsTracingEnabled() {
		spanExporter := exporter.NewTracingExporter(cfg.Tracing)
		obs.tracerProvider = tracer.NewTracerProvider(&cfg.Tracing, spanExporter)
		obs.tracerInst = obs.tracerProvider.Tracer(cfg.Tracing.ServiceName)
		obs.status.Tracer = "healthy"
	} else {
		obs.status.Tracer = "disabled"
	}

	// Initialize meter
	if cfg.IsMetricsEnabled() {
		obs.meterProvider = metrics.NewMeterProvider(&cfg.Metrics)
		obs.meterInst = obs.meterProvider.Meter("devrix")
		obs.status.Metrics = "healthy"
	} else {
		obs.status.Metrics = "disabled"
	}

	// Initialize logger
	if cfg.IsLoggingEnabled() {
		obs.log = logger.NewStructuredLogger(&logger.LoggerConfig{
			Level:  cfg.Logging.Level,
			Format: cfg.Logging.Format,
			Sampling: logger.SamplingConfig{
				Enabled:           cfg.Logging.Sampling.Enabled,
				MaxEntriesPerSpan: cfg.Logging.Sampling.MaxEntriesPerSpan,
			},
			Redactor: logger.RedactorConfig{
				Enabled:  cfg.Logging.Redactor.Enabled,
				Patterns: cfg.Logging.Redactor.Patterns,
			},
		})
		obs.status.Logging = "healthy"
	} else {
		obs.status.Logging = "disabled"
	}

	return obs, nil
}

// Tracer returns the tracer instance
func (o *Observability) Tracer() *tracer.Tracer {
	return o.tracerInst
}

// Meter returns the meter instance
func (o *Observability) Meter() *metrics.Meter {
	return o.meterInst
}

// Logger returns the logger instance
func (o *Observability) Logger() *logger.StructuredLogger {
	return o.log
}

// MemoryExporter returns the span collector when exporter type is "memory".
func (o *Observability) MemoryExporter() *exporter.MemoryExporter {
	if o.tracerProvider == nil {
		return nil
	}
	me, ok := o.tracerProvider.Exporter().(*exporter.MemoryExporter)
	if !ok {
		return nil
	}
	return me
}

// Status returns the current status
func (o *Observability) Status() ComponentStatus {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.status
}

// Shutdown gracefully shuts down the observability layer
func (o *Observability) Shutdown(ctx context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	var errs []error

	// Shutdown tracer provider
	if o.tracerProvider != nil {
		if err := o.tracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("tracer shutdown: %w", err))
		}
	}

	if o.log != nil {
		if err := o.log.Close(); err != nil {
			errs = append(errs, fmt.Errorf("logger shutdown: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	return nil
}

// HealthCheck performs a health check
func (o *Observability) HealthCheck() map[string]interface{} {
	status := o.Status()

	result := map[string]interface{}{
		"status": "healthy",
		"components": map[string]interface{}{
			"tracer": map[string]interface{}{
				"status": status.Tracer,
			},
			"metrics": map[string]interface{}{
				"status": status.Metrics,
			},
			"logging": map[string]interface{}{
				"status": status.Logging,
			},
		},
	}

	// Check if any component is unhealthy
	if status.Tracer == "unhealthy" || status.Metrics == "unhealthy" || status.Logging == "unhealthy" {
		result["status"] = "degraded"
	}

	if counter := coverage.Global(); counter != nil {
		report := counter.Report(coverage.AllOperations(), false)
		result["coverage"] = map[string]interface{}{
			"operations_total": report.OperationsTotal,
			"operations_hit":   report.OperationsHit,
			"coverage_ratio":   report.CoverageRatio,
			"zero_hit_count":   len(report.OperationsZeroHit),
		}
	}

	return result
}

// CoverageReport returns the full operation coverage reconciliation report.
func (o *Observability) CoverageReport(includeHits bool) coverage.Report {
	if counter := coverage.Global(); counter != nil {
		return counter.Report(coverage.AllOperations(), includeHits)
	}
	return coverage.Report{}
}

// NewNoOp returns a no-op observability instance (when observability is disabled)
func NewNoOp() *Observability {
	return &Observability{
		config: &Config{Enabled: false},
		status: ComponentStatus{
			Tracer:  "disabled",
			Metrics: "disabled",
			Logging: "disabled",
		},
	}
}

// IsEnabled returns whether observability is enabled
func (o *Observability) IsEnabled() bool {
	return o.config.Enabled
}
