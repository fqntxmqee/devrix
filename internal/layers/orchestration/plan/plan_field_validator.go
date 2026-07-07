// Package plan: PlanFieldValidator + 8 sub-class rejections
// (DM-20260707-001 PR-F, T66).
//
// PlanFieldValidator decomposes the monolithic Plan.Validate() into 8 named
// sub-class rejections so the caller (DefaultPlanner, ItemPipelineRunner)
// can:
//
//  1. Map each rejection to a specific retry / fallback policy.
//  2. Emit precise telemetry counters per sub-class.
//  3. Drive the Decision mapping (T70: 10 → 11 rows including the plan_error
//     path) without resorting to errors.Is string matching.
//
// The 8 sub-classes cover the structural / PP-1 / PP-2 / PP-3 violation surface:
//
//	PLAN_KIND_8001     KindUnset
//	PLAN_STEPS_8010     StepsEmpty
//	PLAN_LINEAGE_8002   SourceObservationIDsEmpty
//	PLAN_STRENGTH_8011  StrengthOutOfRange
//	PLAN_PERSIST_8012   PersistScopeInvalid
//	PLAN_PP2_EMPTY_8020 FailureCriteriaEmpty
//	PLAN_PP2_OP_8021    FailureCriteriaInvalidOp
//	PLAN_PP2_FIELD_8022 FailureCriteriaInvalidField
//
// (The 3 BlastRadius axes — FileCount/APICallCount/TokenCost — share the
// PLAN_BLAST_8003 code but are reported individually in the audit log; see
// PlanFieldValidator.Audit for the axis breakdown.)
package plan

import (
	"fmt"

	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// FieldRejectionCode is the canonical code for each of the 8 sub-class
// rejections. Values match the existing SentinelError codes so callers can
// switch on the code without parsing the error message.
type FieldRejectionCode string

const (
	// CodeKindUnset — Plan.Kind is the zero value (not classified).
	CodeKindUnset FieldRejectionCode = "PLAN_KIND_8001"

	// CodeStepsEmpty — at least one Step required.
	CodeStepsEmpty FieldRejectionCode = "PLAN_STEPS_8010"

	// CodeSourceObservationIDsEmpty — lineage field empty (PP-4).
	CodeSourceObservationIDsEmpty FieldRejectionCode = "PLAN_LINEAGE_8002"

	// CodeStrengthOutOfRange — Strength outside [0, 1].
	CodeStrengthOutOfRange FieldRejectionCode = "PLAN_STRENGTH_8011"

	// CodePersistScopeInvalid — PersistScope is not transient/session/permanent.
	CodePersistScopeInvalid FieldRejectionCode = "PLAN_PERSIST_8012"

	// CodeFailureCriteriaEmpty — PP-2 falsifiability requires ≥1 criterion.
	CodeFailureCriteriaEmpty FieldRejectionCode = "PLAN_PP2_EMPTY_8020"

	// CodeFailureCriteriaInvalidOp — Op is not in whitelist.
	CodeFailureCriteriaInvalidOp FieldRejectionCode = "PLAN_PP2_OP_8021"

	// CodeFailureCriteriaInvalidField — Field cannot be observed.
	CodeFailureCriteriaInvalidField FieldRejectionCode = "PLAN_PP2_FIELD_8022"

	// CodeBlastRadiusExceeded — PP-3 violation. The Audit field carries the
	// offending axis (FileCount/APICallCount/TokenCost); the code is shared.
	CodeBlastRadiusExceeded FieldRejectionCode = "PLAN_BLAST_8003"

	// CodeDAGInvalid — multi-intent DAG failed dag_validator. New in PR-F.
	CodeDAGInvalid FieldRejectionCode = "PLAN_DAG_8030"
)

// AllFieldRejectionCodes is the stable list of all 8+1 sub-class codes. Useful
// for tests + dashboards that want to enumerate the full taxonomy.
func AllFieldRejectionCodes() []FieldRejectionCode {
	return []FieldRejectionCode{
		CodeKindUnset,
		CodeStepsEmpty,
		CodeSourceObservationIDsEmpty,
		CodeStrengthOutOfRange,
		CodePersistScopeInvalid,
		CodeFailureCriteriaEmpty,
		CodeFailureCriteriaInvalidOp,
		CodeFailureCriteriaInvalidField,
		CodeBlastRadiusExceeded,
		CodeDAGInvalid,
	}
}

// PlanFieldAudit is the structured audit entry the validator emits. The
// caller (DefaultPlanner / ItemPipelineRunner) attaches this to the audit
// log row + telemetry counter.
type PlanFieldAudit struct {
	// Code is the canonical sub-class code (one of 8 + DAG).
	Code FieldRejectionCode

	// Field is the human-readable field path (e.g. "Steps" / "Strength" /
	// "FailureCriteria[0].Op"). Empty when the rejection is structural.
	Field string

	// Axis is the BlastRadius axis name when Code == CodeBlastRadiusExceeded
	// ("FileCount" / "APICallCount" / "TokenCost"). Empty otherwise.
	Axis string

	// Observed / Limit are populated for out-of-range violations. Both 0
	// when not applicable.
	Observed int
	Limit    int

	// Message is a free-form human-readable detail.
	Message string
}

// String renders the audit for logging.
func (a PlanFieldAudit) String() string {
	if a.Axis != "" {
		return fmt.Sprintf("%s field=%s axis=%s observed=%d limit=%d msg=%q",
			a.Code, a.Field, a.Axis, a.Observed, a.Limit, a.Message)
	}
	return fmt.Sprintf("%s field=%s msg=%q", a.Code, a.Field, a.Message)
}

// PlanFieldValidator is the structured validator that returns BOTH the
// SentinelError (for callers that bubble up the error) AND the PlanFieldAudit
// (for telemetry / Decision mapping).
//
// Construct via NewPlanFieldValidator(opts). The zero value is NOT usable
// because ValidateOpts defaults need to be applied.
type PlanFieldValidator struct {
	opts ValidateOpts
}

// NewPlanFieldValidator constructs a validator with the given thresholds.
// Zero-value ValidateOpts falls back to DefaultMax* constants.
func NewPlanFieldValidator(opts ValidateOpts) *PlanFieldValidator {
	return &PlanFieldValidator{opts: opts}
}

// Validate runs the 8-sub-class check sequence and returns the first
// rejection as a SentinelError (with the wrapped underlying error) AND the
// PlanFieldAudit (for callers that want the structured breakdown).
//
// Returns (nil, PlanFieldAudit{}) when the Plan is dispatch-ready.
func (v *PlanFieldValidator) Validate(p *Plan) (*sharederrors.SentinelError, PlanFieldAudit) {
	if p == nil {
		audit := PlanFieldAudit{
			Code:    CodeKindUnset,
			Field:   "Plan",
			Message: "Plan is nil",
		}
		return sharederrors.WithCode(string(audit.Code), audit.Message, ErrPlanKindUnset), audit
	}

	// Sub-class 1: KindUnset
	if !p.Kind.IsKnown() {
		audit := PlanFieldAudit{Code: CodeKindUnset, Field: "Kind", Message: "Plan.Kind is unset"}
		return NewPlanKindUnsetError(), audit
	}

	// Sub-class 2: StepsEmpty
	if len(p.Steps) == 0 {
		audit := PlanFieldAudit{Code: CodeStepsEmpty, Field: "Steps", Message: "Steps must be non-empty"}
		return sharederrors.WithCode(string(audit.Code), audit.Message, ErrPlanStepsEmpty), audit
	}

	// Sub-class 3: SourceObservationIDsEmpty
	if len(p.SourceObservationIDs) == 0 {
		audit := PlanFieldAudit{
			Code:    CodeSourceObservationIDsEmpty,
			Field:   "SourceObservationIDs",
			Message: "SourceObservationIDs is empty (PP-4 lineage required)",
		}
		return NewPlanSourceObservationIDsRequiredError(), audit
	}

	// Sub-class 4: StrengthOutOfRange
	if p.Strength < 0 || p.Strength > 1 {
		audit := PlanFieldAudit{
			Code:    CodeStrengthOutOfRange,
			Field:   "Strength",
			Message: fmt.Sprintf("Strength=%g out of [0, 1] range", p.Strength),
		}
		return sharederrors.WithCode(string(audit.Code), audit.Message, ErrPlanStrengthOutOfRange), audit
	}

	// Sub-class 5: PersistScopeInvalid
	if !p.BlastRadius.PersistScope.Valid() {
		audit := PlanFieldAudit{
			Code:    CodePersistScopeInvalid,
			Field:   "BlastRadius.PersistScope",
			Message: fmt.Sprintf("PersistScope=%q is not one of transient/session/permanent", p.BlastRadius.PersistScope),
		}
		return sharederrors.WithCode(string(audit.Code), audit.Message, ErrPlanPersistScopeInvalid), audit
	}

	// Sub-class 6: FailureCriteriaEmpty
	if len(p.FailureCriteria) == 0 {
		audit := PlanFieldAudit{
			Code:    CodeFailureCriteriaEmpty,
			Field:   "FailureCriteria",
			Message: "FailureCriteria is empty (PP-2 falsifiability requires ≥1 criterion)",
		}
		return sharederrors.WithCode(string(audit.Code), audit.Message, ErrPlanFailureCriteriaEmpty), audit
	}

	// Sub-class 7 + 8: FailureCriteria.Op whitelist + Field observability.
	for i, fc := range p.FailureCriteria {
		if !isOpAllowed(fc.Op) {
			audit := PlanFieldAudit{
				Code:    CodeFailureCriteriaInvalidOp,
				Field:   fmt.Sprintf("FailureCriteria[%d].Op", i),
				Message: fmt.Sprintf("Op=%q not in whitelist (eq/ne/gt/lt/in/contains)", fc.Op),
			}
			return sharederrors.WithCode(string(audit.Code), audit.Message, ErrPlanFailureCriteriaInvalidOp), audit
		}
		if !isFieldObservable(fc.Field) {
			audit := PlanFieldAudit{
				Code:    CodeFailureCriteriaInvalidField,
				Field:   fmt.Sprintf("FailureCriteria[%d].Field", i),
				Message: fmt.Sprintf("Field=%q not in observable set", fc.Field),
			}
			return sharederrors.WithCode(string(audit.Code), audit.Message, ErrPlanFailureCriteriaInvalidField), audit
		}
	}

	// PP-3: BlastRadius limits — code shared (CodeBlastRadiusExceeded) but
	// axis differs in the Audit. Caller distinguishes by audit.Axis.
	if p.BlastRadius.FileCount > v.opts.fileLimit() {
		audit := PlanFieldAudit{
			Code: CodeBlastRadiusExceeded, Field: "BlastRadius.FileCount", Axis: "FileCount",
			Observed: p.BlastRadius.FileCount, Limit: v.opts.fileLimit(),
			Message: "PP-3 violation",
		}
		return NewPlanBlastRadiusExceededError(audit.Axis, audit.Observed, audit.Limit), audit
	}
	if p.BlastRadius.APICallCount > v.opts.apiLimit() {
		audit := PlanFieldAudit{
			Code: CodeBlastRadiusExceeded, Field: "BlastRadius.APICallCount", Axis: "APICallCount",
			Observed: p.BlastRadius.APICallCount, Limit: v.opts.apiLimit(),
			Message: "PP-3 violation",
		}
		return NewPlanBlastRadiusExceededError(audit.Axis, audit.Observed, audit.Limit), audit
	}
	if p.BlastRadius.TokenCost > v.opts.tokenLimit() {
		audit := PlanFieldAudit{
			Code: CodeBlastRadiusExceeded, Field: "BlastRadius.TokenCost", Axis: "TokenCost",
			Observed: p.BlastRadius.TokenCost, Limit: v.opts.tokenLimit(),
			Message: "PP-3 violation",
		}
		return NewPlanBlastRadiusExceededError(audit.Axis, audit.Observed, audit.Limit), audit
	}

	// DAG (DM-20260707-001 PR-F): when DAG is set, validate via dag_validator.
	if p.DAG != nil {
		if err := validateDAG(p.DAG, v.opts.dagOpts()); err != nil {
			audit := PlanFieldAudit{
				Code: CodeDAGInvalid, Field: "DAG", Message: err.Error(),
			}
			return sharederrors.WithCode(string(audit.Code), audit.Message, err), audit
		}
	}

	return nil, PlanFieldAudit{}
}