package types

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLearningClass_String_5Kinds(t *testing.T) {
	cases := []struct {
		class LearningClass
		want  string
	}{
		{LearningSOP, "sop"},
		{LearningProtocol, "protocol"},
		{LearningKnowledge, "knowledge"},
		{LearningConclusion, "conclusion"},
		{LearningPending, "pending"},
	}
	for _, tc := range cases {
		if got := tc.class.String(); got != tc.want {
			t.Errorf("LearningClass(%d).String() = %q, want %q", uint8(tc.class), got, tc.want)
		}
	}
}

func TestLearningClass_String_UnknownValue(t *testing.T) {
	unknown := LearningClass(99)
	got := unknown.String()
	want := "LearningClass(99)"
	if got != want {
		t.Errorf("unknown LearningClass.String() = %q, want %q", got, want)
	}
}

func TestLearningClass_ParseLearningClass_5Kinds(t *testing.T) {
	cases := []struct {
		in   string
		want LearningClass
	}{
		{"sop", LearningSOP},
		{"protocol", LearningProtocol},
		{"knowledge", LearningKnowledge},
		{"conclusion", LearningConclusion},
		{"pending", LearningPending},
	}
	for _, tc := range cases {
		got, err := ParseLearningClass(tc.in)
		if err != nil {
			t.Errorf("ParseLearningClass(%q) returned error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseLearningClass(%q) = %d, want %d", tc.in, uint8(got), uint8(tc.want))
		}
	}
}

func TestLearningClass_ParseLearningClass_UnknownFailFast(t *testing.T) {
	_, err := ParseLearningClass("unknown_class")
	if err == nil {
		t.Fatal(`ParseLearningClass("unknown_class") should return error`)
	}
	if !strings.Contains(err.Error(), `unknown LearningClass "unknown_class"`) {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLearningClass_ParseLearningClass_LearningUnknownRejected(t *testing.T) {
	// "unknown" is a reserved name; passing it must fail (don't allow "unknown" → LearningUnknown).
	_, err := ParseLearningClass("unknown")
	if err == nil {
		t.Fatal(`ParseLearningClass("unknown") should return error (reserved name)`)
	}
}

func TestLearningClass_MarshalJSON_WireFormat(t *testing.T) {
	cases := []struct {
		class LearningClass
		want  string
	}{
		{LearningSOP, `"sop"`},
		{LearningProtocol, `"protocol"`},
		{LearningKnowledge, `"knowledge"`},
		{LearningConclusion, `"conclusion"`},
		{LearningPending, `"pending"`},
	}
	for _, tc := range cases {
		got, err := json.Marshal(tc.class)
		if err != nil {
			t.Errorf("Marshal(%v) returned error: %v", tc.class, err)
			continue
		}
		if string(got) != tc.want {
			t.Errorf("Marshal(%v) = %s, want %s", tc.class, got, tc.want)
		}
	}
}

func TestLearningClass_UnmarshalJSON_EmptyString_DefaultsToLearningSOP(t *testing.T) {
	// Empty string decodes to LearningSOP (zero value compatible with Phase 3 SideEffectStatus precedent).
	var k LearningClass
	if err := json.Unmarshal([]byte(`""`), &k); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if k != LearningSOP {
		t.Errorf("Unmarshal empty string = %d, want LearningSOP (%d)", uint8(k), uint8(LearningSOP))
	}
}

func TestLearningClass_UnmarshalJSON_RoundTrip(t *testing.T) {
	cases := []LearningClass{LearningSOP, LearningProtocol, LearningKnowledge, LearningConclusion, LearningPending}
	for _, tc := range cases {
		data, err := json.Marshal(tc)
		if err != nil {
			t.Fatalf("Marshal(%v) returned error: %v", tc, err)
		}
		var got LearningClass
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal(%s) returned error: %v", data, err)
		}
		if got != tc {
			t.Errorf("round-trip %v → %s → %v", tc, data, got)
		}
	}
}

func TestLearningClass_UnmarshalJSON_UnknownFailFast(t *testing.T) {
	var k LearningClass
	err := json.Unmarshal([]byte(`"bogus_class"`), &k)
	if err == nil {
		t.Fatal(`Unmarshal("bogus_class") should return error`)
	}
	if !strings.Contains(err.Error(), `unknown LearningClass`) {
		t.Errorf("unexpected error message: %v", err)
	}
}