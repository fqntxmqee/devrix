package prompttags

import "github.com/devrix/devrix/internal/shared/contracts"

// TagName identifies a machine-readable prompt tag.
type TagName string

const (
	TagScopeContract       TagName = "scope_contract"
	TagDeliverableContract TagName = "deliverable_contract"
	TagDeliverableSchema   TagName = "deliverable_schema"
	TagPriorVerifyReason   TagName = "prior_verify_reason"
	TagOpenQuestions       TagName = "open_questions"
)

// EncodingProfile selects how a tag payload is serialized.
type EncodingProfile string

const (
	EncodingEnvelope  EncodingProfile = "envelope"  // <tag>...</tag>
	EncodingLineField EncodingProfile = "linefield" // newline-separated lines inside envelope
	EncodingWholeBody EncodingProfile = "wholebody" // bare JSON/array with optional fence strip
)

// TagSpec describes one registered tag.
type TagSpec struct {
	Name    TagName
	Profile EncodingProfile
}

// MUPSRegistry holds envelope tags used across MUPS materialize and orchestration.
//
// Whole-body response shapes (EncodingWholeBody, not envelope tags):
//   - Observe: JSON array of observation proposals — see DocBlockObserveSchema
//   - Plan:    JSON object strategic plan — see DocBlockPlanSchema
var MUPSRegistry = map[TagName]TagSpec{
	TagScopeContract:       {Name: TagScopeContract, Profile: EncodingEnvelope},
	TagDeliverableContract: {Name: TagDeliverableContract, Profile: EncodingEnvelope},
	TagDeliverableSchema:   {Name: TagDeliverableSchema, Profile: EncodingEnvelope},
	TagPriorVerifyReason:   {Name: TagPriorVerifyReason, Profile: EncodingEnvelope},
	TagOpenQuestions:       {Name: TagOpenQuestions, Profile: EncodingLineField},
}

// tagPhases lists MUPS phases where each envelope tag may appear in agent output.
var tagPhases = map[TagName][]contracts.MUPSPhase{
	TagScopeContract:       {contracts.MUPSPhaseExecute},
	TagDeliverableContract: {contracts.MUPSPhaseExecute},
	TagDeliverableSchema:   {contracts.MUPSPhaseExecute},
	TagPriorVerifyReason:   {contracts.MUPSPhaseExecute, contracts.MUPSPhaseVerify},
	TagOpenQuestions:       {contracts.MUPSPhaseExecute},
}

// TagAppliesToPhase reports whether name is applicable to phase.
// Empty phase matches all registered tags (backward compatible).
func TagAppliesToPhase(name TagName, phase string) bool {
	if phase == "" {
		return true
	}
	phases, ok := tagPhases[name]
	if !ok {
		return false
	}
	for _, p := range phases {
		if string(p) == phase {
			return true
		}
	}
	return false
}

// Lookup returns the TagSpec for name when registered.
func Lookup(name TagName) (TagSpec, bool) {
	spec, ok := MUPSRegistry[name]
	return spec, ok
}
