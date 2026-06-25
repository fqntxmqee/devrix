// Package verify anomaly detection tests. v6.0.0 6 S 精简 S4-A47
// (system.anomaly_detect) 的 Certifier role 端到端 Span emit 测试。
//
// 覆盖：
//   D7-S4-A47-T11 DetectSystemAnomaly_Triggered_HighSeverity
//   D7-S4-A47-T12 DetectSystemAnomaly_Triggered_MediumSeverity
//   D7-S4-A47-T13 DetectSystemAnomaly_NotTriggered_NoneSeverity
//   D7-S4-A47-T14 DetectSystemAnomaly_NilBridgeFailSafe
package verify

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/d7spans"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
)

// resetBridgeAndDefer clears the package-level d7spans bridge before
// the test and restores it after — keeps state from leaking between
// TestDetectSystemAnomaly_* cases.
func resetBridgeAndDefer(t *testing.T) {
	t.Helper()
	d7spans.SetBridge(nil)
	t.Cleanup(func() { d7spans.SetBridge(nil) })
}

// makeObsPayload returns a minimal payload appropriate for the
// ObservationKind, mirroring orchtypes/system_anomaly_wiring_test.go's
// mkObsPayload. Required because NewObservation validates the payload
// per its concrete type.
func makeObsPayload(kind orchtypes.ObservationKind) orchtypes.Payload {
	switch kind {
	case orchtypes.ObsFact:
		return orchtypes.FactPayload{Statement: "test"}
	case orchtypes.ObsSignal:
		return orchtypes.SignalPayload{Name: "test", Value: 0}
	case orchtypes.ObsDeviation:
		return orchtypes.DeviationPayload{Metric: "test", Expected: 0, Observed: 0, Delta: 0}
	case orchtypes.ObsUncertainty:
		return orchtypes.UncertaintyPayload{Question: "test", Confidence: 0.5}
	default:
		return orchtypes.FactPayload{Statement: "test"}
	}
}

// makeObs constructs a minimal Observation for testing anomaly wiring.
// Mirrors orchtypes.mkObs.
func makeObs(kind orchtypes.ObservationKind, cat orchtypes.Category, strength float64) orchtypes.Observation {
	obs, _ := orchtypes.NewObservation(kind, cat, strength, makeObsPayload(kind), "test_source")
	return obs
}

// makeReport builds an UncertaintyReport with N CatSystem anomalies
// and M CatBusiness anomalies. The Anomalies slice is set directly to
// bypass Partition (which only retains CatSystem ObsDeviation) — this
// mirrors orchtypes/system_anomaly_wiring_test.go's mkReport pattern
// and lets us construct mixed ratios for severity testing.
func makeReport(sessionID string, catSystem, catBusiness int) orchtypes.UncertaintyReport {
	obs := make([]orchtypes.Observation, 0, catSystem+catBusiness)
	for i := 0; i < catSystem; i++ {
		obs = append(obs, makeObs(orchtypes.ObsDeviation, orchtypes.CatSystem, 0.9))
	}
	for i := 0; i < catBusiness; i++ {
		obs = append(obs, makeObs(orchtypes.ObsDeviation, orchtypes.CatBusiness, 0.9))
	}
	rep, err := orchtypes.NewUncertaintyReport(sessionID, []orchtypes.Observation{})
	if err != nil {
		panic(err)
	}
	rep.Anomalies = obs
	return rep
}

func TestDetectSystemAnomaly_Triggered_HighSeverity(t *testing.T) {
	resetBridgeAndDefer(t)

	rep := makeReport("sess_high", 5, 0) // 100% CatSystem, well above 0.5 ratio + 3 count thresholds
	res := DetectSystemAnomaly(context.Background(), "sess_high", rep, 3, 0.5, "")
	if !res.Triggered {
		t.Fatalf("expected triggered=true, got false (threshold=3, ratio=1.0)")
	}
	if res.Kind != AnomalyKindCatSystemAggregate {
		t.Errorf("Kind=%q, want %q", res.Kind, AnomalyKindCatSystemAggregate)
	}
	if res.Severity != SeverityHigh {
		t.Errorf("Severity=%q, want %q (ratio=1.0)", res.Severity, SeverityHigh)
	}
	if res.Threshold != 5 {
		t.Errorf("Threshold=%d, want 5 (total anomalies observed)", res.Threshold)
	}
	if res.EvidenceID != "sess_high:5:5" {
		t.Errorf("EvidenceID=%q, want %q", res.EvidenceID, "sess_high:5:5")
	}
}

func TestDetectSystemAnomaly_Triggered_MediumSeverity(t *testing.T) {
	resetBridgeAndDefer(t)

	rep := makeReport("sess_med", 4, 1) // 80% CatSystem (>= 0.75), 5 total >= 3
	res := DetectSystemAnomaly(context.Background(), "sess_med", rep, 3, 0.5, "")
	if !res.Triggered {
		t.Fatalf("expected triggered=true (5 ≥ 3 AND 0.8 ≥ 0.5)")
	}
	if res.Severity != SeverityMedium {
		t.Errorf("Severity=%q, want %q (ratio=0.8)", res.Severity, SeverityMedium)
	}
}

func TestDetectSystemAnomaly_NotTriggered_NoneSeverity(t *testing.T) {
	resetBridgeAndDefer(t)

	rep := makeReport("sess_off", 1, 4) // 20% CatSystem < 0.5 ratio
	res := DetectSystemAnomaly(context.Background(), "sess_off", rep, 3, 0.5, "")
	if res.Triggered {
		t.Fatalf("expected triggered=false (ratio=0.2 < 0.5)")
	}
	if res.Severity != SeverityNone {
		t.Errorf("Severity=%q, want %q", res.Severity, SeverityNone)
	}
	if res.Kind != AnomalyKindCatSystemAggregate {
		t.Errorf("Kind=%q, want %q", res.Kind, AnomalyKindCatSystemAggregate)
	}
}

func TestDetectSystemAnomaly_OverrideKind(t *testing.T) {
	resetBridgeAndDefer(t)

	rep := makeReport("sess_k", 3, 0)
	res := DetectSystemAnomaly(context.Background(), "sess_k", rep, 3, 0.5, AnomalyKindRateSpike)
	if res.Kind != AnomalyKindRateSpike {
		t.Errorf("Kind=%q, want override %q", res.Kind, AnomalyKindRateSpike)
	}
}

func TestDetectSystemAnomaly_NilBridgeFailSafe(t *testing.T) {
	resetBridgeAndDefer(t)

	rep := makeReport("sess_safe", 4, 0)
	res := DetectSystemAnomaly(context.Background(), "sess_safe", rep, 3, 0.5, "")
	// Must not panic; the result must still be computed correctly because
	// the Span emit is a no-op when the bridge is nil.
	if !res.Triggered {
		t.Fatal("expected triggered=true even with nil bridge")
	}
	if res.Severity != SeverityHigh {
		t.Errorf("Severity=%q, want %q", res.Severity, SeverityHigh)
	}
}

func TestDetectSystemAnomaly_DefaultThresholds(t *testing.T) {
	resetBridgeAndDefer(t)

	// threshold=0 + ratio=0 → use workmodel defaults (3, 0.5).
	rep := makeReport("sess_def", 3, 0)
	res := DetectSystemAnomaly(context.Background(), "sess_def", rep, 0, 0, "")
	if !res.Triggered {
		t.Fatal("expected triggered=true with default thresholds (3 CatSystem ≥ 3, ratio 1.0 ≥ 0.5)")
	}
}