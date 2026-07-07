package plan

import (
	"errors"
	"fmt"

	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// Sentinel errors for Plan validation. Pattern: pkg/struct/specifics.
var (
	// ErrPlanKindUnset: Plan.Kind is the zero value (not classified).
	ErrPlanKindUnset = errors.New("plan: Kind is unset (must be one of commitment/protocol/scenario/exploration)")

	// ErrPlanSourceObservationIDsRequired: lineage field empty. Required by
	// Phase 4 Verify for reverse-lookup.
	ErrPlanSourceObservationIDsRequired = errors.New("plan: SourceObservationIDs must be non-empty (required for Phase 4 Verify reverse-lookup)")

	// ErrPlanStrengthOutOfRange: Strength must be in [0, 1].
	ErrPlanStrengthOutOfRange = errors.New("plan: Strength must be in [0, 1]")

	// ErrPlanStrengthMismatch: Strength > min(BusinessObservation.Strength)
	// (PP-1 strength match).
	ErrPlanStrengthMismatch = errors.New("plan: Strength exceeds min(BusinessObservation.Strength) — PP-1 violation")

	// ErrPlanFailureCriteriaEmpty: PP-2 falsifiability requires at least one
	// criterion.
	ErrPlanFailureCriteriaEmpty = errors.New("plan: FailureCriteria must be non-empty (PP-2 falsifiability)")

	// ErrPlanFailureCriteriaInvalidOp: Op is not in whitelist (eq/ne/gt/lt/in/contains).
	ErrPlanFailureCriteriaInvalidOp = errors.New("plan: FailureCriteria.Op is not in whitelist (eq/ne/gt/lt/in/contains)")

	// ErrPlanFailureCriteriaInvalidField: Field cannot be observed in ExecutionEvidence.
	ErrPlanFailureCriteriaInvalidField = errors.New("plan: FailureCriteria.Field is not observable in ExecutionEvidence")

	// ErrPlanBlastRadiusExceeded: PP-3 violation — BlastRadius exceeds config limits.
	ErrPlanBlastRadiusExceeded = errors.New("plan: BlastRadius exceeds config limits — PP-3 violation")

	// ErrPlanStepsEmpty: at least one Step required.
	ErrPlanStepsEmpty = errors.New("plan: Steps must be non-empty")

	// ErrPlanPersistScopeInvalid: PersistScope is not recognized.
	ErrPlanPersistScopeInvalid = errors.New("plan: PersistScope is not one of transient/session/permanent")
)

// FailureCriterionOpWhitelist is the set of valid Op values. Validator
// enforces this against each FailureCriterion.Op.
var FailureCriterionOpWhitelist = []string{"eq", "ne", "gt", "lt", "in", "contains"}

// ObservableFailureCriterionFields is the set of Field paths that can be
// extracted from ExecutionEvidence (see Phase 3 PR-C5). PP-2 validation
// rejects any FailureCriterion.Field outside this set.
var ObservableFailureCriterionFields = []string{
	"exit_code",
	"diff_hash",
	"api_status",
	"duration_ms",
	"output_match",
}

// Configurable limits for PP-3 enforcement. Caller may override via ValidateOpts.
const (
	DefaultMaxBlastRadiusFileCount    = 50
	DefaultMaxBlastRadiusAPICallCount = 20
	DefaultMaxBlastRadiusTokenCost    = 100_000
)

// ValidateOpts tunes Plan.Validate() thresholds. Zero-value falls back to
// the DefaultMax* constants above.
type ValidateOpts struct {
	MaxBlastRadiusFileCount    int
	MaxBlastRadiusAPICallCount int
	MaxBlastRadiusTokenCost    int
	// DAG caps (DM-20260707-001 PR-A1 T13). When non-zero, override
	// validateDAG thresholds. Default is 10 nodes / 8 fan-out.
	MaxDAGNodes int
	MaxFanOut   int
}

func (o ValidateOpts) dagOpts() PlanDAGValidationOpts {
	return PlanDAGValidationOpts{
		MaxDAGNodes: o.MaxDAGNodes,
		MaxFanOut:   o.MaxFanOut,
	}
}

func (o ValidateOpts) fileLimit() int {
	if o.MaxBlastRadiusFileCount > 0 {
		return o.MaxBlastRadiusFileCount
	}
	return DefaultMaxBlastRadiusFileCount
}

func (o ValidateOpts) apiLimit() int {
	if o.MaxBlastRadiusAPICallCount > 0 {
		return o.MaxBlastRadiusAPICallCount
	}
	return DefaultMaxBlastRadiusAPICallCount
}

func (o ValidateOpts) tokenLimit() int {
	if o.MaxBlastRadiusTokenCost > 0 {
		return o.MaxBlastRadiusTokenCost
	}
	return DefaultMaxBlastRadiusTokenCost
}

// NewPlanKindUnsetError returns a SentinelError wrapping ErrPlanKindUnset.
func NewPlanKindUnsetError() *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"PLAN_KIND_8001",
		"plan: Kind is unset (zero value); caller must classify before constructing Plan",
		ErrPlanKindUnset,
	)
}

// NewPlanSourceObservationIDsRequiredError returns a SentinelError wrapping
// ErrPlanSourceObservationIDsRequired. Used when Plan is constructed without
// lineage — Phase 4 Verify cannot reverse-lookup without these IDs.
func NewPlanSourceObservationIDsRequiredError() *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"PLAN_LINEAGE_8002",
		"plan: SourceObservationIDs is required for Phase 4 Verify reverse-traceability",
		ErrPlanSourceObservationIDsRequired,
	)
}

// NewPlanBlastRadiusExceededError returns a SentinelError with the offending
// axis name + observed/limit values for log triage.
func NewPlanBlastRadiusExceededError(axis string, observed, limit int) *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"PLAN_BLAST_8003",
		fmt.Sprintf("plan: BlastRadius.%s (%d) exceeds limit (%d) — PP-3 violation", axis, observed, limit),
		fmt.Errorf("%w: %s observed=%d limit=%d", ErrPlanBlastRadiusExceeded, axis, observed, limit),
	)
}