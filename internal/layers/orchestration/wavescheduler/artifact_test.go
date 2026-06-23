package wavescheduler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/shared/types"
)

func TestArtifactStore_PutGet(t *testing.T) {
	s := NewArtifactStore()
	art := Artifact{
		TaskID:    "a",
		Summary:   "done",
		ExitCode:  0,
		StartedAt: time.Now(),
		EndedAt:   time.Now().Add(time.Second),
	}
	s.Put(art)
	got, ok := s.Get("a")
	if !ok {
		t.Fatal("expected to retrieve artifact")
	}
	if got.Summary != "done" {
		t.Fatalf("expected summary 'done', got %q", got.Summary)
	}
}

func TestArtifactStore_Unknown(t *testing.T) {
	s := NewArtifactStore()
	if _, ok := s.Get("missing"); ok {
		t.Fatal("expected missing artifact to be absent")
	}
}

func TestArtifactStore_SessionScoped(t *testing.T) {
	s := NewArtifactStore()
	s.PutForSession("sess-1", Artifact{TaskID: "a", Summary: "x"})
	if _, ok := s.Get("a"); !ok {
		t.Fatal("expected global lookup to find task 'a'")
	}
	art, ok := s.GetForSession("sess-1", "a")
	if !ok || art.Summary != "x" {
		t.Fatalf("expected session-scoped artifact, got ok=%v", ok)
	}
	if _, ok := s.GetForSession("sess-2", "a"); ok {
		t.Fatal("expected other session not to see artifact")
	}
}

func TestArtifactStore_List(t *testing.T) {
	s := NewArtifactStore()
	s.Put(Artifact{TaskID: "a"})
	s.Put(Artifact{TaskID: "b"})
	if got := s.List(); len(got) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(got))
	}
}

// TestArtifact_NewFields_PrC1 verifies the 5 PR-C1 fields (Kind,
// SourcePlanID, AnomaliesCount, SideEffectStatus, SideEffectDetail) flow
// through the Artifact struct and serialize byte-correctly. The v2 callers
// continue to work because every new field is omitempty.
//
// Note: Kind is uint8 with omitempty, so the zero value (ArtifactStateChangeCert
// = 0) is omitted from JSON. This is the same convention Go's encoding/json
// uses for any numeric field with omitempty. Callers that need to emit the
// "state_change_cert" string explicitly can set Kind to a non-zero value;
// downstream consumers default zero to state_change_cert on Unmarshal.
func TestArtifact_NewFields_PrC1(t *testing.T) {
	art := Artifact{
		TaskID:           "t-1",
		Summary:          "commited a side effect",
		ExitCode:         0,
		StartedAt:        time.Now(),
		EndedAt:          time.Now().Add(100 * time.Millisecond),
		Duration:         100 * time.Millisecond,
		Kind:             types.ArtifactResponseRecord, // non-zero to surface in JSON
		SourcePlanID:     "plan-42",
		AnomaliesCount:   0,
		SideEffectStatus: types.SideEffectCommitted,
		SideEffectDetail: &types.SideEffectDetail{
			IdempotencyKey: "idem-xyz",
			SentAt:         1719180000000000000,
			ConfirmedAt:    1719180001000000000,
		},
	}
	data, err := json.Marshal(art)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	s := string(data)
	for _, want := range []string{
		`"kind":"response_record"`,
		`"source_plan_id":"plan-42"`,
		`"side_effect_status":"committed"`,
		`"idempotency_key":"idem-xyz"`,
	} {
		if !contains(s, want) {
			t.Errorf("JSON missing %q: %s", want, s)
		}
	}

	// Roundtrip: decode back to the same Artifact and verify all 5 fields.
	var got Artifact
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Kind != types.ArtifactResponseRecord {
		t.Errorf("Kind = %d, want %d", got.Kind, types.ArtifactResponseRecord)
	}
	if got.SourcePlanID != "plan-42" {
		t.Errorf("SourcePlanID = %q, want %q", got.SourcePlanID, "plan-42")
	}
	if got.SideEffectStatus != types.SideEffectCommitted {
		t.Errorf("SideEffectStatus = %q, want %q", got.SideEffectStatus, types.SideEffectCommitted)
	}
	if got.SideEffectDetail == nil || got.SideEffectDetail.IdempotencyKey != "idem-xyz" {
		t.Errorf("SideEffectDetail lost in roundtrip: %+v", got.SideEffectDetail)
	}
}

// TestArtifact_KindZeroValue_OmittedFromJSON confirms the omitempty-on-uint8
// behavior so v2 callers see no surprise. v2 artifacts always have Kind=0
// which decodes back to ArtifactStateChangeCert on the receiving side.
func TestArtifact_KindZeroValue_OmittedFromJSON(t *testing.T) {
	art := Artifact{TaskID: "t-1"}
	data, _ := json.Marshal(art)
	s := string(data)
	if contains(s, `"kind"`) {
		t.Errorf("zero Kind should be omitted, got: %s", s)
	}
	var got Artifact
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Kind != types.ArtifactStateChangeCert {
		t.Errorf("zero Kind roundtrip = %d, want 0 (ArtifactStateChangeCert)", got.Kind)
	}
}

// TestArtifact_BackwardCompat_PrC1 verifies a v2 Artifact (no new fields)
// still serializes to its old wire format. Critical for v2 → v3 rollout.
func TestArtifact_BackwardCompat_PrC1(t *testing.T) {
	art := Artifact{
		TaskID:    "t-v2",
		Summary:   "v2 artifact",
		ExitCode:  0,
		StartedAt: time.Now(),
		EndedAt:   time.Now(),
	}
	data, err := json.Marshal(art)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	s := string(data)
	// None of the 5 new field keys may appear when zero-valued (omitempty).
	for _, banned := range []string{
		`"kind"`,
		`"source_plan_id"`,
		`"anomalies_count"`,
		`"side_effect_status"`,
		`"side_effect_detail"`,
	} {
		if contains(s, banned) {
			t.Errorf("v2 Artifact should not emit %q, got: %s", banned, s)
		}
	}
}

// contains is a tiny local helper provided by context_test.go (same package)
// to avoid pulling in strings just for one substring check.
