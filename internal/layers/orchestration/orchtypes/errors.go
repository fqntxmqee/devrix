package orchtypes

import (
	"errors"
	"fmt"

	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// Sentinel errors for the orchtypes package. Use these as the unwrapped
// inner error; wrap with sharederrors.WithCode when returning to callers
// so the canonical code travels with the error.
var (
	ErrObservationIDRequired          = errors.New("orchtypes: observation ID required")
	ErrObservationStrengthOutOfRange  = errors.New("orchtypes: observation strength out of [0,1]")
	ErrObservationDetectedAtRequired  = errors.New("orchtypes: observation DetectedAt required")
	ErrObservationUnknownCategory     = errors.New("orchtypes: observation unknown category")
	ErrObservationPayloadRequired     = errors.New("orchtypes: observation payload required")
	ErrObservationPayloadInvalid      = errors.New("orchtypes: observation payload invalid")

	ErrUncertaintyReportPartitionInvariant = errors.New("orchtypes: uncertainty report partition invariant violated")
	ErrUncertaintyReportSessionIDRequired  = errors.New("orchtypes: uncertainty report session_id required")

	ErrUncertaintyCoordValueOutOfRange      = errors.New("orchtypes: uncertainty coord value out of [0,1]")
	ErrUncertaintyCoordConfidenceOutOfRange = errors.New("orchtypes: uncertainty coord confidence out of [0,1]")
	ErrUncertaintyCoordInvalidVerdictKind   = errors.New("orchtypes: uncertainty coord invalid verdict kind")

	// Phase 7 PR-7.2 (D7-S13-A48-T04): ProcessRequest validation.
	ErrProcessRequestSessionIDEmpty   = errors.New("orchtypes: ProcessRequest SessionID is empty")
	ErrProcessRequestMessageEmpty     = errors.New("orchtypes: ProcessRequest Message is empty")
	ErrProcessRequestInvalidTrackMode = errors.New("orchtypes: ProcessRequest TrackMode must be \"developer\", \"operator\", or empty")
)

// Wrap helpers — produce a *sharederrors.SentinelError with a stable code.
// Codes live in the 7000-7999 range (orchestration domain). Adjust as the
// taxonomy evolves; the codes are the canonical machine-readable handle.
func NewObservationStrengthOutOfRangeError(strength float64) *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_OBS_STRENGTH_7001",
		fmt.Sprintf("observation strength %.2f out of [0,1]", strength),
		ErrObservationStrengthOutOfRange,
	)
}

func NewUncertaintyReportPartitionInvariantError() *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_REPORT_PARTITION_7002",
		"partition invariant: business + system must equal observations",
		ErrUncertaintyReportPartitionInvariant,
	)
}

func NewUncertaintyCoordValueOutOfRangeError(v float64) *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_COORD_VALUE_7003",
		fmt.Sprintf("uncertainty coord value %.3f out of [0,1]", v),
		ErrUncertaintyCoordValueOutOfRange,
	)
}

func NewUncertaintyCoordInvalidVerdictKindError(kind string) *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_COORD_VERDICT_7004",
		fmt.Sprintf("unknown verdict kind %q for FromVerifier", kind),
		ErrUncertaintyCoordInvalidVerdictKind,
	)
}
