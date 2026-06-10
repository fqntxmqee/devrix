package observability

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/observability/metrics"
	"github.com/devrix/devrix/internal/layers/observability/settings"
)

// Covers: L5-OBS-METRICS-03
func TestRecordGenAITokenUsage_should_register_token_usage_counter(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Metrics.Enabled = true
	obs, err := New(cfg)
	if err != nil {
		t.Fatalf("observability: %v", err)
	}
	defer obs.Shutdown(t.Context())

	meter := obs.Meter()
	RecordGenAITokenUsage(meter, "gpt-4o", 100, 50)
	RecordGenAITokenUsage(meter, "gpt-4o", 10, 5)

	out := meter.Registry().Output()
	for _, want := range []string{
		"devrix_gen_ai.client.token.usage",
		`token_type="input"`,
		`model="gpt-4o"`,
		`token_type="output"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in metrics:\n%s", want, out)
		}
	}
}

func TestRecordGenAITokenUsage_should_noop_when_meter_nil(t *testing.T) {
	RecordGenAITokenUsage(nil, "m", 1, 1)
}

func TestRecordGenAITokenUsage_should_skip_zero_tokens(t *testing.T) {
	mp := metrics.NewMeterProvider(&settings.MetricsConfig{})
	meter := mp.Meter("test")
	RecordGenAITokenUsage(meter, "m", 0, 0)
	if meter.Registry().Count() != 0 {
		t.Fatalf("expected no metrics registered")
	}
}
