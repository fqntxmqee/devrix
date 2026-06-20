package observability

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/devrix/devrix/internal/layers/observability/diagnose/coverage"
	"github.com/devrix/devrix/internal/layers/observability/export"
	"github.com/devrix/devrix/internal/layers/observability/instrument/logger"
	"github.com/devrix/devrix/internal/layers/observability/instrument/metrics"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
)

// Observability is the main facade for the observability layer
type Observability struct {
	config *Config

	tracerProvider *tracer.TracerProvider
	tracerInst     *tracer.Tracer
	meterProvider  *metrics.MeterProvider
	meterInst      *metrics.Meter
	log            *logger.StructuredLogger
	coverageReporter *coverage.Reporter

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
		spanExporter := export.NewTracingExporter(cfg.Tracing)
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

	// Start coverage reporter
	if cfg.Coverage.Enabled {
		persistence, err := coverage.NewPersistence(cfg.Coverage.Dir)
		if err != nil {
			return nil, fmt.Errorf("create coverage persistence: %w", err)
		}
		obs.coverageReporter = coverage.NewReporter(
			persistence,
			coverage.Global(),
			coverage.AllOperations(),
			cfg.Coverage.Interval,
		)
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
func (o *Observability) MemoryExporter() *export.MemoryExporter {
	if o.tracerProvider == nil {
		return nil
	}
	me, ok := o.tracerProvider.Exporter().(*export.MemoryExporter)
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

	// Stop coverage reporter
	if o.coverageReporter != nil {
		o.coverageReporter.Stop()
	}

	if len(errs) > 0 {
		// DM-20260620-003 (PR-C M3): wrap each shutdown error individually
		// via errors.Join so callers can errors.Is / errors.As against the
		// underlying typed sentinels. The legacy single string format
		// "shutdown errors: [err1 err2]" is preserved via the joined message
		// so log scrapers keep matching the existing pattern.
		combined := make([]error, 0, len(errs))
		for _, e := range errs {
			if e != nil {
				combined = append(combined, e)
			}
		}
		if len(combined) == 0 {
			return nil
		}
		joined := errors.Join(combined...)
		return fmt.Errorf("shutdown errors: %w", joined)
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

// CoverageReporter returns the coverage reporter
func (o *Observability) CoverageReporter() *coverage.Reporter {
	return o.coverageReporter
}

// StartCoverageReporter starts the coverage reporting background job
func (o *Observability) StartCoverageReporter(ctx context.Context) {
	if o.coverageReporter != nil {
		o.coverageReporter.Start(ctx)
	}
}

// GenerateCoverageReport generates and persists a coverage report immediately
func (o *Observability) GenerateCoverageReport() (*coverage.DailyReport, error) {
	if o.coverageReporter != nil {
		return o.coverageReporter.GenerateNow()
	}
	return nil, fmt.Errorf("coverage reporter not enabled")
}

// InstallSlogBridge wires the trace-context-aware slog handler via instrument/logger.
// Facade entry point — implementation lives in `instrument/logger` so the handler
// stack is colocated with the rest of the slog machinery (context handler, redactor, etc.).
// Idempotent at process startup; safe to call once before any slog.Default() writes.
func InstallSlogBridge() {
	logger.InstallSlogBridge()
}
