package prompttags

import "github.com/devrix/devrix/internal/shared/contracts"

// PhaseSemantics holds node role and output semantic entries for one MUPS phase.
// Input rules are derived via InputRulesForFrame (LineFrameRegistry SoT).
type PhaseSemantics struct {
	Phase       contracts.MUPSPhase
	NodeRoleKey string // i18n key for one-line node role (locale overlay only)
	OutputRules []SemanticRule
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
	OutputRules: []SemanticRule{
		{Target: "obs_uncertainty", Plane: PlaneData, WhenUse: CondScopeUnclear, WhenNot: CondStrongFactExists},
		{Target: "obs_fact", Plane: PlaneData, WhenUse: CondSignalBacked, WhenNot: CondNoSpeculation, Enforced: true, Gate: "obs_fact_strength_cap"},
		{Target: "obs_signal", Plane: PlaneData, WhenUse: CondStructuredSignal, WhenNot: CondPreferUncertainty},
		{Target: "obs_deviation", Plane: PlaneData, WhenUse: CondMetricDelta, WhenNot: CondNoBaseline},
		{Target: "strength", Plane: PlaneControl, WhenUse: CondStrengthAlignedKind},
		{Target: "question", Plane: PlaneData, WhenUse: CondRequiredForObsUncertainty, Enforced: true, Gate: "obs_uncertainty_question"},
		{Target: "evidence", Plane: PlaneData, WhenUse: CondEvidenceExistingIDsOnly},
		{Target: "max_proposals", Plane: PlaneControl, WhenUse: CondMaxProposalsThree, Enforced: true, Gate: "ValidateObservationProposals"},
	},
}

var planSemantics = PhaseSemantics{
	Phase:       contracts.MUPSPhasePlan,
	NodeRoleKey: "plan.node_role",
	OutputRules: []SemanticRule{
		{Target: "execution_mode", Plane: PlaneControl, WhenUse: CondExecutionModeDecisionTree, Enforced: true, Gate: "applySingleModeUncertaintyGate"},
		{Target: "deliverable_contract", Plane: PlaneControl, WhenUse: CondDeliverableContractExample},
		{Target: "child_specs", Plane: PlaneControl, WhenUse: CondChildSpecsDecomposeMax2, Enforced: true, Gate: "applyBudgetCap"},
	},
}

var executeSemantics = PhaseSemantics{
	Phase:       contracts.MUPSPhaseExecute,
	NodeRoleKey: "execute.node_role",
	OutputRules: []SemanticRule{
		{Target: string(TagDeliverableContract), Plane: PlaneControl, WhenUse: CondRequiredWhenContract, Enforced: true, Gate: "VerifyDeliverableContract"},
		{Target: "findings_json", Plane: PlaneData, WhenUse: CondRequiredWhenFindingsJSON, Enforced: true, Gate: "findings_json_verify"},
		{Target: string(TagOpenQuestions), Plane: PlaneData, WhenUse: CondOptionalResidualQuestions},
		{Target: string(TagScopeContract), Plane: PlaneControl, WhenUse: CondOptionalScopeUpdate},
		{Target: "conclusion", Plane: PlaneData, WhenUse: CondOptionalConclusionProse},
	},
}

var frameFieldPlanes = map[FrameName]map[TagName]PromptPlane{
	FrameObserveUser: {
		TagWorkItemID:           PlaneControl,
		TagDirective:            PlaneData,
		TagPriorParseReject:     PlaneControl,
		TagPriorMean:            PlaneControl,
		TagScopeGoal:            PlaneData,
		TagScopeOpenQuestion:    PlaneData,
		TagSignal:               PlaneData,
		TagPriorObservationIDs:  PlaneControl,
		TagIncrementalOnly:     PlaneControl,
		// DM-20260705-010 Phase 2 T8: append-only 2 字段.
		TagPriorArtifactSummary: PlaneData,
		TagKnownGaps:            PlaneData,
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
