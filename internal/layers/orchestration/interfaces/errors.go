package interfaces

import (
	"errors"

	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// Sentinel errors for the interfaces package. They are returned unwrapped
// from the constructor helpers (NewTaskSpec / NewTaskReport / AppendDissent
// / WithResource). Callers that need a canonical machine-readable code wrap
// these via sharederrors.WithCode — see orchtypes/errors.go for the wrap
// helpers and the 7xxx code range convention.
var (
	// ErrTaskSpecGoalEmpty is returned by NewTaskSpec when the goal argument
	// is the empty string. TaskSpec.Goal is the one required field and is
	// expected to be present in every down-link.
	ErrTaskSpecGoalEmpty = errors.New("interfaces: TaskSpec.Goal is empty")

	// ErrTaskSpecTraceIDEmpty is returned by NewTaskSpec when the auto-
	// generated TraceID would be empty (should never happen — defensive
	// only; surfacing here lets tests assert the invariant).
	ErrTaskSpecTraceIDEmpty = errors.New("interfaces: TaskSpec.TraceID is empty after auto-generate")

	// ErrTaskReportTraceIDEmpty is returned by NewTaskReport when the
	// supplied traceID is empty. TraceID is the cross-layer correlation key
	// and must always be set so reports can be tied back to their spec.
	ErrTaskReportTraceIDEmpty = errors.New("interfaces: TaskReport.TraceID is empty")

	// ErrDissentRejection is returned by AppendDissent when the entry has
	// an empty Reason. Reason is the load-bearing field for downstream
	// SkillMemory.SOP lookups, so we fail fast instead of silently storing
	// an entry that cannot be reasoned about.
	ErrDissentRejection = errors.New("interfaces: Dissent entry rejected: Reason empty")

	// ErrResourceInvalid is returned by WithResource when any numeric field
	// is negative. Negative tokens/time/steps would corrupt downstream
	// budget accounting, so we reject before construction.
	ErrResourceInvalid = errors.New("interfaces: Resource field invalid: tokens/time/step must be non-negative")
)

// Canonical wrap helpers. Codes follow the 7xxx range reserved for the
// orchestration domain (see orchtypes/errors.go). The wrap helpers live
// here so callers don't need to import orchtypes just to translate a
// sentinel into a Code+Message+Remediation triple.

func NewTaskSpecGoalEmptyError() *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_TASK_SPEC_GOAL_EMPTY_7100",
		"TaskSpec.Goal cannot be empty",
		ErrTaskSpecGoalEmpty,
	)
}

func NewTaskSpecTraceIDEmptyError() *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_TASK_SPEC_TRACE_ID_EMPTY_7101",
		"TaskSpec.TraceID cannot be empty after auto-generate",
		ErrTaskSpecTraceIDEmpty,
	)
}

func NewTaskReportTraceIDEmptyError() *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_TASK_REPORT_TRACE_ID_EMPTY_7102",
		"TaskReport.TraceID cannot be empty",
		ErrTaskReportTraceIDEmpty,
	)
}

func NewDissentRejectionError() *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_DISSENT_REJECTION_7103",
		"Dissent entry rejected: Reason empty",
		ErrDissentRejection,
	)
}

func NewResourceInvalidError() *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_RESOURCE_INVALID_7104",
		"Resource field invalid: tokens/time/step must be non-negative",
		ErrResourceInvalid,
	)
}