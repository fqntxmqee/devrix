// Package tracer is a backward-compatibility bridge.
//
// Deprecated: use github.com/devrix/devrix/internal/layers/observability/instrument/tracer instead.
// This bridge will be removed in v2.1.
package tracer

import "github.com/devrix/devrix/internal/layers/observability/instrument/tracer"

// Types — core

type (
	TraceID       = tracer.TraceID
	SpanID        = tracer.SpanID
	TraceFlags    = tracer.TraceFlags
	TraceState    = tracer.TraceState
	SpanContext   = tracer.SpanContext
	SpanKind      = tracer.SpanKind
	SpanStatusCode = tracer.SpanStatusCode
)

// Constants — trace flags

const FlagSampled = tracer.FlagSampled

// Constants — status codes

const (
	StatusCodeUnset = tracer.StatusCodeUnset
	StatusCodeOk    = tracer.StatusCodeOk
	StatusCodeError = tracer.StatusCodeError
)

// Constants — span kinds

const (
	SpanKindInternal = tracer.SpanKindInternal
	SpanKindServer   = tracer.SpanKindServer
	SpanKindClient   = tracer.SpanKindClient
	SpanKindProducer = tracer.SpanKindProducer
	SpanKindConsumer = tracer.SpanKindConsumer
)

// Types — span

type (
	Span            = tracer.Span
	SpanOption      = tracer.SpanOption
	SpanConfig      = tracer.SpanConfig
	SpanEndOption   = tracer.SpanEndOption
	SpanEndConfig   = tracer.SpanEndConfig
	SpanStartOption = tracer.SpanStartOption
	SpanStartConfig = tracer.SpanStartConfig
	Attribute       = tracer.Attribute
	Event           = tracer.Event
	EventOption     = tracer.EventOption
	EventConfig     = tracer.EventConfig
	Status          = tracer.Status
	RecordErrorOption = tracer.RecordErrorOption
	RecordErrorConfig = tracer.RecordErrorConfig
)

// Types — tracer

type (
	TracerProvider = tracer.TracerProvider
	Tracer         = tracer.Tracer
	TracerOption   = tracer.TracerOption
)

// Types — export

type (
	ReadableSpan = tracer.ReadableSpan
	SpanExporter = tracer.SpanExporter
)

// Types — sampling

type (
	Sampler              = tracer.Sampler
	AlwaysOnSampler      = tracer.AlwaysOnSampler
	AlwaysOffSampler     = tracer.AlwaysOffSampler
	TraceIdRatioSampler  = tracer.TraceIdRatioSampler
)

// Types — propagation

type (
	TextMapCarrier    = tracer.TextMapCarrier
	Propagator        = tracer.Propagator
	MapCarrier        = tracer.MapCarrier
	HTTPHeaderCarrier = tracer.HTTPHeaderCarrier
)

// Types — baggage

type (
	BaggageContextKey = tracer.BaggageContextKey
	BaggageItem       = tracer.BaggageItem
	BaggageManager    = tracer.BaggageManager
)

// Constants

const BaggageHeader = tracer.BaggageHeader

// Functions — tracer

var (
	NewTracerProvider = tracer.NewTracerProvider
	GenerateTraceID   = tracer.GenerateTraceID
	GenerateSpanID    = tracer.GenerateSpanID
)

// Functions — span options

var (
	WithAttributes       = tracer.WithAttributes
	WithEndTime          = tracer.WithEndTime
	WithEventTimestamp   = tracer.WithEventTimestamp
	WithEventAttributes  = tracer.WithEventAttributes
	WithErrorTimestamp   = tracer.WithErrorTimestamp
	WithErrorAttributes  = tracer.WithErrorAttributes
	WithParent           = tracer.WithParent
	WithTraceID          = tracer.WithTraceID
	WithStartTime        = tracer.WithStartTime
	WithSpanKind         = tracer.WithSpanKind
	WithSpanAttributes   = tracer.WithSpanAttributes
)

// Functions — context

var (
	SpanContextFromContext = tracer.SpanContextFromContext
	ContextWithSpan        = tracer.ContextWithSpan
	SpanFromContext        = tracer.SpanFromContext
	ContextWithSpanValue   = tracer.ContextWithSpanValue
	Detach                 = tracer.Detach
)

// Functions — propagation

var (
	NewPropagator        = tracer.NewPropagator
	NewHTTPHeaderCarrier = tracer.NewHTTPHeaderCarrier
	PropagationEnvVars   = tracer.PropagationEnvVars
)

// Functions — types

var (
	ParseTraceID = tracer.ParseTraceID
	ParseSpanID  = tracer.ParseSpanID
)

// Functions — sampling

var NewSampler = tracer.NewSampler

// Functions — baggage

var (
	NewBaggageManager    = tracer.NewBaggageManager
	DefaultBaggageManager = tracer.DefaultBaggageManager
)
