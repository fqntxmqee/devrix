// Package plan: PlanFieldValidator tests (DM-20260707-001 PR-F, T66).
//
// 8-sub-class table-driven coverage:
//
//   1. KindUnset
//   2. StepsEmpty
//   3. SourceObservationIDsEmpty
//   4. StrengthOutOfRange
//   5. PersistScopeInvalid
//   6. FailureCriteriaEmpty
//   7. FailureCriteriaInvalidOp
//   8. FailureCriteriaInvalidField
//
// Plus PP-3 (BlastRadiusExceeded — 3 axes share code but differ in Audit.Axis)
// and DAG (new in PR-F).
package plan

import (
	"errors"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

// validFieldPlan returns a baseline valid Plan that tests then mutate to trigger
// each rejection sub-class. The baseline passes all 8 checks.
func validFieldPlan() *Plan {
	return &Plan{
		ID:                   "p_valid",
		SessionID:            "s_valid",
		Kind:                 CommitmentPlan,
		Strength:             0.85,
		Steps:                []Step{{ID: "s1", Directive: "step 1"}},
		SourceObservationIDs: []string{"obs_1"},
		FailureCriteria: []FailureCriterion{
			{Field: "exit_code", Op: "eq", Value: 0},
		},
		BlastRadius: BlastRadius{
			FileCount:    1,
			APICallCount: 1,
			TokenCost:    100,
			PersistScope: PersistTransient,
		},
	}
}

// TestPlanFieldValidator_ValidPlan: baseline returns no error.
func TestPlanFieldValidator_ValidPlan(t *testing.T) {
	t.Parallel()
	v := NewPlanFieldValidator(ValidateOpts{})
	err, audit := v.Validate(validFieldPlan())
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if audit.Code != "" {
		t.Errorf("expected empty audit, got %s", audit.Code)
	}
}

// TestPlanFieldValidator_NilPlan: nil input → CodeKindUnset.
func TestPlanFieldValidator_NilPlan(t *testing.T) {
	t.Parallel()
	v := NewPlanFieldValidator(ValidateOpts{})
	err, audit := v.Validate(nil)
	if err == nil {
		t.Fatalf("expected error for nil plan, got nil")
	}
	if audit.Code != CodeKindUnset {
		t.Errorf("audit.Code = %s, want CodeKindUnset", audit.Code)
	}
}

// TestPlanFieldValidator_All8SubClasses: table-driven test for each of the
// 8 named sub-classes. Each row mutates one field of the baseline and
// asserts the matching Code + wrapped error.
func TestPlanFieldValidator_All8SubClasses(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		mutate      func(p *Plan)
		wantCode    FieldRejectionCode
		wantErrPart string
		wantField   string
	}{
		{
			name:        "1_KindUnset",
			mutate:      func(p *Plan) { p.Kind = KindUnset },
			wantCode:    CodeKindUnset,
			wantErrPart: "Kind is unset",
			wantField:   "Kind",
		},
		{
			name:        "2_StepsEmpty",
			mutate:      func(p *Plan) { p.Steps = nil },
			wantCode:    CodeStepsEmpty,
			wantErrPart: "Steps must be non-empty",
			wantField:   "Steps",
		},
		{
			name:        "3_SourceObservationIDsEmpty",
			mutate:      func(p *Plan) { p.SourceObservationIDs = nil },
			wantCode:    CodeSourceObservationIDsEmpty,
			wantErrPart: "SourceObservationIDs is required",
			wantField:   "SourceObservationIDs",
		},
		{
			name:        "4_StrengthOutOfRange_Negative",
			mutate:      func(p *Plan) { p.Strength = -0.1 },
			wantCode:    CodeStrengthOutOfRange,
			wantErrPart: "Strength=-0.1 out of",
			wantField:   "Strength",
		},
		{
			name:        "4_StrengthOutOfRange_Above1",
			mutate:      func(p *Plan) { p.Strength = 1.1 },
			wantCode:    CodeStrengthOutOfRange,
			wantErrPart: "Strength=1.1 out of",
			wantField:   "Strength",
		},
		{
			name:        "5_PersistScopeInvalid",
			mutate:      func(p *Plan) { p.BlastRadius.PersistScope = "invalid" },
			wantCode:    CodePersistScopeInvalid,
			wantErrPart: `PersistScope="invalid"`,
			wantField:   "BlastRadius.PersistScope",
		},
		{
			name:        "6_FailureCriteriaEmpty",
			mutate:      func(p *Plan) { p.FailureCriteria = nil },
			wantCode:    CodeFailureCriteriaEmpty,
			wantErrPart: "FailureCriteria is empty",
			wantField:   "FailureCriteria",
		},
		{
			name:        "7_FailureCriteriaInvalidOp",
			mutate:      func(p *Plan) { p.FailureCriteria[0].Op = "regex" },
			wantCode:    CodeFailureCriteriaInvalidOp,
			wantErrPart: `Op="regex" not in whitelist`,
			wantField:   "FailureCriteria[0].Op",
		},
		{
			name:        "8_FailureCriteriaInvalidField",
			mutate:      func(p *Plan) { p.FailureCriteria[0].Field = "internal_state" },
			wantCode:    CodeFailureCriteriaInvalidField,
			wantErrPart: `Field="internal_state" not in observable`,
			wantField:   "FailureCriteria[0].Field",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := validFieldPlan()
			tc.mutate(p)
			v := NewPlanFieldValidator(ValidateOpts{})
			err, audit := v.Validate(p)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if audit.Code != tc.wantCode {
				t.Errorf("audit.Code = %s, want %s", audit.Code, tc.wantCode)
			}
			if !strings.Contains(err.Error(), tc.wantErrPart) {
				t.Errorf("error = %q, missing substring %q", err.Error(), tc.wantErrPart)
			}
			if audit.Field != tc.wantField {
				t.Errorf("audit.Field = %s, want %s", audit.Field, tc.wantField)
			}
			if audit.Message == "" {
				t.Errorf("audit.Message is empty")
			}
		})
	}
}

// TestPlanFieldValidator_BlastRadius_ThreeAxes: PP-3 covers 3 axes
// (FileCount/APICallCount/TokenCost). All share CodeBlastRadiusExceeded but
// differ in Audit.Axis.
func TestPlanFieldValidator_BlastRadius_ThreeAxes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		mutate   func(p *Plan)
		wantAxis string
	}{
		{
			name:     "BlastRadius_FileCount",
			mutate:   func(p *Plan) { p.BlastRadius.FileCount = 1000 },
			wantAxis: "FileCount",
		},
		{
			name:     "BlastRadius_APICallCount",
			mutate:   func(p *Plan) { p.BlastRadius.APICallCount = 1000 },
			wantAxis: "APICallCount",
		},
		{
			name:     "BlastRadius_TokenCost",
			mutate:   func(p *Plan) { p.BlastRadius.TokenCost = 1_000_000 },
			wantAxis: "TokenCost",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := validFieldPlan()
			tc.mutate(p)
			v := NewPlanFieldValidator(ValidateOpts{})
			err, audit := v.Validate(p)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if audit.Code != CodeBlastRadiusExceeded {
				t.Errorf("audit.Code = %s, want CodeBlastRadiusExceeded", audit.Code)
			}
			if audit.Axis != tc.wantAxis {
				t.Errorf("audit.Axis = %s, want %s", audit.Axis, tc.wantAxis)
			}
			if audit.Observed == 0 || audit.Limit == 0 {
				t.Errorf("Observed/Limit not populated: %d/%d", audit.Observed, audit.Limit)
			}
		})
	}
}

// TestPlanFieldValidator_AllFieldRejectionCodes: enumerate the full taxonomy.
func TestPlanFieldValidator_AllFieldRejectionCodes(t *testing.T) {
	t.Parallel()
	codes := AllFieldRejectionCodes()
	if len(codes) != 10 {
		t.Errorf("AllFieldRejectionCodes returned %d codes, want 10", len(codes))
	}
	seen := map[FieldRejectionCode]bool{}
	for _, c := range codes {
		if seen[c] {
			t.Errorf("duplicate code %s", c)
		}
		seen[c] = true
	}
}

// TestPlanFieldValidator_AuditString: log-line format includes all populated
// fields (Code / Field / Axis / Observed / Limit / Message).
func TestPlanFieldValidator_AuditString(t *testing.T) {
	t.Parallel()
	audit := PlanFieldAudit{
		Code:     CodeBlastRadiusExceeded,
		Field:    "BlastRadius.FileCount",
		Axis:     "FileCount",
		Observed: 200,
		Limit:    50,
		Message:  "PP-3 violation",
	}
	s := audit.String()
	for _, want := range []string{"PLAN_BLAST_8003", "BlastRadius.FileCount", "FileCount", "200", "50", "PP-3"} {
		if !strings.Contains(s, want) {
			t.Errorf("String() = %q, missing %q", s, want)
		}
	}
}

// TestPlanFieldValidator_ValidPlan_DoesNotMatchAnySentinel: sanity check —
// a valid Plan's Validate returns no error AND no wrapped sentinel.
func TestPlanFieldValidator_ValidPlan_DoesNotMatchAnySentinel(t *testing.T) {
	t.Parallel()
	v := NewPlanFieldValidator(ValidateOpts{})
	err, audit := v.Validate(validFieldPlan())
	if err != nil {
		t.Errorf("valid plan returned error: %v", err)
	}
	for _, sentinel := range []error{
		ErrPlanKindUnset, ErrPlanStepsEmpty, ErrPlanSourceObservationIDsRequired,
		ErrPlanStrengthOutOfRange, ErrPlanPersistScopeInvalid,
		ErrPlanFailureCriteriaEmpty, ErrPlanFailureCriteriaInvalidOp,
		ErrPlanFailureCriteriaInvalidField, ErrPlanBlastRadiusExceeded,
	} {
		if err != nil && errors.Is(err, sentinel) {
			t.Errorf("valid plan matched sentinel %v", sentinel)
		}
	}
	if audit.Code != "" {
		t.Errorf("valid plan audit.Code = %q, want empty", audit.Code)
	}
}

// Sanity check that types is imported (used in other test files; prevents
// unused-import false positives in this file when refactoring).
var _ = types.VerdictPass