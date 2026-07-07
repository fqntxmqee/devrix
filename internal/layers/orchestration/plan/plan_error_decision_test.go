// Package plan: PlanErrorDecisionTable tests (DM-20260707-001 PR-F, T70).
//
// Coverage:
//
//   1. TestActionString_AllCases: ActionStr rendering.
//   2. TestAllRoutes_Has11NamedRows: enumeration invariant.
//   3. TestPlanErrorRouteFor_FieldCodes: each of 10 field codes maps to its
//      expected action.
//   4. TestPlanErrorRouteFor_ParseCodes: each of 6 parse codes maps to
//      retry/decompose based on Retryable flag.
//   5. TestPlanErrorRouteFor_CatchAll: unknown Code → catch-all row.
package plan

import (
	"testing"
)

// TestActionString_AllCases.
func TestActionString_AllCases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		a    PlanErrorAction
		want string
	}{
		{ActionUnset, "unset"},
		{ActionRetry, "retry"},
		{ActionDecompose, "decompose"},
		{ActionForcePlan, "force_plan"},
		{ActionAbort, "abort"},
		{PlanErrorAction(99), "unset"}, // unknown → unset
	}
	for _, tc := range cases {
		if got := tc.a.String(); got != tc.want {
			t.Errorf("PlanErrorAction(%d).String() = %s, want %s", tc.a, got, tc.want)
		}
	}
}

// TestAllRoutes_Has11NamedRows.
func TestAllRoutes_Has11NamedRows(t *testing.T) {
	t.Parallel()
	routes := AllRoutes()
	if len(routes) != 11 {
		t.Errorf("AllRoutes returned %d routes, want 11 (10 named + 1 catch-all)", len(routes))
	}
	// Verify names are unique.
	seen := map[string]bool{}
	for _, r := range routes {
		if seen[r.Name] {
			t.Errorf("duplicate route name %q", r.Name)
		}
		seen[r.Name] = true
	}
	// Verify catch-all is the LAST row.
	last := routes[len(routes)-1]
	if last.Name != "plan-error" {
		t.Errorf("last route name = %q, want plan-error (catch-all)", last.Name)
	}
	if last.Action != ActionAbort {
		t.Errorf("catch-all action = %s, want ActionAbort", last.Action)
	}
}

// TestPlanErrorRouteFor_FieldCodes: the 10 named field-validator codes.
func TestPlanErrorRouteFor_FieldCodes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		code          FieldRejectionCode
		wantAction    PlanErrorAction
		wantMaxRetry  int
		wantNameRegex string
	}{
		{CodeKindUnset, ActionAbort, 0, "^kind-unset$"},
		{CodeStepsEmpty, ActionDecompose, 1, "^steps-empty$"},
		{CodeSourceObservationIDsEmpty, ActionAbort, 0, "^lineage-empty$"},
		{CodeStrengthOutOfRange, ActionRetry, DefaultMaxRetries, "^strength-range$"},
		{CodePersistScopeInvalid, ActionRetry, DefaultMaxRetries, "^persist-scope$"},
		{CodeFailureCriteriaEmpty, ActionDecompose, 1, "^pp2-empty$"},
		{CodeFailureCriteriaInvalidOp, ActionRetry, DefaultMaxRetries, "^pp2-op$"},
		{CodeFailureCriteriaInvalidField, ActionRetry, DefaultMaxRetries, "^pp2-field$"},
		{CodeBlastRadiusExceeded, ActionForcePlan, 0, "^blast-radius$"},
		{CodeDAGInvalid, ActionDecompose, 1, "^dag-invalid$"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.code), func(t *testing.T) {
			t.Parallel()
			row, ok := PlanErrorRouteFor(tc.code)
			if !ok {
				t.Fatalf("PlanErrorRouteFor(%s): no match", tc.code)
			}
			if row.Action != tc.wantAction {
				t.Errorf("Action = %s, want %s", row.Action, tc.wantAction)
			}
			if row.MaxRetries != tc.wantMaxRetry {
				t.Errorf("MaxRetries = %d, want %d", row.MaxRetries, tc.wantMaxRetry)
			}
			if row.Name != tc.wantNameRegex[1:len(tc.wantNameRegex)-1] { // strip ^...$
				t.Errorf("Name = %s, want %s", row.Name, tc.wantNameRegex)
			}
		})
	}
}

// TestPlanErrorRouteFor_ParseCodes: retry vs decompose based on hint.Retryable.
func TestPlanErrorRouteFor_ParseCodes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		code         FieldRejectionCode
		wantAction   PlanErrorAction
		wantMaxRetry int
	}{
		{CodeParseMalformedJSON, ActionRetry, DefaultMaxRetries},
		{CodeParseUnknownKind, ActionRetry, DefaultMaxRetries},
		{CodeParseMissingField, ActionRetry, DefaultMaxRetries},
		{CodeParseInvalidNumeric, ActionRetry, DefaultMaxRetries},
		{CodeParseInvalidEnum, ActionRetry, DefaultMaxRetries},
		{CodeParseInvalidAST, ActionDecompose, 1}, // non-retryable
	}
	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.code), func(t *testing.T) {
			t.Parallel()
			row, ok := PlanErrorRouteFor(tc.code)
			if !ok {
				t.Fatalf("PlanErrorRouteFor(%s): no match", tc.code)
			}
			if row.Action != tc.wantAction {
				t.Errorf("Action = %s, want %s", row.Action, tc.wantAction)
			}
			if row.MaxRetries != tc.wantMaxRetry {
				t.Errorf("MaxRetries = %d, want %d", row.MaxRetries, tc.wantMaxRetry)
			}
			// Parse rows come from parseRouteFor, not the catch-all.
			if row.Name == "plan-error" {
				t.Errorf("Name = %s, want per-parse name", row.Name)
			}
		})
	}
}

// TestPlanErrorRouteFor_CatchAll: unknown Code falls into the catch-all row.
func TestPlanErrorRouteFor_CatchAll(t *testing.T) {
	t.Parallel()
	row, ok := PlanErrorRouteFor(FieldRejectionCode("PLAN_UNKNOWN_9999"))
	if !ok {
		t.Fatalf("catch-all should always match; got ok=false")
	}
	if row.Name != "plan-error" {
		t.Errorf("unknown code Name = %s, want plan-error", row.Name)
	}
	if row.Action != ActionAbort {
		t.Errorf("catch-all Action = %s, want ActionAbort", row.Action)
	}
}

// TestPlanErrorRouteFor_EmptyCode: empty code (zero value) → catch-all.
func TestPlanErrorRouteFor_EmptyCode(t *testing.T) {
	t.Parallel()
	row, ok := PlanErrorRouteFor("")
	if !ok {
		t.Fatalf("empty code should match catch-all")
	}
	if row.Name != "plan-error" {
		t.Errorf("empty code Name = %s, want plan-error", row.Name)
	}
}
