package observability

import (
	"strings"
	"testing"
)

// T: D5-S2-A01-T03
func TestToolBridge_InitLatencyMetrics_should_register_tool_latency(t *testing.T) {
	cfg := DefaultConfig()
	obs, err := New(cfg)
	if err != nil {
		t.Fatalf("observability: %v", err)
	}
	defer obs.Shutdown(t.Context())

	bridge := NewBridge(obs)
	tb := NewToolBridgeFromBridge(bridge)
	m, err := tb.InitLatencyMetrics("bash", "high", "ok")
	if err != nil {
		t.Fatalf("InitLatencyMetrics: %v", err)
	}
	if m == nil || m.Latency == nil {
		t.Fatal("expected latency histogram")
	}
	m.Latency.Observe(0.42)

	out := obs.Meter().Registry().Output()
	checks := []string{
		"devrix_tool_latency",
		`tool="bash"`,
		`risk_level="high"`,
		`status="ok"`,
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in metrics output:\n%s", want, out)
		}
	}
}
