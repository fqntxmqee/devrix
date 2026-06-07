package observability

import (
	"bytes"
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/observability/metrics"
	"github.com/devrix/devrix/internal/layers/observability/tracer"
)

func BenchmarkSpanCreate(b *testing.B) {
	tp := tracer.NewTracerProvider(nil, nil)
	tr := tp.Tracer("test")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, span := tr.Start(ctx, "test.span")
		span.End()
	}
}

func BenchmarkSpanCreateWithAttributes(b *testing.B) {
	tp := tracer.NewTracerProvider(nil, nil)
	tr := tp.Tracer("test")
	ctx := context.Background()
	attrs := []tracer.Attribute{
		{Key: "key1", Value: "value1"},
		{Key: "key2", Value: 42},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, span := tr.Start(ctx, "test.span", tracer.WithSpanAttributes(attrs...))
		span.End()
	}
}

func BenchmarkCounterInc(b *testing.B) {
	c := metrics.NewCounter("test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Inc()
	}
}

func BenchmarkHistogramObserve(b *testing.B) {
	h := metrics.NewHistogram("test", nil, metrics.LLMHistogramBounds())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.Observe(1.5)
	}
}

func BenchmarkJSONLogger(b *testing.B) {
	cfg := &LoggerConfig{
		Level:  "info",
		Format: "json",
	}
	l := NewStructuredLogger(cfg)
	var buf bytes.Buffer
	l.SetOutput(&buf)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Info("test message", "key", "value")
		buf.Reset()
	}
}

func BenchmarkPrometheusOutput(b *testing.B) {
	r := metrics.NewRegistry(nil, nil)
	c := metrics.NewCounter("test_counter", metrics.LabelMap{"label": "value"})
	_ = r.RegisterCounter("test_counter", metrics.LabelMap{"label": "value"}, c)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.Output()
	}
}

func BenchmarkSpanContextPropagation(b *testing.B) {
	tp := tracer.NewTracerProvider(nil, nil)
	tr := tp.Tracer("test")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx2, span := tr.Start(ctx, "span1")
		ctx3, span2 := tr.Start(ctx2, "span2")
		span.End()
		span2.End()
		ctx = context.Background() // Reset
	}
}
