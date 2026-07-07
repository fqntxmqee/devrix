// Package plan: PlanErrorDecisionTable — T70 mapping (DM-20260707-001 PR-F).
//
// The 16 FieldRejectionCodes (10 field-validator + 6 parse-reject) are routed
// to a PlanErrorAction by the PlanErrorDecisionTable. Each row pairs a Code
// range with one of 4 actions:
//
//	ActionRetry     — feed the rejection back to the LLM as a hint (Retryable
//	                  under the retry budget).
//	ActionDecompose — fall back to DecomposeIntoChildren (LLM produced a
//	                  shape the parser could not normalise).
//	ActionForcePlan — flag the next round as force_plan so the LLMPlanner
//	                  bypasses the FastPath. Tied to T63 force_plan signal.
//	ActionAbort     — emit PlanErrorDecision; Learn skips BayesianUpdate.
//
// The table has 11 named rows. 10 rows cover the field-validator codes
// (kind/steps/lineage/strength/persist/pp2-empty/pp2-op/pp2-field/blast/dag);
// the 11th row is the plan_error catch-all for any PARSE rejection (the
// 6 parse codes map to ActionRetry/ActionDecompose/ActionAbort by their
// Retryable flag, but they all share the plan_error audit label so the
// dashboard sees one bucket).
//
// Routing matrix:
//
//	| Code                                | Action          |
//	| ----------------------------------- | --------------- |
//	| CodeKindUnset                       | ActionAbort     | (categorical — no fix)
//	| CodeStepsEmpty                      | ActionDecompose | (≥2 sub-plans)
//	| CodeSourceObservationIDsEmpty       | ActionAbort     | (PP-4 lineage required)
//	| CodeStrengthOutOfRange              | ActionRetry     |
//	| CodePersistScopeInvalid             | ActionRetry     |
//	| CodeFailureCriteriaEmpty            | ActionDecompose | (PP-2 needs ≥1, designer must add)
//	| CodeFailureCriteriaInvalidOp        | ActionRetry     |
//	| CodeFailureCriteriaInvalidField     | ActionRetry     |
//	| CodeBlastRadiusExceeded             | ActionForcePlan | (FP1 — see policy.go)
//	| CodeDAGInvalid                      | ActionDecompose | (graph-level)
//
//	| CodeParseMalformedJSON              | ActionRetry     |
//	| CodeParseUnknownKind                | ActionRetry     |
//	| CodeParseMissingField               | ActionRetry     |
//	| CodeParseInvalidNumeric             | ActionRetry     |
//	| CodeParseInvalidEnum                | ActionRetry     |
//	| CodeParseInvalidAST                 | ActionDecompose | (semantic, LLM cannot fix)
//	| (anything else / unknown)           | ActionAbort     | (catch-all)
package plan

// PlanErrorAction enumerates the 4 routing actions a Plan rejection can
// trigger. The session orchestrator (item_pipeline.go:handlePlanError) reads
// this and dispatches accordingly.
type PlanErrorAction uint8

const (
	// ActionUnset is the zero value (no decision yet).
	ActionUnset PlanErrorAction = iota

	// ActionRetry — feed rejection hint back to the LLM (≤DefaultMaxRetries).
	ActionRetry

	// ActionDecompose — invoke DecomposeIntoChildren to split into sub-plans.
	ActionDecompose

	// ActionForcePlan — set the round-level force_plan flag (T63/T71). The
	// next Execute round routes through the LLMPlanner instead of the
	// fast-path optimiser.
	ActionForcePlan

	// ActionAbort — emit PlanErrorDecision (T69); Learn skips BayesianUpdate.
	ActionAbort
)

// String returns the human-readable action name (for logging + dashboards).
func (a PlanErrorAction) String() string {
	switch a {
	case ActionRetry:
		return "retry"
	case ActionDecompose:
		return "decompose"
	case ActionForcePlan:
		return "force_plan"
	case ActionAbort:
		return "abort"
	default:
		return "unset"
	}
}

// PlanErrorRoute is one row of the PlanErrorDecisionTable. Identifies a Code
// (or a Code "family" via the AllParseRejectionCodes bucket) and the Action
// to take. MaxRetries is the per-action retry cap; some actions (ActionAbort)
// cap at 0 (no retry allowed).
type PlanErrorRoute struct {
	Name       string
	Code       FieldRejectionCode // exact match (zero value = catch-all)
	CodeFamily string             // "field-validator" or "parse" — used for the catch-all
	Action     PlanErrorAction
	MaxRetries int
}

// AllRoutes returns the canonical 11-row routing table (10 named + 1 catch-all).
// The table is in stable declaration order — callers MUST iterate in order to
// ensure consistent audit / DecisionNode behaviour.
func AllRoutes() []PlanErrorRoute {
	return []PlanErrorRoute{
		// ----- 10 named rows: field-validator codes (DM-20260707-001 PR-F T66) -----
		{Name: "kind-unset", Code: CodeKindUnset, CodeFamily: "field-validator",
			Action: ActionAbort, MaxRetries: 0},
		{Name: "steps-empty", Code: CodeStepsEmpty, CodeFamily: "field-validator",
			Action: ActionDecompose, MaxRetries: 1},
		{Name: "lineage-empty", Code: CodeSourceObservationIDsEmpty, CodeFamily: "field-validator",
			Action: ActionAbort, MaxRetries: 0},
		{Name: "strength-range", Code: CodeStrengthOutOfRange, CodeFamily: "field-validator",
			Action: ActionRetry, MaxRetries: DefaultMaxRetries},
		{Name: "persist-scope", Code: CodePersistScopeInvalid, CodeFamily: "field-validator",
			Action: ActionRetry, MaxRetries: DefaultMaxRetries},
		{Name: "pp2-empty", Code: CodeFailureCriteriaEmpty, CodeFamily: "field-validator",
			Action: ActionDecompose, MaxRetries: 1},
		{Name: "pp2-op", Code: CodeFailureCriteriaInvalidOp, CodeFamily: "field-validator",
			Action: ActionRetry, MaxRetries: DefaultMaxRetries},
		{Name: "pp2-field", Code: CodeFailureCriteriaInvalidField, CodeFamily: "field-validator",
			Action: ActionRetry, MaxRetries: DefaultMaxRetries},
		{Name: "blast-radius", Code: CodeBlastRadiusExceeded, CodeFamily: "field-validator",
			Action: ActionForcePlan, MaxRetries: 0},
		{Name: "dag-invalid", Code: CodeDAGInvalid, CodeFamily: "field-validator",
			Action: ActionDecompose, MaxRetries: 1},

		// ----- 11th row: parse-code catch-all ("plan_error" path) -----
		// The catch-all covers ALL 6 parse codes; per-code retry/decompose
		// selection happens via PlanErrorRouteFor below (which looks at the
		// Retryable flag of each parse rejection).
		{Name: "plan-error", Code: "", CodeFamily: "parse",
			Action: ActionAbort, MaxRetries: 0},
	}
}

// PlanErrorRouteFor returns the most specific routing row for the given
// rejection code. The lookup walks AllRoutes() in order and returns the
// first match:
//
//   - exact code match (CodeKindUnset / CodeParseMalformedJSON / etc.)
//   - parse family match (any CodeParse*)
//   - catch-all ("plan-error" — ActionAbort)
//
// Returns (row, true) on hit, (PlanErrorRoute{}, false) if no route matches
// (impossible by construction — the catch-all always matches).
func PlanErrorRouteFor(code FieldRejectionCode) (PlanErrorRoute, bool) {
	if code == "" {
		return allRoutesCatchAll(), true
	}
	// First try exact match.
	for _, r := range AllRoutes() {
		if r.Code != "" && r.Code == code {
			return r, true
		}
	}
	// Then family bucket — parse codes that didn't match an exact row.
	if isParseCode(code) {
		return parseRouteFor(code), true
	}
	// Fallback to catch-all (shouldn't happen since the table has one).
	return allRoutesCatchAll(), true
}

// allRoutesCatchAll returns the canonical "plan-error" row.
func allRoutesCatchAll() PlanErrorRoute {
	for _, r := range AllRoutes() {
		if r.Name == "plan-error" {
			return r
		}
	}
	return PlanErrorRoute{Name: "plan-error", Action: ActionAbort}
}

// parseRouteFor maps a parse-time code to an action based on its Retryable
// flag (single source of truth). Coding here stays in sync with
// FeedbackForRejection in plan_retry_decompose.go.
func parseRouteFor(code FieldRejectionCode) PlanErrorRoute {
	hint := FeedbackForRejection(&PlanParseRejection{Reason: ParseRejectReason{Code: code}})
	r := PlanErrorRoute{Name: "parse:" + string(code), Action: ActionRetry, MaxRetries: DefaultMaxRetries}
	if !hint.Retryable {
		r.Action = ActionDecompose
		r.MaxRetries = 1
	}
	return r
}

// isParseCode reports whether c is one of the 6 parse codes.
func isParseCode(c FieldRejectionCode) bool {
	for _, pc := range AllParseRejectionCodes() {
		if pc == c {
			return true
		}
	}
	return false
}
