package prompttags

import "github.com/devrix/devrix/internal/shared/contracts"

// PromptPlane classifies a prompt field/tag as task payload (data) or format/budget (control).
type PromptPlane string

const (
	PlaneData    PromptPlane = "data"
	PlaneControl PromptPlane = "control"
)

// FieldSemantic documents LLM-facing when/when-not guidance for one field or output kind.
type FieldSemantic struct {
	Name     string // tag name, lineframe key, or obs_* kind
	Plane    PromptPlane
	WhenUse  string // i18n key suffix under prompttags.semantics.*
	WhenNot  string // optional i18n key suffix
	Enforced bool   // true when a Go gate backs the prompt claim
}

// PhaseSemantics holds node role and input/output semantic entries for one MUPS phase.
type PhaseSemantics struct {
	Phase       contracts.MUPSPhase
	NodeRoleKey string // i18n key for one-line node role
	OutputRules []FieldSemantic
	InputRules  []FieldSemantic
}

// SemanticsForPhase returns the locale-neutral TagSemanticsRegistry entry for phase.
func SemanticsForPhase(phase contracts.MUPSPhase) PhaseSemantics {
	switch phase {
	case contracts.MUPSPhaseObserve:
		return observeSemantics
	case contracts.MUPSPhasePlan:
		return planSemantics
	case contracts.MUPSPhaseExecute:
		return executeSemantics
	default:
		return PhaseSemantics{Phase: phase}
	}
}

// FrameFieldPlane returns the data/control plane for a user-frame field.
func FrameFieldPlane(frame FrameName, name TagName) PromptPlane {
	if m, ok := frameFieldPlanes[frame]; ok {
		if p, ok := m[name]; ok {
			return p
		}
	}
	return PlaneData
}

var observeSemantics = PhaseSemantics{
	Phase:       contracts.MUPSPhaseObserve,
	NodeRoleKey: "observe.node_role",
	OutputRules: []FieldSemantic{
		{Name: "obs_uncertainty", Plane: PlaneData, WhenUse: "observe.kind.obs_uncertainty.when_use", WhenNot: "observe.kind.obs_uncertainty.when_not"},
		{Name: "obs_fact", Plane: PlaneData, WhenUse: "observe.kind.obs_fact.when_use", WhenNot: "observe.kind.obs_fact.when_not", Enforced: true},
		{Name: "obs_signal", Plane: PlaneData, WhenUse: "observe.kind.obs_signal.when_use", WhenNot: "observe.kind.obs_signal.when_not"},
		{Name: "obs_deviation", Plane: PlaneData, WhenUse: "observe.kind.obs_deviation.when_use", WhenNot: "observe.kind.obs_deviation.when_not"},
		{Name: "strength", Plane: PlaneControl, WhenUse: "observe.field.strength.when_use"},
		{Name: "question", Plane: PlaneData, WhenUse: "observe.field.question.when_use", Enforced: true},
		{Name: "evidence", Plane: PlaneData, WhenUse: "observe.field.evidence.when_use"},
		{Name: "max_proposals", Plane: PlaneControl, WhenUse: "observe.rule.max_proposals", Enforced: true},
	},
	InputRules: []FieldSemantic{
		{Name: string(TagDirective), Plane: PlaneData, WhenUse: "observe.input.directive.when_use"},
		{Name: string(TagPriorParseReject), Plane: PlaneControl, WhenUse: "observe.input.prior_parse_reject.when_use", Enforced: true},
		{Name: string(TagSignal), Plane: PlaneData, WhenUse: "observe.input.signal.when_use"},
		{Name: string(TagPriorMean), Plane: PlaneControl, WhenUse: "observe.input.prior_mean.when_use"},
		{Name: string(TagScopeOpenQuestion), Plane: PlaneData, WhenUse: "observe.input.scope_open_question.when_use"},
		{Name: string(TagIncrementalOnly), Plane: PlaneControl, WhenUse: "observe.input.incremental_only.when_use", Enforced: true},
	},
}

var planSemantics = PhaseSemantics{
	Phase:       contracts.MUPSPhasePlan,
	NodeRoleKey: "plan.node_role",
	OutputRules: []FieldSemantic{
		{Name: "execution_mode", Plane: PlaneControl, WhenUse: "plan.output.execution_mode.when_use", Enforced: true},
		{Name: "deliverable_contract", Plane: PlaneControl, WhenUse: "plan.output.deliverable_contract.when_use"},
		{Name: "child_specs", Plane: PlaneControl, WhenUse: "plan.output.child_specs.when_use", Enforced: true},
	},
	InputRules: []FieldSemantic{
		{Name: string(TagDirective), Plane: PlaneData, WhenUse: "plan.input.directive.when_use"},
		{Name: string(TagPriorParseReject), Plane: PlaneControl, WhenUse: "plan.input.prior_parse_reject.when_use", Enforced: true},
		{Name: string(TagObservationSummary), Plane: PlaneData, WhenUse: "plan.input.observation_summary.when_use"},
		{Name: string(TagUncertaintyMean), Plane: PlaneControl, WhenUse: "plan.input.uncertainty_mean.when_use", Enforced: true},
		{Name: string(TagRemainingChildren), Plane: PlaneControl, WhenUse: "plan.input.remaining_children.when_use"},
	},
}

var executeSemantics = PhaseSemantics{
	Phase:       contracts.MUPSPhaseExecute,
	NodeRoleKey: "execute.node_role",
	OutputRules: []FieldSemantic{
		{Name: string(TagDeliverableContract), Plane: PlaneControl, WhenUse: "execute.output.deliverable_contract.when_use", Enforced: true},
		{Name: "findings_json", Plane: PlaneData, WhenUse: "execute.output.findings_json.when_use", Enforced: true},
		{Name: string(TagOpenQuestions), Plane: PlaneData, WhenUse: "execute.output.open_questions.when_use"},
		{Name: string(TagScopeContract), Plane: PlaneControl, WhenUse: "execute.output.scope_contract.when_use"},
		{Name: "conclusion", Plane: PlaneData, WhenUse: "execute.output.conclusion.when_use"},
	},
}

var frameFieldPlanes = map[FrameName]map[TagName]PromptPlane{
	FrameObserveUser: {
		TagWorkItemID:          PlaneControl,
		TagDirective:           PlaneData,
		TagPriorParseReject:    PlaneControl,
		TagPriorMean:           PlaneControl,
		TagScopeGoal:          PlaneData,
		TagScopeOpenQuestion:  PlaneData,
		TagSignal:             PlaneData,
		TagPriorObservationIDs: PlaneControl,
		TagIncrementalOnly:    PlaneControl,
	},
	FramePlanUser: {
		TagWorkItemID:         PlaneControl,
		TagDirective:          PlaneData,
		TagPriorParseReject:   PlaneControl,
		TagObservationIDs:     PlaneData,
		TagObservationSummary: PlaneData,
		TagDepth:              PlaneControl,
		TagMaxDepth:           PlaneControl,
		TagExistingChildren:   PlaneControl,
		TagRemainingChildren:  PlaneControl,
		TagMaxChildren:        PlaneControl,
		TagDecomposeUsedToday: PlaneControl,
		TagRemainingDaily:     PlaneControl,
		TagMaxDaily:           PlaneControl,
		TagMaxIters:           PlaneControl,
		TagParentScopeIn:      PlaneControl,
		TagUncertaintyMean:    PlaneControl,
	},
}
