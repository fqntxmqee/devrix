package sessionorchestrator

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
)

func TestPromoteSystemCategory_BaselineArtifactSummary(t *testing.T) {
	payload := orchtypes.DeviationPayload{Metric: "p99_latency", Expected: 200, Observed: 850, Delta: 650}
	obs, err := orchtypes.NewObservation(orchtypes.ObsDeviation, orchtypes.CatBusiness, 0.9, payload, "test")
	if err != nil {
		t.Fatalf("NewObservation: %v", err)
	}
	lines := []string{"artifact_summary: P99 latency 850ms (baseline 200ms)"}
	out := promoteSystemCategory([]orchtypes.Observation{obs}, lines)
	if out[0].Category != orchtypes.CatSystem {
		t.Fatalf("category = %s, want CatSystem", out[0].Category)
	}
}

func TestPromoteSystemCategory_NoSignalHint(t *testing.T) {
	payload := orchtypes.DeviationPayload{Metric: "drift", Expected: 1, Observed: 2, Delta: 1}
	obs, err := orchtypes.NewObservation(orchtypes.ObsDeviation, orchtypes.CatBusiness, 0.9, payload, "test")
	if err != nil {
		t.Fatalf("NewObservation: %v", err)
	}
	out := promoteSystemCategory([]orchtypes.Observation{obs}, nil)
	if out[0].Category != orchtypes.CatBusiness {
		t.Fatalf("category = %s, want CatBusiness", out[0].Category)
	}
}

func TestFormatObserveSignalLine_RegisteredPrefix(t *testing.T) {
	line := formatObserveSignalLine(SignalPrefixArtifactSummary, "baseline ok")
	if !isRegisteredObserveSignalLine(line) {
		t.Fatalf("expected registered line, got %q", line)
	}
}
