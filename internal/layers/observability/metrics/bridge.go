// Package metrics is a backward-compatibility bridge.
//
// Deprecated: use github.com/devrix/devrix/internal/layers/observability/instrument/metrics instead.
// This bridge will be removed in v2.1.
package metrics

import "github.com/devrix/devrix/internal/layers/observability/instrument/metrics"

// Types — core

type (
	Counter  = metrics.Counter
	Gauge    = metrics.Gauge
	Histogram = metrics.Histogram
)

// Types — meter

type (
	MeterProvider    = metrics.MeterProvider
	Meter            = metrics.Meter
	CounterConfig    = metrics.CounterConfig
	CounterOption    = metrics.CounterOption
	HistogramConfig  = metrics.HistogramConfig
	HistogramOption  = metrics.HistogramOption
)

// Types — registry

type (
	LabelMap   = metrics.LabelMap
	Metric     = metrics.Metric
	MetricType = metrics.MetricType
	Registry   = metrics.Registry
)

// Types — prometheus

type PrometheusExporter = metrics.PrometheusExporter

// Types — multi-agent

type (
	MultiAgentMetrics = metrics.MultiAgentMetrics
	PolicyCounter     = metrics.PolicyCounter
)

// Errors

var (
	ErrMetricAlreadyExists = metrics.ErrMetricAlreadyExists
	ErrInvalidLabel        = metrics.ErrInvalidLabel
)

// Functions — counter

var NewCounter = metrics.NewCounter

// Functions — gauge

var NewGauge = metrics.NewGauge

// Functions — histogram

var (
	NewHistogram           = metrics.NewHistogram
	DefaultHistogramBounds = metrics.DefaultHistogramBounds
	CompressionRatioBounds = metrics.CompressionRatioBounds
	LLMHistogramBounds     = metrics.LLMHistogramBounds
)

// Functions — meter

var (
	NewMeterProvider       = metrics.NewMeterProvider
	WithLabels             = metrics.WithLabels
	WithHistogramLabels    = metrics.WithHistogramLabels
	WithBounds             = metrics.WithBounds
)

// Functions — registry

var NewRegistry = metrics.NewRegistry

// Functions — prometheus

var NewPrometheusExporter = metrics.NewPrometheusExporter

// Functions — multi-agent

var RegisterMultiAgentMetrics = metrics.RegisterMultiAgentMetrics
