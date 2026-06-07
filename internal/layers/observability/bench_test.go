package observability

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/observability/metrics"
)

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

func BenchmarkRegistryOutput(b *testing.B) {
	r := metrics.NewRegistry(nil, nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = r.Output()
	}
}
