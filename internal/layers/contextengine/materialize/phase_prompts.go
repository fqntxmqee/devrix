package materialize

import (
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/prompttags"
)

// BuildPhaseAppendix returns the MUPS phase-specific system prompt appendix.
func BuildPhaseAppendix(phase contracts.MUPSPhase, loc i18n.Locale, wi *contracts.MUPSWorkItemSnapshot, toolProfile, contractDimensionDoc string) string {
	switch phase {
	case contracts.MUPSPhaseObserve:
		return i18n.ObservationTaskAppendix(loc)
	case contracts.MUPSPhasePlan:
		return i18n.StrategicPlanAppendix(loc, contractDimensionDoc)
	case contracts.MUPSPhaseExecute:
		if toolProfile == "rollup_synth" {
			return i18n.RollupSynthAppendix(loc)
		}
		return ""
	default:
		return ""
	}
}

// BuildExecuteOutputHints assembles deliverable_schema and scope_contract tags.
func BuildExecuteOutputHints(loc i18n.Locale, wi *contracts.MUPSWorkItemSnapshot) string {
	hints := i18n.WorkItemExecuteOutputHints(loc)
	if wi == nil {
		return hints
	}
	if tag := prompttags.Wrap(prompttags.TagDeliverableSchema, wi.DeliverableSchema); tag != "" {
		hints += "\n" + tag
	}
	if wi.ScopeContract != nil {
		if tag := prompttags.Wrap(prompttags.TagScopeContract, *wi.ScopeContract); tag != "" {
			hints += "\n" + tag
		}
	}
	if tag := prompttags.Wrap(prompttags.TagPriorVerifyReason, wi.PriorVerifyReason); tag != "" {
		hints += "\n" + tag
	}
	return hints
}

// AssembleMUPSSystemPrompt combines base system prompt, output hints, and phase appendix.
func AssembleMUPSSystemPrompt(base, outputHints, phaseAppendix string) string {
	var parts []string
	if s := strings.TrimSpace(base); s != "" {
		parts = append(parts, s)
	}
	if s := strings.TrimSpace(outputHints); s != "" {
		parts = append(parts, s)
	}
	if s := strings.TrimSpace(phaseAppendix); s != "" {
		parts = append(parts, s)
	}
	return strings.Join(parts, "\n\n")
}
