package types

import (
	"encoding/json"
	"fmt"
)

// ArtifactKind classifies the output of an Execute channel run. The 4 kinds
// are mapped 1:1 to the 4 PlanKind (commit/protocol/scenario/exploration) in
// PR-C2 once plan.PlanKind lands. Until then, callers must set Kind
// explicitly on Artifact construction.
//
// Promoted to shared/types (2026-06-23, DM-20260625-001 PR-C1) to break a
// cyclic import: orchtypes → workmodel → wavescheduler → orchtypes. The
// cross-domain D5 dashboards already key on this type's wire format string
// so the move is wire-compatible.
type ArtifactKind uint8

const (
	// ArtifactStateChangeCert is produced by CommitChannel — a real
	// side-effect (DB write / HTTP POST / file create) that must be
	// compensated on failure.
	ArtifactStateChangeCert ArtifactKind = iota
	// ArtifactResponseRecord is produced by ProtocolChannel — a structured
	// response to a multi-step protocol (e.g. login → fetch → parse).
	ArtifactResponseRecord
	// ArtifactProbeReport is produced by ScenarioChannel — a read-only probe
	// result used to evaluate a scenario plan branch.
	ArtifactProbeReport
	// ArtifactExperimentData is produced by ExplorationChannel — exploratory
	// output feeding back into a learning loop.
	ArtifactExperimentData
)

// String returns the wire format name (snake_case). Unknown kinds return a
// debug-formatted integer so logs stay grep-able.
func (k ArtifactKind) String() string {
	switch k {
	case ArtifactStateChangeCert:
		return "state_change_cert"
	case ArtifactResponseRecord:
		return "response_record"
	case ArtifactProbeReport:
		return "probe_report"
	case ArtifactExperimentData:
		return "experiment_data"
	default:
		return fmt.Sprintf("ArtifactKind(%d)", uint8(k))
	}
}

// ParseArtifactKind reverses String() to recover the enum value from a wire
// payload. Returns an error on unknown input.
func ParseArtifactKind(s string) (ArtifactKind, error) {
	switch s {
	case "state_change_cert":
		return ArtifactStateChangeCert, nil
	case "response_record":
		return ArtifactResponseRecord, nil
	case "probe_report":
		return ArtifactProbeReport, nil
	case "experiment_data":
		return ArtifactExperimentData, nil
	default:
		return 0, fmt.Errorf("types: unknown ArtifactKind %q", s)
	}
}

// MarshalJSON encodes the kind as its String() form. The wire format is the
// string, not the integer, so dashboards / D5 log filters stay
// human-readable.
func (k ArtifactKind) MarshalJSON() ([]byte, error) {
	return json.Marshal(k.String())
}

// UnmarshalJSON accepts the wire format produced by MarshalJSON. An empty
// string decodes to the zero value (ArtifactStateChangeCert) for backward
// compatibility with v2 Artifacts that did not carry Kind.
func (k *ArtifactKind) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == "" {
		*k = ArtifactStateChangeCert
		return nil
	}
	parsed, err := ParseArtifactKind(s)
	if err != nil {
		return err
	}
	*k = parsed
	return nil
}

// SideEffectStatus tracks the side-effect state of an Execute channel run.
// It is intentionally a string alias (not a uint8) so that UncertaintyCoord
// (Phase 2 PR-A1) and Artifact (Phase 3 PR-C1) can share the same wire
// format and D5 dashboards can filter uniformly across both call sites.
//
// Lifecycle:
//
//	None       — tool has no side effect
//	Inflight   — tool invoked, response not yet confirmed
//	Committed  — tool side effect confirmed (e.g. HTTP 200 + ack)
//	RolledBack — side effect was compensated
//	Unknown    — state could not be determined (e.g. network partition)
type SideEffectStatus string

const (
	// SideEffectNone — the tool is pure (no observable side effect).
	SideEffectNone SideEffectStatus = "none"
	// SideEffectUnknown — state could not be determined. Treat as
	// NeedsAttention=true until the verifier can reclassify.
	SideEffectUnknown SideEffectStatus = "unknown"
	// SideEffectInflight — tool invoked, response not yet confirmed.
	// StrategyDecider (PR-C3) MUST treat this as StrategyAskNow.
	SideEffectInflight SideEffectStatus = "inflight"
	// SideEffectCommitted — side effect observed and confirmed.
	SideEffectCommitted SideEffectStatus = "committed"
	// SideEffectRolledBack — side effect was successfully compensated.
	SideEffectRolledBack SideEffectStatus = "rolled_back"
)

// IsTerminal reports whether the status represents a stable end-state that
// downstream consumers (Verify / Learn) can safely act on.
func (s SideEffectStatus) IsTerminal() bool {
	return s == SideEffectNone || s == SideEffectCommitted || s == SideEffectRolledBack
}

// NeedsAttention reports whether the status represents an indeterminate state
// that the StrategyDecider must surface to a human before continuing.
func (s SideEffectStatus) NeedsAttention() bool {
	return s == SideEffectUnknown || s == SideEffectInflight
}

// SideEffectDetail captures the evidence behind a non-trivial side-effect
// transition (Inflight / Committed / RolledBack). For SideEffectNone the
// detail is usually nil; for SideEffectUnknown it is best-effort.
type SideEffectDetail struct {
	IdempotencyKey   string `json:"idempotency_key"`
	SentAt           int64  `json:"sent_at"` // unix nano
	ConfirmedAt      int64  `json:"confirmed_at,omitempty"`
	CompensationLog  string `json:"compensation_log,omitempty"`
	CompensationTool string `json:"compensation_tool,omitempty"`
}
