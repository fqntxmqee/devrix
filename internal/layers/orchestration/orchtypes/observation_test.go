package orchtypes

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

func TestObservation_4Kinds_4Categories(t *testing.T) {
	// 4 Kinds × 2 Categories = 8 valid combinations.
	kinds := []ObservationKind{ObsFact, ObsSignal, ObsDeviation, ObsUncertainty}
	categories := []Category{CatBusiness, CatSystem}

	for _, k := range kinds {
		for _, c := range categories {
			t.Run(k.String()+"_"+c.String(), func(t *testing.T) {
				var p Payload
				switch k {
				case ObsFact:
					p = FactPayload{Statement: "user is admin"}
				case ObsSignal:
					p = SignalPayload{Name: "tokens", Value: 100, Threshold: 50}
				case ObsDeviation:
					p = DeviationPayload{Metric: "latency", Expected: 100, Observed: 300, Delta: 200}
				case ObsUncertainty:
					p = UncertaintyPayload{Question: "intent?", Confidence: 0.4}
				}
				obs, err := NewObservation(k, c, 0.5, p, "test")
				if err != nil {
					t.Fatalf("NewObservation failed: %v", err)
				}
				if obs.ID == "" {
					t.Error("ID should be auto-generated")
				}
				if obs.DetectedAt.IsZero() {
					t.Error("DetectedAt should default to now")
				}
				if obs.Strength != 0.5 {
					t.Errorf("Strength = %.2f, want 0.5", obs.Strength)
				}
			})
		}
	}
}

func TestObservation_Immutability(t *testing.T) {
	p := FactPayload{Statement: "x"}
	obs, _ := NewObservation(ObsFact, CatBusiness, 0.5, p, "test")

	// With* must return a copy, not mutate the receiver.
	obs2 := obs.WithKind(ObsSignal)
	obs3 := obs.WithStrength(0.9)
	obs4 := obs.WithCategory(CatSystem)

	// Receiver is unchanged.
	if obs.Kind != ObsFact {
		t.Errorf("receiver mutated: Kind=%s, want fact", obs.Kind)
	}
	if obs.Strength != 0.5 {
		t.Errorf("receiver mutated: Strength=%.2f, want 0.5", obs.Strength)
	}
	if obs.Category != CatBusiness {
		t.Errorf("receiver mutated: Category=%s, want business", obs.Category)
	}

	// Returned copies reflect the new values.
	if obs2.Kind != ObsSignal {
		t.Errorf("WithKind: Kind=%s, want signal", obs2.Kind)
	}
	if obs3.Strength != 0.9 {
		t.Errorf("WithStrength: Strength=%.2f, want 0.9", obs3.Strength)
	}
	if obs4.Category != CatSystem {
		t.Errorf("WithCategory: Category=%s, want system", obs4.Category)
	}

	// obs2 should still hold the original Payload (WithKind doesn't touch it).
	if obs2.Payload == nil {
		t.Error("WithKind dropped Payload")
	}
}

func TestObservation_StrengthClamping(t *testing.T) {
	p := FactPayload{Statement: "x"}
	obs, _ := NewObservation(ObsFact, CatBusiness, 1.5, p, "test")
	if obs.Strength != 1.0 {
		t.Errorf("Strength overflow: got %.2f, want 1.0", obs.Strength)
	}
	obs2 := obs.WithStrength(-0.3)
	if obs2.Strength != 0.0 {
		t.Errorf("Strength underflow: got %.2f, want 0.0", obs2.Strength)
	}
}

func TestObservation_NilPayload_Rejected(t *testing.T) {
	_, err := NewObservation(ObsFact, CatBusiness, 0.5, nil, "test")
	if err == nil {
		t.Fatal("expected error for nil Payload")
	}
}

func TestObservation_PayloadValidate(t *testing.T) {
	tests := []struct {
		name    string
		kind    ObservationKind
		payload Payload
		wantErr bool
	}{
		{"Fact_OK", ObsFact, FactPayload{Statement: "x"}, false},
		{"Fact_Empty", ObsFact, FactPayload{Statement: ""}, true},
		{"Signal_OK", ObsSignal, SignalPayload{Name: "n", Value: 1}, false},
		{"Signal_EmptyName", ObsSignal, SignalPayload{}, true},
		{"Deviation_OK", ObsDeviation, DeviationPayload{Metric: "m", Expected: 1, Observed: 2, Delta: 1}, false},
		{"Deviation_EmptyMetric", ObsDeviation, DeviationPayload{}, true},
		{"Uncertainty_OK", ObsUncertainty, UncertaintyPayload{Question: "q?", Confidence: 0.5}, false},
		{"Uncertainty_EmptyQuestion", ObsUncertainty, UncertaintyPayload{Confidence: 0.5}, true},
		{"Uncertainty_BadConfidence", ObsUncertainty, UncertaintyPayload{Question: "q?", Confidence: 1.5}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewObservation(tt.kind, CatBusiness, 0.5, tt.payload, "test")
			if (err != nil) != tt.wantErr {
				t.Errorf("err=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestObservation_PayloadTypeAssertion(t *testing.T) {
	factObs, _ := NewObservation(ObsFact, CatBusiness, 0.5, FactPayload{Statement: "x"}, "test")
	if _, ok := factObs.Payload.(FactPayload); !ok {
		t.Error("Payload should be FactPayload")
	}

	sigObs, _ := NewObservation(ObsSignal, CatBusiness, 0.5, SignalPayload{Name: "n", Value: 1}, "test")
	if _, ok := sigObs.Payload.(SignalPayload); !ok {
		t.Error("Payload should be SignalPayload")
	}
}

func TestObservation_JSON_RoundTrip(t *testing.T) {
	obs, _ := NewObservation(ObsFact, CatBusiness, 0.7, FactPayload{Statement: "x", Evidence: []string{"a", "b"}}, "src")
	data, err := json.Marshal(obs)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(data), `"category":"business"`) {
		t.Errorf("JSON missing category: %s", data)
	}
	var got Observation
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.ID != obs.ID || got.Strength != obs.Strength {
		t.Errorf("Roundtrip mismatch: got=%+v, want=%+v", got, obs)
	}
}

func TestCategory_String(t *testing.T) {
	if CatBusiness.String() != "business" {
		t.Errorf("CatBusiness = %s", CatBusiness.String())
	}
	if CatSystem.String() != "system" {
		t.Errorf("CatSystem = %s", CatSystem.String())
	}
	if Category(99).String() == "" {
		t.Error("unknown category should produce non-empty debug string")
	}
}

func TestObservationKind_String(t *testing.T) {
	want := map[ObservationKind]string{
		ObsFact:        "fact",
		ObsSignal:      "signal",
		ObsDeviation:   "deviation",
		ObsUncertainty: "uncertainty",
	}
	for k, w := range want {
		if k.String() != w {
			t.Errorf("%d = %s, want %s", k, k.String(), w)
		}
	}
	if ObservationKind(99).String() == "" {
		t.Error("unknown kind should produce non-empty debug string")
	}
}

func TestObservation_DetectedAt_PreservesCustom(t *testing.T) {
	now := time.Now().Add(-time.Hour)
	p := FactPayload{Statement: "x"}
	obs, _ := NewObservation(ObsFact, CatBusiness, 0.5, p, "test")
	obs.DetectedAt = now
	if !obs.DetectedAt.Equal(now) {
		t.Error("DetectedAt should be assignable for backfill")
	}
}

func TestObservation_Validate_OK(t *testing.T) {
	p := FactPayload{Statement: "x"}
	obs, _ := NewObservation(ObsFact, CatBusiness, 0.5, p, "test")
	if err := obs.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestObservation_Validate_RejectsBadFields(t *testing.T) {
	p := FactPayload{Statement: "x"}
	obs, _ := NewObservation(ObsFact, CatBusiness, 0.5, p, "test")
	cases := []struct {
		name   string
		mutate func(o *Observation)
	}{
		{"empty_id", func(o *Observation) { o.ID = "" }},
		{"strength_high", func(o *Observation) { o.Strength = 1.5 }},
		{"strength_low", func(o *Observation) { o.Strength = -0.1 }},
		{"zero_detected_at", func(o *Observation) { o.DetectedAt = time.Time{} }},
		{"nil_payload", func(o *Observation) { o.Payload = nil }},
		{"unknown_category", func(o *Observation) { o.Category = 99 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := obs
			tc.mutate(&o)
			if err := o.Validate(); err == nil {
				t.Errorf("expected error for %s", tc.name)
			}
		})
	}
}

func TestObservation_Validate_PropagatesPayloadError(t *testing.T) {
	obs, _ := NewObservation(ObsFact, CatBusiness, 0.5, FactPayload{Statement: "x"}, "test")
	obs.Payload = FactPayload{Statement: ""} // invalid: empty statement
	err := obs.Validate()
	if err == nil {
		t.Fatal("expected error from payload validation")
	}
	if !errors.Is(err, ErrObservationPayloadInvalid) {
		t.Errorf("expected ErrObservationPayloadInvalid, got %v", err)
	}
}

// ⭐ RF.3.1 W1: Observation wire format = nested payload object under
// "payload" key, discriminator via "kind". Roundtrip must reconstruct
// the concrete Payload type (FactPayload / SignalPayload / etc.) from
// the Kind tag.
func TestObservation_MarshalJSON_WireFormat(t *testing.T) {
	p := FactPayload{Statement: "user is admin", Evidence: []string{"e1"}}
	obs, err := NewObservation(ObsFact, CatBusiness, 0.75, p, "detector-x")
	if err != nil {
		t.Fatalf("NewObservation: %v", err)
	}

	data, err := json.Marshal(obs)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	s := string(data)

	// Wire format must include the discriminator and the payload as a
	// nested object (not a string / array).
	if !strings.Contains(s, `"kind":"fact"`) {
		t.Errorf("wire format missing kind discriminator: %s", s)
	}
	if !strings.Contains(s, `"payload":{`) {
		t.Errorf("payload should be a nested object: %s", s)
	}
	if !strings.Contains(s, `"statement":"user is admin"`) {
		t.Errorf("payload fields should be flattened inside the nested object: %s", s)
	}
	if !strings.Contains(s, `"category":"business"`) {
		t.Errorf("category should be human-readable string: %s", s)
	}

	// Roundtrip: unmarshal must reconstruct the concrete FactPayload.
	var got Observation
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	fp, ok := got.Payload.(FactPayload)
	if !ok {
		t.Fatalf("Payload type lost in roundtrip: %T", got.Payload)
	}
	if fp.Statement != "user is admin" {
		t.Errorf("Statement = %q, want %q", fp.Statement, "user is admin")
	}
	if got.Strength != 0.75 {
		t.Errorf("Strength = %.3f, want 0.75", got.Strength)
	}
}

// ⭐ RF.3.1 W2: validateFact must wrap ErrObservationPayloadInvalid via
// fmt.Errorf("%w") so errors.Is works uniformly across all Payload
// concrete types (Signal/Deviation/Uncertainty already do this).
func TestObservation_ValidateFact_WrappedError(t *testing.T) {
	err := validateFact(FactPayload{Statement: ""})
	if err == nil {
		t.Fatal("expected error for empty FactPayload.Statement")
	}
	if !errors.Is(err, ErrObservationPayloadInvalid) {
		t.Errorf("errors.Is(err, ErrObservationPayloadInvalid) = false; got %v", err)
	}
	// Sanity: error message should mention FactPayload so the source is
	// traceable when reading logs.
	if !strings.Contains(err.Error(), "FactPayload") {
		t.Errorf("error message should mention FactPayload, got: %s", err.Error())
	}
}

// ⭐ RF.3.1 W3: clamp01Float is the unified clamp helper. NaN routes to
// the caller-supplied onNaN. Strength-style clamps pass 0 (hard bound),
// coord-style clamps pass 0.5 (cold-start neutral).
func TestClamp01Float_NaN_Fallback(t *testing.T) {
	if got := clamp01Float(math.NaN(), 0); got != 0 {
		t.Errorf("NaN with onNaN=0 should clamp to 0, got %.3f", got)
	}
	if got := clamp01Float(math.NaN(), 0.5); got != 0.5 {
		t.Errorf("NaN with onNaN=0.5 should clamp to 0.5, got %.3f", got)
	}
	if got := clamp01Float(-0.3, 0); got != 0 {
		t.Errorf("negative should clamp to 0, got %.3f", got)
	}
	if got := clamp01Float(1.5, 0); got != 1 {
		t.Errorf("overflow should clamp to 1, got %.3f", got)
	}
	if got := clamp01Float(0.42, 0); got != 0.42 {
		t.Errorf("in-range value should pass through, got %.3f", got)
	}
	// Symmetric: NaN with onNaN=0.5 should not accidentally hit the 0
	// path (the bug W3 was designed to prevent).
	if got := clamp01Float(math.NaN(), 0.5); got == 0 {
		t.Errorf("NaN/0.5 leaked into the 0 branch — clamp01Float dedup broken")
	}
}
