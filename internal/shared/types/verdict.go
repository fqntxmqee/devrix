package types

import (
	"encoding/json"
	"fmt"
)

// VerdictKind classifies the output of a Verifier sub-agent. The 4 kinds
// form a strict ordinal scale (Pass < Partial < Indeterminate < Fail) that
// AggregateVerdicts can rank against.
//
// Lifecycle:
//
//	Pass          — criteria fully satisfied
//	Partial       — criteria partially satisfied (need human review)
//	Indeterminate — verifier abstained (parse failure / no consensus)
//	Fail          — criteria violated
//
// Promoted to shared/types (2026-06-23, DM-20260623-002 Phase 4 PR-D1) to
// mirror the Phase 3 SideEffectStatus precedent: typed enums shared across
// orchtypes/workmodel/turn avoid cyclic imports and keep D5 dashboard
// filters uniform. The cross-domain D5 dashboards already key on this
// type's wire format string so the move is wire-compatible.
type VerdictKind uint8

const (
	// VerdictPass — criteria fully satisfied.
	VerdictPass VerdictKind = iota
	// VerdictPartial — criteria partially satisfied.
	VerdictPartial
	// VerdictIndeterminate — verifier abstained.
	VerdictIndeterminate
	// VerdictFail — criteria violated.
	VerdictFail
)

// String returns the wire format name (lowercase). Unknown kinds return a
// debug-formatted integer so logs stay grep-able.
func (k VerdictKind) String() string {
	switch k {
	case VerdictPass:
		return "pass"
	case VerdictPartial:
		return "partial"
	case VerdictIndeterminate:
		return "indeterminate"
	case VerdictFail:
		return "fail"
	default:
		return fmt.Sprintf("VerdictKind(%d)", uint8(k))
	}
}

// ParseVerdictKind reverses String() to recover the enum value from a wire
// payload. Returns an error on unknown input (fail-fast, mirroring
// ParseArtifactKind).
func ParseVerdictKind(s string) (VerdictKind, error) {
	switch s {
	case "pass":
		return VerdictPass, nil
	case "partial":
		return VerdictPartial, nil
	case "indeterminate":
		return VerdictIndeterminate, nil
	case "fail":
		return VerdictFail, nil
	default:
		return 0, fmt.Errorf("types: unknown VerdictKind %q", s)
	}
}

// MarshalJSON encodes the kind as its String() form. The wire format is the
// string, not the integer, so dashboards / D5 log filters stay
// human-readable.
func (k VerdictKind) MarshalJSON() ([]byte, error) {
	return json.Marshal(k.String())
}

// UnmarshalJSON accepts the wire format produced by MarshalJSON. An empty
// string decodes to the zero value (VerdictPass) for backward compatibility
// with v2 verifier outputs that did not carry Kind.
func (k *VerdictKind) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == "" {
		*k = VerdictPass
		return nil
	}
	parsed, err := ParseVerdictKind(s)
	if err != nil {
		return err
	}
	*k = parsed
	return nil
}