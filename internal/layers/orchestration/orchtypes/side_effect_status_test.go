package orchtypes

import (
	"encoding/json"
	"testing"
)

func TestSideEffectStatus_5States_String(t *testing.T) {
	cases := map[SideEffectStatus]string{
		SideEffectNone:       "none",
		SideEffectUnknown:    "unknown",
		SideEffectInflight:   "inflight",
		SideEffectCommitted:  "committed",
		SideEffectRolledBack: "rolled_back",
	}
	for s, want := range cases {
		if got := string(s); got != want {
			t.Errorf("%v String() = %q, want %q", s, got, want)
		}
	}
}

func TestSideEffectStatus_5States_RoundTrip(t *testing.T) {
	statuses := []SideEffectStatus{
		SideEffectNone,
		SideEffectUnknown,
		SideEffectInflight,
		SideEffectCommitted,
		SideEffectRolledBack,
	}
	for _, s := range statuses {
		t.Run(string(s), func(t *testing.T) {
			data, err := json.Marshal(s)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if got := string(data); got != `"`+string(s)+`"` {
				t.Errorf("Marshal = %s, want quoted string", got)
			}
			var got SideEffectStatus
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if got != s {
				t.Errorf("Roundtrip = %v, want %v", got, s)
			}
		})
	}
}

func TestSideEffectStatus_IsTerminal(t *testing.T) {
	terminal := []SideEffectStatus{
		SideEffectNone,
		SideEffectCommitted,
		SideEffectRolledBack,
	}
	for _, s := range terminal {
		if !s.IsTerminal() {
			t.Errorf("%v should be terminal", s)
		}
	}
	notTerminal := []SideEffectStatus{
		SideEffectUnknown,
		SideEffectInflight,
	}
	for _, s := range notTerminal {
		if s.IsTerminal() {
			t.Errorf("%v should NOT be terminal", s)
		}
	}
}

func TestSideEffectStatus_NeedsAttention(t *testing.T) {
	attention := []SideEffectStatus{
		SideEffectUnknown,
		SideEffectInflight,
	}
	for _, s := range attention {
		if !s.NeedsAttention() {
			t.Errorf("%v should need attention", s)
		}
	}
	noAttention := []SideEffectStatus{
		SideEffectNone,
		SideEffectCommitted,
		SideEffectRolledBack,
	}
	for _, s := range noAttention {
		if s.NeedsAttention() {
			t.Errorf("%v should NOT need attention", s)
		}
	}
}

// TestSideEffectStatus_ReusesUncertaintyCoordType verifies that the type
// alias defined here is the same one consumed by UncertaintyCoord. This
// invariant is what lets D5 dashboards filter on side_effect_status across
// both Artifact and UncertaintyCoord without a translation shim.
func TestSideEffectStatus_ReusesUncertaintyCoordType(t *testing.T) {
	c := NewUncertaintyCoord(0.5)
	c2 := c.WithSideEffect(SideEffectInflight)
	if c2.SideEffectStatus != SideEffectInflight {
		t.Errorf("UncertaintyCoord.WithSideEffect(SideEffectInflight) = %q, want %q",
			c2.SideEffectStatus, SideEffectInflight)
	}
	// A different literal of the same string value should also work, proving
	// the type alias is a string-typed enum (compile-time guarantee).
	c3 := c.WithSideEffect("inflight")
	if c3.SideEffectStatus != SideEffectInflight {
		t.Errorf("UncertaintyCoord.WithSideEffect(\"inflight\") = %q, want %q",
			c3.SideEffectStatus, SideEffectInflight)
	}
}

func TestSideEffectDetail_JSON_RoundTrip(t *testing.T) {
	d := SideEffectDetail{
		IdempotencyKey:   "idem-123",
		SentAt:           1719180000000000000,
		ConfirmedAt:      1719180001000000000,
		CompensationLog:  "rolled back via http_delete",
		CompensationTool: "http_delete",
	}
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got SideEffectDetail
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.IdempotencyKey != d.IdempotencyKey {
		t.Errorf("IdempotencyKey = %q, want %q", got.IdempotencyKey, d.IdempotencyKey)
	}
	if got.SentAt != d.SentAt {
		t.Errorf("SentAt = %d, want %d", got.SentAt, d.SentAt)
	}
	if got.CompensationTool != d.CompensationTool {
		t.Errorf("CompensationTool = %q, want %q", got.CompensationTool, d.CompensationTool)
	}
}
