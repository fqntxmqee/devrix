package orchtypes

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/devrix/devrix/internal/shared/types"
)

// SideEffectStatus is a type alias re-exported from shared/types so that
// UncertaintyCoord (Phase 2 PR-A1) and Artifact (Phase 3 PR-C1) can share
// the same wire format. The concrete enum + IsTerminal/NeedsAttention
// helpers live in shared/types/execute.go.
type SideEffectStatus = types.SideEffectStatus

const (
	SideEffectNone       = types.SideEffectNone
	SideEffectUnknown    = types.SideEffectUnknown
	SideEffectInflight   = types.SideEffectInflight
	SideEffectCommitted  = types.SideEffectCommitted
	SideEffectRolledBack = types.SideEffectRolledBack
)

// VerdictKind is a type alias re-exported from shared/types so that
// UncertaintyCoord (Phase 2 PR-A1) and Verdict (Phase 4 PR-D1) can share
// the same wire format. The concrete enum + String/Parse live in
// shared/types/verdict.go.
type VerdictKind = types.VerdictKind

const (
	VerdictPass          = types.VerdictPass
	VerdictPartial       = types.VerdictPartial
	VerdictIndeterminate = types.VerdictIndeterminate
	VerdictFail          = types.VerdictFail
)

// UncertaintyCoord is the per-item uncertainty value attached to Plans and
// propagated through Execute/Verify. Phase 1 introduced a float-only
// representation; Phase 2 extends it with provenance + side-effect coupling
// so the value can be reconstructed at any point in the pipeline.
type UncertaintyCoord struct {
	Value            float64         `json:"value"` // [0,1]
	Confidence       float64         `json:"confidence,omitempty"`
	Reason           string          `json:"reason,omitempty"`
	FromVerifier     bool            `json:"from_verifier,omitempty"`
	SideEffectStatus SideEffectStatus `json:"side_effect_status,omitempty"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

// NewUncertaintyCoord constructs a coord with the given value, clamped to
// [0,1].
func NewUncertaintyCoord(value float64) UncertaintyCoord {
	return UncertaintyCoord{
		Value:     clamp01Float(value, 0.5),
		UpdatedAt: time.Now(),
	}
}

// FromVerifierTyped projects a typed VerdictKind into an UncertaintyCoord.
// Verdict kind maps to coord value as:
//
//	Pass          → 0.0  (low uncertainty, the plan worked)
//	Partial       → 0.4
//	Indeterminate → 0.7  (high uncertainty, verifier abstained)
//	Fail          → 0.9
//
// SystemAnomaly=true forces a clamped 0.95 to signal orchestrator-level
// distrust.
//
// Unknown verdict kinds are FAIL-FAST: returns
// (UncertaintyCoord{}, NewUncertaintyCoordInvalidVerdictKindError(kind))
// so the ORCH_COORD_VERDICT_7004 error code fires and upstream typos
// surface immediately rather than being silently coerced to the 0.5
// neutral default.
//
// This is the canonical entry point for Verify→Coord wiring. Callers
// that have a string kind (e.g. parsed from wire payloads) should
// convert via types.ParseVerdictKind first.
func FromVerifierTyped(kind VerdictKind, confidence float64, reason string, systemAnomaly bool) (UncertaintyCoord, error) {
	base := 0.5
	switch kind {
	case VerdictPass:
		base = 0.0
	case VerdictPartial:
		base = 0.4
	case VerdictIndeterminate:
		base = 0.7
	case VerdictFail:
		base = 0.9
	default:
		return UncertaintyCoord{}, NewUncertaintyCoordInvalidVerdictKindError(kind.String())
	}
	if systemAnomaly {
		base = 0.95
	}
	return UncertaintyCoord{
		Value:        clamp01Float(base, 0.5),
		Confidence:   clamp01Float(confidence, 0.5),
		Reason:       reason,
		FromVerifier: true,
		UpdatedAt:    time.Now(),
	}, nil
}

// WithValue returns a copy with the new value (clamped). UpdatedAt is bumped.
func (u UncertaintyCoord) WithValue(v float64) UncertaintyCoord {
	u.Value = clamp01Float(v, 0.5)
	u.UpdatedAt = time.Now()
	return u
}

// WithReason returns a copy with the new reason.
func (u UncertaintyCoord) WithReason(r string) UncertaintyCoord {
	u.Reason = r
	return u
}

// WithSideEffect returns a copy with the given SideEffectStatus. This is
// how Phase 3 propagate "the plan's side effect state" back into the coord
// so downstream ObserveNode can adjust strength.
func (u UncertaintyCoord) WithSideEffect(s SideEffectStatus) UncertaintyCoord {
	u.SideEffectStatus = s
	u.UpdatedAt = time.Now()
	return u
}

// Validate ensures basic shape correctness.
func (u UncertaintyCoord) Validate() error {
	if u.Value < 0 || u.Value > 1 {
		return NewUncertaintyCoordValueOutOfRangeError(u.Value)
	}
	if u.Confidence < 0 || u.Confidence > 1 {
		return fmt.Errorf("orchtypes: UncertaintyCoord.Confidence %.3f: %w",
			u.Confidence, ErrUncertaintyCoordConfidenceOutOfRange)
	}
	return nil
}

// MarshalJSON keeps the original Phase 1 wire shape (just "value") plus
// the new optional fields. We use omitempty on all optional fields so
// older consumers that only know {value} keep working.
func (u UncertaintyCoord) MarshalJSON() ([]byte, error) {
	type alias UncertaintyCoord
	return json.Marshal(alias(u))
}

// UnmarshalJSON is the symmetric counterpart. New fields default to their
// zero values when absent in legacy payloads.
func (u *UncertaintyCoord) UnmarshalJSON(data []byte) error {
	type alias UncertaintyCoord
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*u = UncertaintyCoord(a)
	if u.UpdatedAt.IsZero() {
		u.UpdatedAt = time.Now()
	}
	return nil
}

// IsColdStart returns true if this coord has no provenance and a default
// value (0.5).
func (u UncertaintyCoord) IsColdStart() bool {
	return !u.FromVerifier && u.Reason == "" && math.Abs(u.Value-0.5) < 1e-9
}

// Equal performs a tolerant equality check ignoring UpdatedAt (which is
// wall-clock and not stable across marshalling).
func (u UncertaintyCoord) Equal(other UncertaintyCoord) bool {
	return u.Value == other.Value &&
		u.Confidence == other.Confidence &&
		u.Reason == other.Reason &&
		u.FromVerifier == other.FromVerifier &&
		u.SideEffectStatus == other.SideEffectStatus
}
