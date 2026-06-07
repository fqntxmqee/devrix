package metrics

import (
	"fmt"

	"github.com/devrix/devrix/internal/layers/observability/settings"
)

// MeterProvider creates Meter instances
type MeterProvider struct {
	config   *settings.MetricsConfig
	registry *Registry
}

// NewMeterProvider creates a new MeterProvider
func NewMeterProvider(cfg *settings.MetricsConfig) *MeterProvider {
	return &MeterProvider{
		config: cfg,
		registry: NewRegistry(
			cfg.Labels.Allowlist,
			cfg.Labels.Blocklist,
		),
	}
}

// Meter creates metric instruments
type Meter struct {
	provider *MeterProvider
	name     string
}

// Meter creates a new Meter
func (mp *MeterProvider) Meter(name string) *Meter {
	return &Meter{
		provider: mp,
		name:     name,
	}
}

// Int64Counter creates a new counter
func (m *Meter) Int64Counter(name string, opts ...CounterOption) (Counter, error) {
	cfg := &CounterConfig{Labels: make(LabelMap)}
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}

	counter := NewCounter(fullMetricName(m.name, name), cfg.Labels)
	
	if err := m.provider.registry.RegisterCounter(
		fullMetricName(m.name, name),
		cfg.Labels,
		counter,
	); err != nil {
		return nil, err
	}
	
	return counter, nil
}

// Float64Histogram creates a new histogram
func (m *Meter) Float64Histogram(name string, opts ...HistogramOption) (Histogram, error) {
	cfg := &HistogramConfig{
		Labels: make(LabelMap),
		Bounds: DefaultHistogramBounds(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}

	histo := NewHistogram(fullMetricName(m.name, name), cfg.Labels, cfg.Bounds)
	
	if err := m.provider.registry.RegisterHistogram(
		fullMetricName(m.name, name),
		cfg.Labels,
		histo,
	); err != nil {
		return nil, err
	}
	
	return histo, nil
}

// Int64UpDownCounter creates a new gauge (up-down counter)
func (m *Meter) Int64UpDownCounter(name string, opts ...CounterOption) (Counter, error) {
	// For now, use counter as gauge
	cfg := &CounterConfig{Labels: make(LabelMap)}
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}

	counter := NewCounter(fullMetricName(m.name, name), cfg.Labels)

	if err := m.provider.registry.RegisterCounter(
		fullMetricName(m.name, name),
		cfg.Labels,
		counter,
	); err != nil {
		return nil, err
	}

	return counter, nil
}

// Registry returns the underlying registry
func (m *Meter) Registry() *Registry {
	return m.provider.registry
}

// fullMetricName combines meter name and instrument name
func fullMetricName(meter, instrument string) string {
	if meter == "" {
		return instrument
	}
	return meter + "_" + instrument
}

// CounterConfig holds counter configuration
type CounterConfig struct {
	Labels LabelMap
}

// CounterOption applies configuration to a counter
type CounterOption func(*CounterConfig)

// WithLabels sets the labels for a counter
func WithLabels(labels LabelMap) CounterOption {
	return func(cfg *CounterConfig) {
		cfg.Labels = labels
	}
}

// HistogramConfig holds histogram configuration
type HistogramConfig struct {
	Labels LabelMap
	Bounds []float64
}

// HistogramOption applies configuration to a histogram
type HistogramOption func(*HistogramConfig)

// WithHistogramLabels sets the labels for a histogram
func WithHistogramLabels(labels LabelMap) HistogramOption {
	return func(cfg *HistogramConfig) {
		cfg.Labels = labels
	}
}

// WithBounds sets the bucket bounds for a histogram
func WithBounds(bounds []float64) HistogramOption {
	return func(cfg *HistogramConfig) {
		cfg.Bounds = bounds
	}
}

// Errors

// ErrMetricAlreadyExists is returned when a metric is already registered
var ErrMetricAlreadyExists = fmt.Errorf("metric already exists")

// ErrInvalidLabel is returned when a label is invalid
var ErrInvalidLabel = fmt.Errorf("invalid label")
