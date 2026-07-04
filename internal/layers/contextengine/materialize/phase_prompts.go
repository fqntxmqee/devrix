package materialize

import (
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/shared/contracts"
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
	if schema := strings.TrimSpace(wi.DeliverableSchema); schema != "" {
		hints += fmt.Sprintf("\n<deliverable_schema>%s</deliverable_schema>", schema)
	}
	if wi.ScopeContract != nil {
		hints += scopeContractBlock(wi.ScopeContract)
	}
	if wi.PriorVerifyReason != "" {
		hints += fmt.Sprintf("\n<prior_verify_reason>%s</prior_verify_reason>", wi.PriorVerifyReason)
	}
	return hints
}

func scopeContractBlock(sc *contracts.MUPSScopeContract) string {
	if sc == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n<scope_contract>{")
	first := true
	writeField := func(key, val string) {
		if strings.TrimSpace(val) == "" {
			return
		}
		if !first {
			b.WriteByte(',')
		}
		first = false
		fmt.Fprintf(&b, `%q:%q`, key, val)
	}
	writeField("goal_statement", sc.GoalStatement)
	if len(sc.InScope) > 0 {
		if !first {
			b.WriteByte(',')
		}
		first = false
		b.WriteString(`"in_scope":[`)
		for i, p := range sc.InScope {
			if i > 0 {
				b.WriteByte(',')
			}
			fmt.Fprintf(&b, `%q`, p)
		}
		b.WriteByte(']')
	}
	writeField("out_of_scope", strings.Join(sc.OutOfScope, ","))
	b.WriteString("}</scope_contract>")
	return b.String()
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
