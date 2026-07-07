// Package plan: PlanParseReject tests (DM-20260707-001 PR-F, T67).
//
// 6 sub-class rejection tests:
//
//   1. MalformedJSON (truncated / bad brace)
//   2. UnknownKind (commitment_typo)
//   3. MissingField (no "kind" key)
//   4. InvalidNumeric (NaN / out-of-range)
//   5. InvalidEnum (PersistScope invalid / Op invalid)
//   6. InvalidAST (duplicate Step IDs / empty directive / > 32 steps)
//
// Plus helpers: Error()/Unwrap()/Raw() sanity, AllRejectionCodes enumeration.
package plan

import (
	"errors"
	"strings"
	"testing"
)

// minimalValidPlanJSON returns a JSON document that passes ParsePlan's
// happy path. Tests then mutate one field to trigger each sub-class.
func minimalValidPlanJSON() string {
	return `{
  "id": "p_test",
  "session_id": "s_test",
  "kind": "commitment_plan",
  "strength": 0.85,
  "steps": [{"id": "s1", "directive": "step 1"}],
  "source_observation_ids": ["obs_1"],
  "failure_criteria": [{"field": "exit_code", "op": "eq", "value": 0}],
  "blast_radius": {"file_count": 1, "api_call_count": 1, "token_cost": 100, "persist_scope": "transient"}
}`
}

// TestPlanParseReject_ValidPlan: baseline happy path returns no error.
func TestPlanParseReject_ValidPlan(t *testing.T) {
	t.Parallel()
	p, err := ParsePlan([]byte(minimalValidPlanJSON()))
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if p == nil {
		t.Fatalf("expected plan, got nil")
	}
	if p.Kind != CommitmentPlan {
		t.Errorf("Kind = %v, want CommitmentPlan", p.Kind)
	}
}

// TestPlanParseReject_All6SubClasses: table-driven across each sub-class.
func TestPlanParseReject_All6SubClasses(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		input       string
		wantCode    FieldRejectionCode
		wantErrHas  string
		wantField   string
	}{
		{
			name:       "1_MalformedJSON_Truncated",
			input:      `{"id": "p_test", "session_id":`,
			wantCode:   CodeParseMalformedJSON,
			wantErrHas: "json decode failed",
			wantField:  "$.<root>",
		},
		{
			name:       "1_MalformedJSON_MultiDocument",
			input:      minimalValidPlanJSON() + "\n" + minimalValidPlanJSON(),
			wantCode:   CodeParseMalformedJSON,
			wantErrHas: "multiple JSON documents",
			wantField:  "$.<root>",
		},
		{
			name:       "1_MalformedJSON_UnknownField",
			input:      strings.Replace(minimalValidPlanJSON(), `"commitment_plan"`, `"commitment_plan", "extra_typo": 1`, 1),
			wantCode:   CodeParseMalformedJSON,
			wantErrHas: "extra_typo",
			wantField:  "$.<root>",
		},
		{
			name: "2_UnknownKind",
			input: strings.Replace(
				minimalValidPlanJSON(),
				`"commitment_plan"`,
				`"commitment_typo"`, 1),
			wantCode:   CodeParseUnknownKind,
			wantErrHas: "unknown kind",
			wantField:  "$.kind",
		},
		{
			name: "3_MissingField_Kind",
			input: strings.Replace(
				minimalValidPlanJSON(),
				`"kind": "commitment_plan",`,
				``, 1),
			wantCode:   CodeParseMissingField,
			wantErrHas: "missing required field",
			wantField:  "$.kind",
		},
		{
			name: "3_MissingField_Strength",
			input: strings.Replace(
				minimalValidPlanJSON(),
				`"strength": 0.85,`,
				``, 1),
			wantCode:   CodeParseMissingField,
			wantErrHas: "missing required field",
			wantField:  "$.strength",
		},
		{
			name: "4_InvalidNumeric_StrengthAsString",
			input: strings.Replace(
				minimalValidPlanJSON(),
				`"strength": 0.85`,
				`"strength": "high"`, 1),
			wantCode:   CodeParseInvalidNumeric,
			wantErrHas: "strength must be a finite number",
			wantField:  "$.strength",
		},
		{
			name: "4_InvalidNumeric_StrengthAboveRange",
			input: strings.Replace(
				minimalValidPlanJSON(),
				`"strength": 0.85`,
				`"strength": 1.5`, 1),
			wantCode:   CodeParseInvalidNumeric,
			wantErrHas: "out of [0, 1]",
			wantField:  "$.strength",
		},
		{
			name: "5_InvalidEnum_PersistScope",
			input: strings.Replace(
				minimalValidPlanJSON(),
				`"persist_scope": "transient"`,
				`"persist_scope": "rainy_day"`, 1),
			wantCode:   CodeParseInvalidEnum,
			wantErrHas: "persist_scope",
			wantField:  "$.blast_radius.persist_scope",
		},
		{
			name: "5_InvalidEnum_FailureCriterionOp",
			input: strings.Replace(
				minimalValidPlanJSON(),
				`"op": "eq"`,
				`"op": "regex"`, 1),
			wantCode:   CodeParseInvalidEnum,
			wantErrHas: "op=",
			wantField:  "$.failure_criteria[0].op",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p, err := ParsePlan([]byte(tc.input))
			if p != nil {
				t.Errorf("expected nil plan on rejection, got %+v", p)
			}
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			var rej *PlanParseRejection
			if !errors.As(err, &rej) {
				t.Fatalf("expected *PlanParseRejection, got %T (%v)", err, err)
			}
			if rej.Reason.Code != tc.wantCode {
				t.Errorf("Code = %s, want %s", rej.Reason.Code, tc.wantCode)
			}
			if rej.Reason.Field != tc.wantField {
				t.Errorf("Field = %s, want %s", rej.Reason.Field, tc.wantField)
			}
			if !strings.Contains(rej.Reason.Message, tc.wantErrHas) {
				t.Errorf("Message = %q, missing %q", rej.Reason.Message, tc.wantErrHas)
			}
			if rej.Error() == "" {
				t.Errorf("Error() returned empty string")
			}
		})
	}
}

// TestPlanParseReject_InvalidAST: AST-level semantic invariants.
func TestPlanParseReject_InvalidAST(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		mutate    func() string
		wantField string
		wantMsg   string
	}{
		{
			name: "DuplicateStepIDs",
			mutate: func() string {
				s := minimalValidPlanJSON()
				s = strings.Replace(s,
					`"steps": [{"id": "s1", "directive": "step 1"}]`,
					`"steps": [{"id": "s1", "directive": "step 1"}, {"id": "s1", "directive": "step 2"}]`,
					1)
				return s
			},
			wantField: "$.steps[1].id",
			wantMsg:   "duplicate step id",
		},
		{
			name: "EmptyDirective",
			mutate: func() string {
				s := minimalValidPlanJSON()
				s = strings.Replace(s,
					`"directive": "step 1"`,
					`"directive": ""`,
					1)
				return s
			},
			wantField: "$.steps[0].directive",
			wantMsg:   "directive must be non-empty",
		},
		{
			name: "TooManySteps",
			mutate: func() string {
				var b strings.Builder
				b.WriteString(`"steps": [`)
				for i := 0; i < 33; i++ {
					if i > 0 {
						b.WriteString(",")
					}
					b.WriteString(`{"id": "s` + intToStr(i) + `", "directive": "d"}`)
				}
				b.WriteString(`]`)
				return strings.Replace(minimalValidPlanJSON(),
					`"steps": [{"id": "s1", "directive": "step 1"}]`,
					b.String(), 1)
			},
			wantField: "$.steps",
			wantMsg:   "exceeds hard cap",
		},
		{
			name: "DuplicateSourceObsIDs",
			mutate: func() string {
				return strings.Replace(minimalValidPlanJSON(),
					`"source_observation_ids": ["obs_1"]`,
					`"source_observation_ids": ["obs_1", "obs_1"]`,
					1)
			},
			wantField: "$.source_observation_ids",
			wantMsg:   "duplicates",
		},
		{
			name: "EmptySourceObsID",
			mutate: func() string {
				return strings.Replace(minimalValidPlanJSON(),
					`"source_observation_ids": ["obs_1"]`,
					`"source_observation_ids": ["obs_1", ""]`,
					1)
			},
			wantField: "$.source_observation_ids[1]",
			wantMsg:   "must be non-empty",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParsePlan([]byte(tc.mutate()))
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			var rej *PlanParseRejection
			if !errors.As(err, &rej) {
				t.Fatalf("expected *PlanParseRejection, got %T", err)
			}
			if rej.Reason.Code != CodeParseInvalidAST {
				t.Errorf("Code = %s, want CodeParseInvalidAST", rej.Reason.Code)
			}
			if rej.Reason.Field != tc.wantField {
				t.Errorf("Field = %s, want %s", rej.Reason.Field, tc.wantField)
			}
			if !strings.Contains(rej.Reason.Message, tc.wantMsg) {
				t.Errorf("Message = %q, missing %q", rej.Reason.Message, tc.wantMsg)
			}
		})
	}
}

// TestPlanParseReject_RawBytes: Raw() returns truncated input (≤4 KB).
func TestPlanParseReject_RawBytes(t *testing.T) {
	t.Parallel()
	// Build a 5 KB input that fails JSON decode (truncated).
	big := strings.Repeat(`"a":1,`, 5000)
	input := `{"id":"p","session_id":"s"` + big
	_, err := ParsePlan([]byte(input))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	var rej *PlanParseRejection
	if !errors.As(err, &rej) {
		t.Fatalf("expected *PlanParseRejection")
	}
	raw := rej.Raw()
	if len(raw) > 4096 {
		t.Errorf("Raw() returned %d bytes, expected ≤ 4096", len(raw))
	}
	if len(raw) == 0 {
		t.Errorf("Raw() returned 0 bytes, want truncated input")
	}
}

// TestPlanParseReject_Unwrap: errors.Is matches ErrPlanParseRejected.
func TestPlanParseReject_Unwrap(t *testing.T) {
	t.Parallel()
	_, err := ParsePlan([]byte(`{"id":`))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, ErrPlanParseRejected) {
		t.Errorf("errors.Is did not match ErrPlanParseRejected")
	}
}

// TestPlanParseReject_AllRejectionCodes: 10+6 = 16 codes enumerated
// (10 field-validator codes from plan_field_validator.go + 6 parse codes).
func TestPlanParseReject_AllRejectionCodes(t *testing.T) {
	t.Parallel()
	codes := AllRejectionCodes()
	if len(codes) != 16 {
		t.Errorf("AllRejectionCodes returned %d codes, want 16", len(codes))
	}
	// 6 parse-only enumeration.
	parseCodes := AllParseRejectionCodes()
	if len(parseCodes) != 6 {
		t.Errorf("AllParseRejectionCodes returned %d codes, want 6", len(parseCodes))
	}
	seen := map[FieldRejectionCode]bool{}
	for _, c := range codes {
		if seen[c] {
			t.Errorf("duplicate code %s in taxonomy", c)
		}
		seen[c] = true
	}
}

// TestPlanParseReject_EmptyInput: empty input → CodeParseMalformedJSON.
func TestPlanParseReject_EmptyInput(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"", "  ", "\n\t"} {
		_, err := ParsePlan([]byte(in))
		if err == nil {
			t.Errorf("empty input %q: expected error, got nil", in)
			continue
		}
		var rej *PlanParseRejection
		if !errors.As(err, &rej) {
			continue
		}
		if rej.Reason.Code != CodeParseMalformedJSON {
			t.Errorf("input %q: Code = %s, want CodeParseMalformedJSON", in, rej.Reason.Code)
		}
	}
}

// intToStr avoids importing strconv for the small test helper above.
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
