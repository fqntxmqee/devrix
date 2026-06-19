package logger

import (
	"context"
	"log/slog"
	"os"

	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
)

// ContextHandler injects traceId/spanId from context into slog records.
type ContextHandler struct {
	inner slog.Handler
}

// NewContextHandler wraps a slog handler with trace context injection.
func NewContextHandler(inner slog.Handler) *ContextHandler {
	return &ContextHandler{inner: inner}
}

func (h *ContextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := tracer.SpanContextFromContext(ctx); sc != nil && sc.IsValid() {
		r.AddAttrs(
			slog.String("traceId", sc.TraceID.String()),
			slog.String("spanId", sc.SpanID.String()),
		)
	}
	return h.inner.Handle(ctx, r)
}

func (h *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ContextHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *ContextHandler) WithGroup(name string) slog.Handler {
	return &ContextHandler{inner: h.inner.WithGroup(name)}
}

// InstallSlogBridge wraps the default slog handler to inject traceId/spanId from context.
// Idempotent wrapper stack: subsequent calls re-wrap the latest default handler rather than
// the original, preventing handler duplication across repeated init paths.
func InstallSlogBridge() {
	inner := slog.Default().Handler()
	if inner == nil {
		inner = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
	}
	slog.SetDefault(slog.New(NewContextHandler(inner)))
}
