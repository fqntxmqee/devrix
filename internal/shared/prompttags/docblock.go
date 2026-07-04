package prompttags

import (
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// ScopeContractJSONShape is the canonical empty scope_contract JSON example.
const ScopeContractJSONShape = `{"goal_statement":"","in_scope":[],"out_of_scope":[],"assumptions":[],"open_questions":[],"success_criteria":[]}`

// ExecuteOutputTagDoc returns machine-readable tag syntax lines for Execute output.
// Locale-specific prose lives in i18n; this supplies tag shape only.
func ExecuteOutputTagDoc() string {
	var lines []string
	for _, name := range executeDocTagOrder {
		if line := tagSyntaxLine(name); line != "" {
			lines = append(lines, line)
		}
	}
	lines = append(lines, "- <conclusion>...</conclusion>")
	return strings.Join(lines, "\n")
}

var executeDocTagOrder = []TagName{
	TagOpenQuestions,
	TagScopeContract,
	TagDeliverableSchema,
	TagDeliverableContract,
	TagPriorVerifyReason,
}

func tagSyntaxLine(name TagName) string {
	switch name {
	case TagOpenQuestions:
		return "- <open_questions>one question per line</open_questions>"
	case TagScopeContract:
		return fmt.Sprintf("- <scope_contract>%s</scope_contract>", ScopeContractJSONShape)
	case TagDeliverableSchema:
		return "- <deliverable_schema>{registered_schema}</deliverable_schema>"
	case TagDeliverableContract:
		return `- <deliverable_contract>{"citation":"...","severity":"...","reject":[]}</deliverable_contract>`
	case TagPriorVerifyReason:
		return "- <prior_verify_reason>...</prior_verify_reason>"
	default:
		return ""
	}
}

// DocBlock returns machine tag syntax lines for the given MUPS phase.
func DocBlock(phase contracts.MUPSPhase) string {
	switch phase {
	case contracts.MUPSPhaseExecute:
		return ExecuteOutputTagDoc()
	case contracts.MUPSPhaseObserve:
		return DocBlockObserveSchema()
	case contracts.MUPSPhasePlan:
		return DocBlockPlanSchema("")
	default:
		return ""
	}
}

// DocBlockObserveSchema returns the Observe whole-body JSON array element shape.
func DocBlockObserveSchema() string {
	return `{"kind":"obs_fact|obs_signal|obs_uncertainty|obs_deviation","strength":0.0-1.0,"statement":"...","question":"...","evidence":["wi_id"]}`
}

// DocBlockPlanSchema returns the Plan whole-body JSON object shape.
func DocBlockPlanSchema(contractExample string) string {
	if strings.TrimSpace(contractExample) == "" {
		contractExample = `{"citation":"file_line","severity":"p0_p1","reject":["planning_meta"],"min_runes":0}`
	}
	return fmt.Sprintf(
		`{"execution_mode":"single|decompose|parallel_probe","scope_in":["path/"],"child_specs":[],"deliverable_contract":%s,"react_iters_hint":5,"rationale":"..."}`,
		contractExample,
	)
}
