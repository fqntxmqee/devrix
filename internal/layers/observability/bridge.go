package observability

import (
	"github.com/devrix/devrix/internal/layers/observability/logger"
	"github.com/devrix/devrix/internal/layers/observability/metrics"
	"github.com/devrix/devrix/internal/layers/observability/tracer"
)

// Bridge provides convenience methods for integrating observability into other layers
type Bridge struct {
	tracer *tracer.Tracer
	meter  *metrics.Meter
	logger *logger.StructuredLogger
}

// NewBridge creates a new observability bridge
func NewBridge(obs *Observability) *Bridge {
	if obs == nil {
		return nil
	}
	return &Bridge{
		tracer: obs.Tracer(),
		meter:  obs.Meter(),
		logger: obs.Logger(),
	}
}

// Tracer returns the tracer
func (b *Bridge) Tracer() *tracer.Tracer {
	return b.tracer
}

// Meter returns the meter
func (b *Bridge) Meter() *metrics.Meter {
	return b.meter
}

// Logger returns the logger
func (b *Bridge) Logger() *logger.StructuredLogger {
	return b.logger
}

// IsEnabled returns whether observability is enabled
func (b *Bridge) IsEnabled() bool {
	return b != nil && b.tracer != nil
}

// LLMBridge provides LLM-specific observability helpers
type LLMBridge struct {
	bridge *Bridge
}

// NewLLMBridge creates a new LLM bridge
func NewLLMBridge(obs *Observability) *LLMBridge {
	return &LLMBridge{
		bridge: NewBridge(obs),
	}
}

// InitMetrics initializes LLM metrics
func (b *LLMBridge) InitMetrics(provider, model string) (*LLMMetrics, error) {
	if b.bridge == nil || b.bridge.meter == nil {
		return nil, nil
	}

	labels := metrics.LabelMap{
		"provider": provider,
		"model":   model,
	}

	tokensInput, err := b.bridge.meter.Int64Counter("tokens",
		metrics.WithLabels(labels))
	if err != nil {
		return nil, err
	}

	latency, err := b.bridge.meter.Float64Histogram("latency",
		metrics.WithHistogramLabels(labels),
		metrics.WithBounds(metrics.LLMHistogramBounds()))
	if err != nil {
		return nil, err
	}

	errors, err := b.bridge.meter.Int64Counter("errors",
		metrics.WithLabels(labels))
	if err != nil {
		return nil, err
	}

	return &LLMMetrics{
		TokensInput: tokensInput,
		Latency:    latency,
		Errors:     errors,
	}, nil
}

// LLMMetrics holds LLM-related metrics
type LLMMetrics struct {
	TokensInput metrics.Counter
	Latency     metrics.Histogram
	Errors      metrics.Counter
}

// ToolBridge provides tool-specific observability helpers
type ToolBridge struct {
	bridge *Bridge
}

// NewToolBridge creates a new tool bridge
func NewToolBridge(obs *Observability) *ToolBridge {
	return &ToolBridge{
		bridge: NewBridge(obs),
	}
}

// InitMetrics initializes tool metrics
func (b *ToolBridge) InitMetrics(toolName, riskLevel string) (*ToolMetrics, error) {
	if b.bridge == nil || b.bridge.meter == nil {
		return nil, nil
	}

	labels := metrics.LabelMap{
		"tool":       toolName,
		"risk_level": riskLevel,
	}

	calls, err := b.bridge.meter.Int64Counter("calls",
		metrics.WithLabels(labels))
	if err != nil {
		return nil, err
	}

	errors, err := b.bridge.meter.Int64Counter("tool_errors",
		metrics.WithLabels(labels))
	if err != nil {
		return nil, err
	}

	return &ToolMetrics{
		Calls:  calls,
		Errors: errors,
	}, nil
}

// ToolMetrics holds tool-related metrics
type ToolMetrics struct {
	Calls  metrics.Counter
	Errors metrics.Counter
}

// SessionBridge provides session-specific observability helpers
type SessionBridge struct {
	bridge *Bridge
}

// NewSessionBridge creates a new session bridge
func NewSessionBridge(obs *Observability) *SessionBridge {
	return &SessionBridge{
		bridge: NewBridge(obs),
	}
}

// ActiveSessions returns a counter for active sessions
func (b *SessionBridge) ActiveSessions(adapter string) (metrics.Counter, error) {
	if b.bridge == nil || b.bridge.meter == nil {
		return nil, nil
	}

	return b.bridge.meter.Int64UpDownCounter("active_sessions",
		metrics.WithLabels(metrics.LabelMap{"adapter": adapter}))
}
