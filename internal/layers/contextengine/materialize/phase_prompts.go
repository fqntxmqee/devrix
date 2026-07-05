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

// AssembleMUPSSystemPrompt builds the final MUPS system prompt with node-specific
// dynamic sections before the static PrepareBase system prompt (devrix_core).
// Default order: outputHints → workItemBody → phaseAppendix → staticBase.
// Execute uses AssembleMUPSExecuteSystemPrompt (task body before output hints).
func AssembleMUPSSystemPrompt(staticBase, outputHints, workItemBody, phaseAppendix string) string {
	var parts []string
	for _, s := range []string{outputHints, workItemBody, phaseAppendix, staticBase} {
		if t := strings.TrimSpace(s); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, "\n\n")
}

// AssembleMUPSExecuteSystemPrompt puts WorkItem task context before output format hints.
func AssembleMUPSExecuteSystemPrompt(staticBase, outputHints, workItemBody, phaseAppendix string) string {
	var parts []string
	for _, s := range []string{workItemBody, outputHints, phaseAppendix, staticBase} {
		if t := strings.TrimSpace(s); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, "\n\n")
}

// AssembleMUPSPhaseSystemPrompt selects assembly order by MUPS phase.
func AssembleMUPSPhaseSystemPrompt(phase contracts.MUPSPhase, staticBase, outputHints, workItemBody, phaseAppendix string) string {
	if phase == contracts.MUPSPhaseExecute {
		return AssembleMUPSExecuteSystemPrompt(staticBase, outputHints, workItemBody, phaseAppendix)
	}
	return AssembleMUPSSystemPrompt(staticBase, outputHints, workItemBody, phaseAppendix)
}
