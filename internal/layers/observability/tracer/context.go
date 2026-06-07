package tracer

import (
	"context"
)

// contextKeyType is the type of the context key
type contextKeyType struct{}

// spanContextKey is the key used to store SpanContext in context
var spanContextKey = contextKeyType{}

// SpanContextFromContext extracts SpanContext from context
func SpanContextFromContext(ctx context.Context) *SpanContext {
	if ctx == nil {
		return nil
	}
	
	val := ctx.Value(spanContextKey)
	if val == nil {
		return nil
	}
	
	sc, ok := val.(SpanContext)
	if !ok {
		return nil
	}
	
	// Check if valid
	if !sc.TraceID.IsValid() || !sc.SpanID.IsValid() {
		return nil
	}
	
	return &sc
}

// ContextWithSpan returns a new context with the given SpanContext
func ContextWithSpan(ctx context.Context, sc SpanContext) context.Context {
	return context.WithValue(ctx, spanContextKey, sc)
}

// SpanFromContext extracts Span from context (if stored)
func SpanFromContext(ctx context.Context) Span {
	if ctx == nil {
		return nil
	}
	
	val := ctx.Value(spanKey)
	if val == nil {
		return nil
	}
	
	s, ok := val.(Span)
	if !ok {
		return nil
	}
	
	return s
}

// spanKey is the key used to store the active Span
var spanKey = contextKeyType{}

// ContextWithSpan returns a new context with the given Span
func ContextWithSpanValue(ctx context.Context, s Span) context.Context {
	return context.WithValue(ctx, spanKey, s)
}

// Detach returns a new context that's detached from the parent
// but preserves the trace context for propagation
func Detach(ctx context.Context) context.Context {
	sc := SpanContextFromContext(ctx)
	if sc == nil {
		return context.Background()
	}
	return ContextWithSpan(context.Background(), *sc)
}
