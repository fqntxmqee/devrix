package tracer

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
)

// TextMapCarrier is an interface for propagating trace context via text maps
type TextMapCarrier interface {
	Get(key string) string
	Set(key string, value string)
	Keys() []string
}

// W3C TraceContext propagator
type Propagator struct{}

// NewPropagator creates a new W3C TraceContext propagator
func NewPropagator() *Propagator {
	return &Propagator{}
}

// Inject injects trace context into a carrier
func (p *Propagator) Inject(ctx context.Context, carrier TextMapCarrier) error {
	sc := SpanContextFromContext(ctx)
	if sc == nil || !sc.IsValid() {
		return nil
	}

	// Inject traceparent header
	traceparent := sc.String()
	carrier.Set("traceparent", traceparent)

	// Inject tracestate header if present
	if sc.TraceState != "" {
		carrier.Set("tracestate", string(sc.TraceState))
	}

	if baggage := DefaultBaggageManager.FormatHeader(ctx); baggage != "" {
		carrier.Set(BaggageHeader, baggage)
	}

	return nil
}

// Extract extracts trace context from a carrier
func (p *Propagator) Extract(ctx context.Context, carrier TextMapCarrier) (SpanContext, error) {
	// Extract traceparent header
	traceparent := carrier.Get("traceparent")
	if traceparent == "" {
		return SpanContext{}, fmt.Errorf("missing traceparent header")
	}

	// Parse traceparent: 00-{traceId}-{spanId}-{flags}
	sc, err := parseTraceparent(traceparent)
	if err != nil {
		return SpanContext{}, fmt.Errorf("invalid traceparent: %w", err)
	}

	// Extract tracestate header
	tracestate := carrier.Get("tracestate")
	if tracestate != "" {
		sc.TraceState = TraceState(tracestate)
	}

	sc.Remote = true
	return sc, nil
}

// ExtractContext extracts trace and baggage context from a carrier.
func (p *Propagator) ExtractContext(ctx context.Context, carrier TextMapCarrier) (context.Context, SpanContext, error) {
	sc, err := p.Extract(ctx, carrier)
	if err != nil {
		return ctx, SpanContext{}, err
	}
	ctx = ContextWithSpan(ctx, sc)
	ctx = DefaultBaggageManager.ApplyHeader(ctx, carrier.Get(BaggageHeader))
	return ctx, sc, nil
}

// parseTraceparent parses W3C traceparent header
// Format: 00-{traceId}-{spanId}-{flags}
// - version: 2 hex chars (currently always "00")
// - traceId: 32 hex chars
// - spanId: 16 hex chars
// - flags: 2 hex chars
func parseTraceparent(header string) (SpanContext, error) {
	parts := strings.Split(header, "-")
	if len(parts) != 4 {
		return SpanContext{}, fmt.Errorf("invalid traceparent format: expected 4 parts, got %d", len(parts))
	}

	// Version (ignore for now)
	version := parts[0]
	if version != "00" {
		// For future versions, we'd need version-specific handling
		// For now, accept but log warning
	}

	traceIDStr := parts[1]
	spanIDStr := parts[2]
	flagsStr := parts[3]

	// Validate lengths
	if len(traceIDStr) != 32 {
		return SpanContext{}, fmt.Errorf("invalid traceId length: %d (expected 32)", len(traceIDStr))
	}
	if len(spanIDStr) != 16 {
		return SpanContext{}, fmt.Errorf("invalid spanId length: %d (expected 16)", len(spanIDStr))
	}
	if len(flagsStr) != 2 {
		return SpanContext{}, fmt.Errorf("invalid flags length: %d (expected 2)", len(flagsStr))
	}

	// Parse traceId (16 bytes = 32 hex chars)
	traceIDBytes, err := hex.DecodeString(traceIDStr)
	if err != nil {
		return SpanContext{}, fmt.Errorf("invalid traceId hex: %w", err)
	}
	var traceID TraceID
	copy(traceID[:], traceIDBytes)

	// Parse spanId (8 bytes = 16 hex chars)
	spanIDBytes, err := hex.DecodeString(spanIDStr)
	if err != nil {
		return SpanContext{}, fmt.Errorf("invalid spanId hex: %w", err)
	}
	var spanID SpanID
	copy(spanID[:], spanIDBytes)

	// Parse flags
	flags, err := hex.DecodeString(flagsStr)
	if err != nil {
		return SpanContext{}, fmt.Errorf("invalid flags hex: %w", err)
	}

	return SpanContext{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: TraceFlags(flags[0]),
	}, nil
}

// Traceparent returns the W3C traceparent header value
func (sc SpanContext) Traceparent() string {
	return sc.String()
}

// Tracestate returns the W3C tracestate header value
func (sc SpanContext) Tracestate() string {
	return string(sc.TraceState)
}

// MapCarrier is a simple implementation of TextMapCarrier using a map
type MapCarrier map[string]string

// Get returns the value for a key
func (m MapCarrier) Get(key string) string {
	return m[key]
}

// Set sets a key-value pair
func (m MapCarrier) Set(key string, value string) {
	m[key] = value
}

// Keys returns all keys
func (m MapCarrier) Keys() []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// HTTPHeaderCarrier is a carrier for HTTP headers
type HTTPHeaderCarrier struct {
	headers map[string][]string
}

// NewHTTPHeaderCarrier creates a new HTTP header carrier
func NewHTTPHeaderCarrier() *HTTPHeaderCarrier {
	return &HTTPHeaderCarrier{
		headers: make(map[string][]string),
	}
}

// Get returns the first value for a key
func (h *HTTPHeaderCarrier) Get(key string) string {
	if vals, ok := h.headers[key]; ok && len(vals) > 0 {
		return vals[0]
	}
	return ""
}

// Set sets a header value
func (h *HTTPHeaderCarrier) Set(key string, value string) {
	h.headers[key] = []string{value}
}

// Keys returns all header keys
func (h *HTTPHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(h.headers))
	for k := range h.headers {
		keys = append(keys, k)
	}
	return keys
}

// GetAll returns all values for a key
func (h *HTTPHeaderCarrier) GetAll(key string) []string {
	if vals, ok := h.headers[key]; ok {
		return vals
	}
	return nil
}
