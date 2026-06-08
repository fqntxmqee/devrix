package tracer

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/observability/coverage"
	"github.com/devrix/devrix/internal/layers/observability/settings"
)

// TracerProvider creates Tracer instances
type TracerProvider struct {
	config     *settings.TracingConfig
	sampler    Sampler
	exporter   SpanExporter
	shutdownMu sync.RWMutex
	shutdown   bool
	tracersMu  sync.Mutex
	tracers    []*Tracer
}

// NewTracerProvider creates a new TracerProvider
func NewTracerProvider(cfg *settings.TracingConfig, exporter SpanExporter) *TracerProvider {
	if cfg == nil {
		cfg = &settings.TracingConfig{
			Sampling: settings.SamplingConfig{Type: "always_on", Rate: 1.0},
		}
	}
	return &TracerProvider{
		config:   cfg,
		sampler:  NewSampler(&cfg.Sampling),
		exporter: exporter,
	}
}

// Tracer creates a new Tracer
func (tp *TracerProvider) Tracer(name string) *Tracer {
	t := &Tracer{
		name:          name,
		provider:      tp,
		activeSpansMu: sync.RWMutex{},
		activeSpans:   make(map[SpanID]*span),
	}
	tp.tracersMu.Lock()
	tp.tracers = append(tp.tracers, t)
	tp.tracersMu.Unlock()
	return t
}

// Shutdown shuts down the TracerProvider
func (tp *TracerProvider) Shutdown(ctx context.Context) error {
	tp.shutdownMu.Lock()
	defer tp.shutdownMu.Unlock()

	if tp.shutdown {
		return nil
	}

	tp.shutdown = true

	tp.tracersMu.Lock()
	tracers := append([]*Tracer(nil), tp.tracers...)
	tp.tracersMu.Unlock()

	for _, tr := range tracers {
		tr.activeSpansMu.Lock()
		for _, s := range tr.activeSpans {
			s.End()
		}
		tr.activeSpans = make(map[SpanID]*span)
		tr.activeSpansMu.Unlock()
	}

	if tp.exporter != nil {
		return tp.exporter.Shutdown(ctx)
	}
	return nil
}

// isShutdown returns whether the provider is shut down
func (tp *TracerProvider) isShutdown() bool {
	tp.shutdownMu.RLock()
	defer tp.shutdownMu.RUnlock()
	return tp.shutdown
}

// Tracer creates spans
type Tracer struct {
	name      string
	provider  *TracerProvider
	activeSpansMu sync.RWMutex
	activeSpans   map[SpanID]*span
}

// TracerOption applies configuration to a Tracer
type TracerOption func(*tracerConfig)

type tracerConfig struct {
	schemaURL string
}

// WithSchemaURL sets the schema URL
func WithSchemaURL(url string) TracerOption {
	return func(cfg *tracerConfig) {
		cfg.schemaURL = url
	}
}

// Start starts a new span
func (t *Tracer) Start(ctx context.Context, name string, opts ...SpanStartOption) (context.Context, Span) {
	if t.provider.isShutdown() {
		// Return no-op span if shutdown
		return ctx, &noOpSpan{}
	}

	if name != "" {
		coverage.RecordHit(name)
		if !coverage.IsKnown(name) {
			coverage.RecordUnknown()
			slog.Warn("observability: unknown operation", "operation", name)
		}
	}

	// Apply options
	startOpts := &SpanStartConfig{
		Parent:  SpanContextFromContext(ctx),
		TraceID: GenerateTraceID(),
	}
	for _, opt := range opts {
		opt(startOpts)
	}
	if startOpts.StartTime.IsZero() {
		startOpts.StartTime = time.Now()
	}

	// Determine if this span should be sampled
	shouldSample := t.provider.sampler.ShouldSample(startOpts.TraceID)

	// Generate span ID
	spanID := GenerateSpanID()

	// Create span context
	var sc SpanContext
	if startOpts.Parent != nil && startOpts.Parent.IsValid() {
		// Inherit trace ID from parent
		sc = SpanContext{
			TraceID:    startOpts.Parent.TraceID,
			SpanID:     spanID,
			TraceFlags: FlagSampled,
			TraceState: startOpts.Parent.TraceState,
		}
	} else {
		// New trace
		sc = SpanContext{
			TraceID:    startOpts.TraceID,
			SpanID:     spanID,
			TraceFlags: FlagSampled,
		}
	}

	// Create parent context if provided
	var parent *SpanContext
	if startOpts.Parent != nil && startOpts.Parent.IsValid() {
		parent = startOpts.Parent
	}

	// Create span
	s := &span{
		name:      name,
		sc:        sc,
		parent:    parent,
		kind:      startOpts.Kind,
		startTime: startOpts.StartTime,
		attrs:     make(map[string]interface{}),
		events:    make([]Event, 0),
		recording: shouldSample,
		exporter:  t.provider.exporter,
	}

	// Set initial attributes
	for _, attr := range startOpts.Attributes {
		s.attrs[attr.Key] = attr.Value
	}

	// Store span for shutdown cleanup
	t.activeSpansMu.Lock()
	t.activeSpans[spanID] = s
	t.activeSpansMu.Unlock()

	// Inject span context into context
	ctx = context.WithValue(ctx, spanContextKey, sc)

	return ctx, s
}

// EndSpan ends a span by its ID
func (t *Tracer) EndSpan(spanID SpanID) {
	t.activeSpansMu.Lock()
	defer t.activeSpansMu.Unlock()

	if s, ok := t.activeSpans[spanID]; ok {
		s.End()
		delete(t.activeSpans, spanID)
	}
}

// ActiveSpanCount returns the number of active spans
func (t *Tracer) ActiveSpanCount() int {
	t.activeSpansMu.RLock()
	defer t.activeSpansMu.RUnlock()
	return len(t.activeSpans)
}

// SpanStartOption applies configuration when starting a span
type SpanStartOption func(*SpanStartConfig)

// SpanStartConfig holds span start configuration
type SpanStartConfig struct {
	Parent    *SpanContext
	TraceID   TraceID
	StartTime time.Time
	Kind      SpanKind
	Attributes []Attribute
}

// WithParent sets the parent span context
func WithParent(sc SpanContext) SpanStartOption {
	return func(cfg *SpanStartConfig) {
		cfg.Parent = &sc
	}
}

// WithTraceID sets the trace ID
func WithTraceID(tid TraceID) SpanStartOption {
	return func(cfg *SpanStartConfig) {
		cfg.TraceID = tid
	}
}

// WithStartTime sets the span start time
func WithStartTime(t time.Time) SpanStartOption {
	return func(cfg *SpanStartConfig) {
		cfg.StartTime = t
	}
}

// WithSpanKind sets the span kind
func WithSpanKind(kind SpanKind) SpanStartOption {
	return func(cfg *SpanStartConfig) {
		cfg.Kind = kind
	}
}

// WithAttributes sets initial attributes
func WithSpanAttributes(attrs ...Attribute) SpanStartOption {
	return func(cfg *SpanStartConfig) {
		cfg.Attributes = append(cfg.Attributes, attrs...)
	}
}

// GenerateTraceID generates a new 16-byte trace ID
func GenerateTraceID() TraceID {
	var tid TraceID
	_, _ = rand.Read(tid[:])
	return tid
}

// GenerateSpanID generates a new 8-byte span ID
func GenerateSpanID() SpanID {
	var sid SpanID
	_, _ = rand.Read(sid[:])
	return sid
}

// noOpSpan is a no-operation span for when tracing is disabled
type noOpSpan struct{}

func (n *noOpSpan) End(opts ...SpanEndOption)                          {}
func (n *noOpSpan) SetStatus(code SpanStatusCode, description string)  {}
func (n *noOpSpan) RecordError(err error, opts ...RecordErrorOption)  {}
func (n *noOpSpan) SetAttributes(kv ...Attribute)                    {}
func (n *noOpSpan) AddEvent(name string, opts ...EventOption)        {}
func (n *noOpSpan) SpanContext() SpanContext                          { return SpanContext{} }
func (n *noOpSpan) IsRecording() bool                               { return false }

// String implements Stringer
func (t *Tracer) String() string {
	return fmt.Sprintf("Tracer(%s)", t.name)
}
