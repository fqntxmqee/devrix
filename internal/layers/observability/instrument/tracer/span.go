package tracer

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"
)

// SpanOption applies configuration to a Span
type SpanOption func(*SpanConfig)

// SpanConfig holds span configuration
type SpanConfig struct {
	StartTime  time.Time
	Attributes []Attribute
	SpanKind   SpanKind
}

// Attribute key-value pair
type Attribute struct {
	Key   string
	Value interface{}
}

// Span represents an OpenTelemetry-compatible span
type Span interface {
	End(opts ...SpanEndOption)
	SetStatus(code SpanStatusCode, description string)
	RecordError(err error, opts ...RecordErrorOption)
	SetAttributes(kv ...Attribute)
	AddEvent(name string, opts ...EventOption)
	SpanContext() SpanContext
	IsRecording() bool
}

// span is the internal implementation of Span
type span struct {
	name      string
	sc        SpanContext
	parent    *SpanContext
	kind      SpanKind
	startTime time.Time
	endTime   time.Time
	status    Status
	attrs     map[string]interface{}
	events    []Event
	exporter  SpanExporter
	mu        sync.RWMutex
	recording bool
}

// SpanEndOption applies configuration when ending a span
type SpanEndOption func(*SpanEndConfig)

// SpanEndConfig holds end span configuration
type SpanEndConfig struct {
	EndTime time.Time
}

// RecordErrorOption applies configuration when recording an error
type RecordErrorOption func(*RecordErrorConfig)

// RecordErrorConfig holds error recording configuration
type RecordErrorConfig struct {
	Timestamp  time.Time
	Attributes []Attribute
}

// Event represents a span event
type Event struct {
	Name       string
	Timestamp  time.Time
	Attributes []Attribute
}

// EventOption applies configuration when adding an event
type EventOption func(*EventConfig)

// EventConfig holds event configuration
type EventConfig struct {
	Timestamp  time.Time
	Attributes []Attribute
}

// Status represents span status
type Status struct {
	Code        SpanStatusCode
	Description string
}

// End ends the span
func (s *span) End(opts ...SpanEndOption) {
	cfg := &SpanEndConfig{EndTime: time.Now()}
	for _, opt := range opts {
		opt(cfg)
	}

	s.mu.Lock()
	if !s.recording {
		s.mu.Unlock()
		return
	}
	s.endTime = cfg.EndTime
	s.recording = false
	exporter := s.exporter
	s.mu.Unlock()

	if exporter != nil {
		_ = exporter.Export(context.Background(), s)
	}
}

// SetStatus sets the span status
func (s *span) SetStatus(code SpanStatusCode, description string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.status = Status{
		Code:        code,
		Description: description,
	}
}

// RecordError records an error in the span
func (s *span) RecordError(err error, opts ...RecordErrorOption) {
	if err == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := &RecordErrorConfig{Timestamp: time.Now()}
	for _, opt := range opts {
		opt(cfg)
	}

	s.events = append(s.events, Event{
		Name:      "exception",
		Timestamp: cfg.Timestamp,
		Attributes: []Attribute{
			{Key: "exception.type", Value: reflect.TypeOf(err).String()},
			{Key: "exception.message", Value: err.Error()},
		},
	})
	s.status = Status{
		Code:        StatusCodeError,
		Description: err.Error(),
	}
}

// SetAttributes sets multiple attributes
func (s *span) SetAttributes(kv ...Attribute) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, attr := range kv {
		s.attrs[attr.Key] = attr.Value
	}
}

// AddEvent adds an event to the span
func (s *span) AddEvent(name string, opts ...EventOption) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := &EventConfig{Timestamp: time.Now()}
	for _, opt := range opts {
		opt(cfg)
	}

	s.events = append(s.events, Event{
		Name:       name,
		Timestamp:  cfg.Timestamp,
		Attributes: cfg.Attributes,
	})
}

// SpanContext returns the span context
func (s *span) SpanContext() SpanContext {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sc
}

// IsRecording returns whether the span is recording
func (s *span) IsRecording() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.recording
}

// Duration returns the span duration
func (s *span) Duration() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.endTime.IsZero() {
		return time.Since(s.startTime)
	}
	return s.endTime.Sub(s.startTime)
}

// Attributes returns a copy of the attributes
func (s *span) Attributes() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]interface{}, len(s.attrs))
	for k, v := range s.attrs {
		result[k] = v
	}
	return result
}

// Events returns a copy of the events
func (s *span) Events() []Event {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Event, len(s.events))
	copy(result, s.events)
	return result
}

// Status returns the span status
func (s *span) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// Name returns the span name
func (s *span) Name() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.name
}

// Kind returns the span kind
func (s *span) Kind() SpanKind {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.kind
}

// StartTime returns the span start time
func (s *span) StartTime() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.startTime
}

// EndTime returns the span end time
func (s *span) EndTime() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.endTime
}

// Parent returns the parent span context
func (s *span) Parent() *SpanContext {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.parent
}

// SetParent sets the parent span context
func (s *span) SetParent(parent *SpanContext) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.parent = parent
}

// --- Option functions ---

// WithAttributes sets initial attributes
func WithAttributes(attrs ...Attribute) SpanOption {
	return func(cfg *SpanConfig) {
		cfg.Attributes = append(cfg.Attributes, attrs...)
	}
}

// WithEndTime sets the span end time
func WithEndTime(t time.Time) SpanEndOption {
	return func(cfg *SpanEndConfig) {
		cfg.EndTime = t
	}
}

// WithEventTimestamp sets the event timestamp
func WithEventTimestamp(t time.Time) EventOption {
	return func(cfg *EventConfig) {
		cfg.Timestamp = t
	}
}

// WithEventAttributes sets event attributes
func WithEventAttributes(attrs ...Attribute) EventOption {
	return func(cfg *EventConfig) {
		cfg.Attributes = append(cfg.Attributes, attrs...)
	}
}

// WithErrorTimestamp sets the error timestamp
func WithErrorTimestamp(t time.Time) RecordErrorOption {
	return func(cfg *RecordErrorConfig) {
		cfg.Timestamp = t
	}
}

// WithErrorAttributes sets error attributes
func WithErrorAttributes(attrs ...Attribute) RecordErrorOption {
	return func(cfg *RecordErrorConfig) {
		cfg.Attributes = append(cfg.Attributes, attrs...)
	}
}

// String returns a human-readable representation of the span
func (s *span) String() string {
	return fmt.Sprintf("Span(%s, traceID=%s, spanID=%s, duration=%v)",
		s.name, s.sc.TraceID.String(), s.sc.SpanID.String(), s.Duration())
}
