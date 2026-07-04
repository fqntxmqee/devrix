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
	EncodingLineFrame EncodingProfile = "lineframe" // bare key: value user prompt frames
	EncodingWholeBody EncodingProfile = "wholebody" // bare JSON/array with optional fence strip
)

// FrameName identifies a registered MUPS user prompt frame.
type FrameName string

const (
	FrameObserveUser FrameName = "observe_user"
	FramePlanUser    FrameName = "plan_user"
)

// IOShapeEntry documents one MUPS LLM I/O shape (parseability invariant: one profile each).
type IOShapeEntry struct {
	Name    string
	Profile EncodingProfile
	Phases  []contracts.MUPSPhase
}

// TagSpec describes one registered tag.
type TagSpec struct {
	Name    TagName
	Profile EncodingProfile
}

// LineFrameRegistry maps frame names to fixed-order user prompt field specs.
var LineFrameRegistry = map[FrameName]FrameSpec{
	FrameObserveUser: ObserveUserFrame,
	FramePlanUser:    PlanUserFrame,
}

// WholeBodyRegistry documents whole-body LLM response shapes per MUPS phase.
var WholeBodyRegistry = map[contracts.MUPSPhase]IOShapeEntry{
	contracts.MUPSPhaseObserve: {Name: "observe_proposals", Profile: EncodingWholeBody, Phases: []contracts.MUPSPhase{contracts.MUPSPhaseObserve}},
	contracts.MUPSPhasePlan:    {Name: "strategic_plan", Profile: EncodingWholeBody, Phases: []contracts.MUPSPhase{contracts.MUPSPhasePlan}},
}

// MUPSIOCatalog is the flat index of all MUPS I/O shapes (envelope + lineframe + wholebody).
var MUPSIOCatalog = buildMUPSIOCatalog()

func buildMUPSIOCatalog() []IOShapeEntry {
	var out []IOShapeEntry
	for name, spec := range MUPSRegistry {
		out = append(out, IOShapeEntry{
			Name:    string(name),
			Profile: spec.Profile,
			Phases:  tagPhases[name],
		})
	}
	for name := range LineFrameRegistry {
		switch name {
		case FrameObserveUser:
			out = append(out, IOShapeEntry{Name: string(name), Profile: EncodingLineFrame, Phases: []contracts.MUPSPhase{contracts.MUPSPhaseObserve}})
		case FramePlanUser:
			out = append(out, IOShapeEntry{Name: string(name), Profile: EncodingLineFrame, Phases: []contracts.MUPSPhase{contracts.MUPSPhasePlan}})
		}
	}
	for _, entry := range WholeBodyRegistry {
		out = append(out, entry)
	}
	return out
}

// MUPSRegistry holds envelope tags used across MUPS materialize and orchestration.
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

// LookupLineFrame returns the FrameSpec for a registered user prompt frame.
func LookupLineFrame(name FrameName) (FrameSpec, bool) {
	spec, ok := LineFrameRegistry[name]
	return spec, ok
}
