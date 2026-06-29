package orchtypes

import (
	"encoding/json"
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

func TestArtifactKind_4Types_String(t *testing.T) {
	cases := map[ArtifactKind]string{
		types.ArtifactStateChangeCert: "state_change_cert",
		types.ArtifactResponseRecord:  "response_record",
		types.ArtifactProbeReport:     "probe_report",
		types.ArtifactExperimentData:  "experiment_data",
	}
	for k, want := range cases {
		if got := k.String(); got != want {
			t.Errorf("%d String() = %q, want %q", k, got, want)
		}
	}
	if k := ArtifactKind(99); k.String() == "" {
		t.Error("unknown kind should produce non-empty debug string")
	}
}

func TestArtifactKind_4Types_ParseRoundTrip(t *testing.T) {
	kinds := []ArtifactKind{
		types.ArtifactStateChangeCert,
		types.ArtifactResponseRecord,
		types.ArtifactProbeReport,
		types.ArtifactExperimentData,
	}
	for _, k := range kinds {
		t.Run(k.String(), func(t *testing.T) {
			parsed, err := types.ParseArtifactKind(k.String())
			if err != nil {
				t.Fatalf("ParseArtifactKind(%q): %v", k.String(), err)
			}
			if parsed != k {
				t.Errorf("ParseArtifactKind(%q) = %d, want %d", k.String(), parsed, k)
			}
		})
	}
}

func TestArtifactKind_UnknownValue_ParseError(t *testing.T) {
	_, err := types.ParseArtifactKind("nonexistent_kind")
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
	if !contains(err.Error(), "nonexistent_kind") {
		t.Errorf("error message should echo the bad kind, got: %s", err.Error())
	}
}

func TestArtifactKind_JSON_WireFormat(t *testing.T) {
	// JSON wire format must be a string (snake_case), not the underlying uint8.
	// D5 dashboards key on the string form; a number would break filtering.
	k := types.ArtifactProbeReport
	data, err := json.Marshal(k)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if got := string(data); got != `"probe_report"` {
		t.Errorf("JSON wire format = %s, want %q", got, `"probe_report"`)
	}

	// Roundtrip: unmarshal back to the same kind.
	var got ArtifactKind
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got != types.ArtifactProbeReport {
		t.Errorf("Unmarshal = %d, want %d", got, types.ArtifactProbeReport)
	}
}

func TestArtifactKind_UnmarshalEmptyString_DefaultsToZero(t *testing.T) {
	// Backward compat: a v2 Artifact with no Kind field deserializes to
	// types.ArtifactStateChangeCert (zero value) rather than failing.
	var k ArtifactKind
	if err := json.Unmarshal([]byte(`""`), &k); err != nil {
		t.Fatalf("Unmarshal empty: %v", err)
	}
	if k != types.ArtifactStateChangeCert {
		t.Errorf("empty string should default to zero value, got %d", k)
	}
}

func TestArtifactKind_UnmarshalUnknownString_FailsLoudly(t *testing.T) {
	var k ArtifactKind
	err := json.Unmarshal([]byte(`"not_a_real_kind"`), &k)
	if err == nil {
		t.Fatal("expected error for unknown wire-format kind")
	}
	if !contains(err.Error(), "not_a_real_kind") {
		t.Errorf("error message should echo the bad kind, got: %s", err.Error())
	}
}
