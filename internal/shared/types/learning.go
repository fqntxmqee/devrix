package types

import (
	"encoding/json"
	"fmt"
)

// LearningClass classifies the kind of LearningAsset produced by the Learn
// node. The 5 classes form a strict ordinal scale (SOP=5 > Protocol=4 >
// Knowledge=3 > Conclusion=2 > Pending=1) that AssetBuilder routes to 3
// Memory channels (Skill / Feedback / Scheduled).
//
// Lifecycle:
//
//	SOP         — Standard Operating Procedure (★5)
//	Protocol    — Protocol (★4)
//	Knowledge   — Knowledge (★3)
//	Conclusion  — Conclusion (★2)
//	Pending     — Pending retry (★1, ⭐new — VerdictIndeterminate)
//
// Promoted to shared/types (2026-06-23, DM-20260623-003 Phase 5 PR-E1) to
// mirror the Phase 3 SideEffectStatus (D7-S9-A25-T04) + Phase 4 VerdictKind
// (D7-S10-A32-T01) precedent: typed enums shared across orchtypes/learn/turn
// avoid cyclic imports and keep D5 dashboard filters uniform.
type LearningClass uint8

const (
	// LearningUnknown — reserved zero value; MUST be rejected by
	// ParseLearningClass and factory functions.
	LearningUnknown LearningClass = iota

	// LearningSOP — Standard Operating Procedure (★5).
	// Source: ComplianceVerdict (Phase 4 doc 45 §4.4).
	// Routed to: SkillMemory.
	LearningSOP

	// LearningProtocol — Protocol (★4).
	// Source: TimelinessVerdict.
	// Routed to: SkillMemory.
	LearningProtocol

	// LearningKnowledge — Knowledge (★3).
	// Source: RootCauseVerdict.
	// Routed to: FeedbackMemory.
	LearningKnowledge

	// LearningConclusion — Conclusion (★2).
	// Source: StatisticalVerdict.
	// Routed to: FeedbackMemory.
	LearningConclusion

	// LearningPending — Pending retry (★1, ⭐new in Phase 5).
	// Source: VerdictIndeterminate.
	// Routed to: ScheduledMemory (NOT Skill/Feedback).
	LearningPending
)

// String returns the wire format name (lowercase). Unknown classes return a
// debug-formatted integer so logs stay grep-able.
func (k LearningClass) String() string {
	switch k {
	case LearningSOP:
		return "sop"
	case LearningProtocol:
		return "protocol"
	case LearningKnowledge:
		return "knowledge"
	case LearningConclusion:
		return "conclusion"
	case LearningPending:
		return "pending"
	default:
		return fmt.Sprintf("LearningClass(%d)", uint8(k))
	}
}

// ParseLearningClass reverses String() to recover the enum value from a wire
// payload. Returns an error on unknown input (fail-fast, mirroring
// ParseVerdictKind).
func ParseLearningClass(s string) (LearningClass, error) {
	switch s {
	case "sop":
		return LearningSOP, nil
	case "protocol":
		return LearningProtocol, nil
	case "knowledge":
		return LearningKnowledge, nil
	case "conclusion":
		return LearningConclusion, nil
	case "pending":
		return LearningPending, nil
	default:
		return LearningUnknown, fmt.Errorf("types: unknown LearningClass %q", s)
	}
}

// MarshalJSON encodes the class as its String() form. The wire format is the
// string, not the integer, so dashboards / D5 log filters stay
// human-readable.
func (k LearningClass) MarshalJSON() ([]byte, error) {
	return json.Marshal(k.String())
}

// UnmarshalJSON accepts the wire format produced by MarshalJSON. An empty
// string decodes to the zero value (LearningSOP) for backward compatibility
// with v2 verifier outputs that did not carry LearningClass.
func (k *LearningClass) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == "" {
		*k = LearningSOP
		return nil
	}
	parsed, err := ParseLearningClass(s)
	if err != nil {
		return err
	}
	*k = parsed
	return nil
}