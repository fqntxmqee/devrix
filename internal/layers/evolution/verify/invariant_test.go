package verify

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/ltllite"
)

func TestParseVerifyInvariants_GoodStruct_Succeeds(t *testing.T) {
	set, err := parseVerifyInvariants()
	if err != nil {
		t.Fatalf("default invariant parse failed: %v", err)
	}
	if len(set.Invariants) == 0 {
		t.Error("expected non-empty invariant set")
	}
}

func TestParseVerifyInvariants_BadStruct_ReturnsError(t *testing.T) {
	// Empty tag triggers ErrInvalidInvariant (parser line 84)
	type badStruct struct {
		X string `invariant:""`
	}
	_, err := ltllite.ParseStruct(badStruct{})
	if err == nil {
		t.Fatal("expected parse error for empty invariant tag")
	}
}

func TestVerifyInvariants_InitSucceeds(t *testing.T) {
	// init() runs at package load; verify that verifyInvariantSet is populated.
	if len(verifyInvariantSet.Invariants) == 0 {
		t.Error("verifyInvariantSet should be populated after init()")
	}
}

func TestCheckVerifyInvariants_NoViolations(t *testing.T) {
	state := ltllite.MapState{
		"verify_called":           true,
		"no_plan_mutation":        true,
		"known_kind":              true,
		"checker_available":       true,
		"skipped_counted":         true,
		"not_in_unverified":       true,
		"report_emitted":          true,
		"required_fields_present": true,
	}
	violations := CheckVerifyInvariants(state)
	if len(violations) != 0 {
		t.Errorf("expected no violations, got %d: %v", len(violations), violations)
	}
}

func TestCheckVerifyInvariants_ViolationDetected(t *testing.T) {
	// ReadOnly invariant: verify_called => no_plan_mutation
	// If verify_called is true but no_plan_mutation is false, expect violation.
	state := ltllite.MapState{
		"verify_called":           true,
		"no_plan_mutation":        false,
		"known_kind":              true,
		"checker_available":       true,
		"skipped_counted":         true,
		"not_in_unverified":       true,
		"report_emitted":          true,
		"required_fields_present": true,
	}
	violations := CheckVerifyInvariants(state)
	if len(violations) == 0 {
		t.Error("expected at least one violation, got 0")
	}
}