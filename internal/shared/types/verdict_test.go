package types

import (
	"encoding/json"
	"testing"
)

func TestVerdictKind_String_4Kinds(t *testing.T) {
	cases := []struct {
		kind VerdictKind
		want string
	}{
		{VerdictPass, "pass"},
		{VerdictPartial, "partial"},
		{VerdictIndeterminate, "indeterminate"},
		{VerdictFail, "fail"},
	}
	for _, tc := range cases {
		if got := tc.kind.String(); got != tc.want {
			t.Errorf("VerdictKind(%d).String() = %q, want %q", uint8(tc.kind), got, tc.want)
		}
	}
}

func TestVerdictKind_String_UnknownValue(t *testing.T) {
	unknown := VerdictKind(99)
	if got := unknown.String(); got != "VerdictKind(99)" {
		t.Errorf("unknown VerdictKind.String() = %q, want %q", got, "VerdictKind(99)")
	}
}

func TestVerdictKind_ParseVerdictKind_4Kinds(t *testing.T) {
	cases := []struct {
		in   string
		want VerdictKind
	}{
		{"pass", VerdictPass},
		{"partial", VerdictPartial},
		{"indeterminate", VerdictIndeterminate},
		{"fail", VerdictFail},
	}
	for _, tc := range cases {
		got, err := ParseVerdictKind(tc.in)
		if err != nil {
			t.Errorf("ParseVerdictKind(%q) returned error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseVerdictKind(%q) = %d, want %d", tc.in, uint8(got), uint8(tc.want))
		}
	}
}

func TestVerdictKind_ParseVerdictKind_UnknownFailFast(t *testing.T) {
	_, err := ParseVerdictKind("unknown_kind")
	if err == nil {
		t.Fatal("ParseVerdictKind(\"unknown_kind\") should return error")
	}
	if err.Error() != "types: unknown VerdictKind \"unknown_kind\"" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestVerdictKind_MarshalJSON_WireFormat(t *testing.T) {
	cases := []struct {
		kind VerdictKind
		want string
	}{
		{VerdictPass, `"pass"`},
		{VerdictPartial, `"partial"`},
		{VerdictIndeterminate, `"indeterminate"`},
		{VerdictFail, `"fail"`},
	}
	for _, tc := range cases {
		got, err := json.Marshal(tc.kind)
		if err != nil {
			t.Errorf("Marshal(%v) returned error: %v", tc.kind, err)
			continue
		}
		if string(got) != tc.want {
			t.Errorf("Marshal(%v) = %s, want %s", tc.kind, got, tc.want)
		}
	}
}

func TestVerdictKind_UnmarshalJSON_EmptyString_DefaultsToZeroValue(t *testing.T) {
	var k VerdictKind = VerdictFail // start non-zero
	if err := json.Unmarshal([]byte(`""`), &k); err != nil {
		t.Fatalf("Unmarshal empty string returned error: %v", err)
	}
	if k != VerdictPass {
		t.Errorf("Unmarshal empty string: got %d, want %d (VerdictPass zero value)", uint8(k), uint8(VerdictPass))
	}
}

func TestVerdictKind_UnmarshalJSON_4Kinds(t *testing.T) {
	cases := []struct {
		in   string
		want VerdictKind
	}{
		{`"pass"`, VerdictPass},
		{`"partial"`, VerdictPartial},
		{`"indeterminate"`, VerdictIndeterminate},
		{`"fail"`, VerdictFail},
	}
	for _, tc := range cases {
		var k VerdictKind
		if err := json.Unmarshal([]byte(tc.in), &k); err != nil {
			t.Errorf("Unmarshal(%s) returned error: %v", tc.in, err)
			continue
		}
		if k != tc.want {
			t.Errorf("Unmarshal(%s) = %d, want %d", tc.in, uint8(k), uint8(tc.want))
		}
	}
}

func TestVerdictKind_UnmarshalJSON_UnknownValue_Error(t *testing.T) {
	var k VerdictKind
	if err := json.Unmarshal([]byte(`"unknown"`), &k); err == nil {
		t.Fatal("Unmarshal unknown value should return error")
	}
}

func TestVerdictKind_RoundTrip_JSON(t *testing.T) {
	original := VerdictPartial
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	var decoded VerdictKind
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if decoded != original {
		t.Errorf("round-trip mismatch: got %d, want %d", uint8(decoded), uint8(original))
	}
}

func TestVerdictKind_ZeroValue_VerdictPass(t *testing.T) {
	var k VerdictKind
	if k != VerdictPass {
		t.Errorf("VerdictKind zero value = %d, want %d (VerdictPass)", uint8(k), uint8(VerdictPass))
	}
}