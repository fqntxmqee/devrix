package metrics

import (
	"strings"
	"testing"
)

// Covers: L5-OBS-FIX-02
func TestHistogram_golden_prometheus_output(t *testing.T) {
	h := NewHistogram("request_latency", LabelMap{"service": "devrix"}, []float64{0.1, 0.5, 1.0})
	h.Observe(0.05)
	h.Observe(0.3)
	h.Observe(0.8)
	h.Observe(2.0)

	buckets := h.Buckets()
	if buckets[0.1] != 1 {
		t.Fatalf("bucket 0.1: got %d want 1", buckets[0.1])
	}
	if buckets[0.5] != 1 {
		t.Fatalf("bucket 0.5: got %d want 1", buckets[0.5])
	}
	if buckets[1.0] != 1 {
		t.Fatalf("bucket 1.0: got %d want 1", buckets[1.0])
	}

	r := NewRegistry(nil, nil)
	if err := r.RegisterHistogram("request_latency", LabelMap{"service": "devrix"}, h); err != nil {
		t.Fatalf("register: %v", err)
	}
	out := r.Output()

	checks := []string{
		`le="0.1"{service="devrix"}} 1`,
		`le="0.5"{service="devrix"}} 2`,
		`le="1"{service="devrix"}} 3`,
		`le="+Inf"{service="devrix"}} 4`,
		`_count{service="devrix"} 4`,
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in output:\n%s", want, out)
		}
	}
}
