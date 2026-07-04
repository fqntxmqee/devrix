package sessionorchestrator

import (
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

// AppendDeliverableContractExecuteHint adds contract tag + machine acceptance criteria.
func AppendDeliverableContractExecuteHint(directive string, contract workmodel.DeliverableContract) string {
	tag := workmodel.DeliverableContractTag(contract)
	if tag == "" {
		return directive
	}
	if strings.Contains(directive, tag) {
		return directive
	}
	crit := workmodel.AcceptanceCriteriaForContract(contract)
	finalHint := workmodel.DeliverableFinalAnswerHint(contract)
	var hint strings.Builder
	hint.WriteString(tag)
	if crit != "" {
		hint.WriteString("\n")
		hint.WriteString(crit)
	}
	if finalHint != "" {
		hint.WriteString("\n")
		hint.WriteString(finalHint)
	}
	return strings.TrimSpace(directive) + "\n\n" + hint.String()
}

// SynthesisTurnExecuteHint instructs the model on the tool-free final iteration.
func SynthesisTurnExecuteHint(contract workmodel.DeliverableContract) string {
	if contract.Normalized().Structure != workmodel.DeliverableStructureFindingsJSON {
		return ""
	}
	return "Final iteration: tools are disabled. Use evidence from prior tool results only. " +
		"Respond with ONLY the findings_json deliverable; do not explore new paths or OpenSpec docs."
}

// PriorDeliverableRetryHint builds machine retry context for inline rounds.
// Omits spawn rationale and full artifact prose to avoid scope drift (DM-20260703-001).
func PriorDeliverableRetryHint(item *workmodel.WorkItem, contract workmodel.DeliverableContract) string {
	if item == nil || item.LastRound == nil {
		return ""
	}
	lr := item.LastRound
	if lr.DeliverableStatus != workmodel.DeliverableStatusIncomplete {
		return ""
	}
	var parts []string
	if sc := item.ScopeContract; sc != nil && len(sc.InScope) > 0 {
		parts = append(parts, "ScopeIn: "+strings.Join(sc.InScope, ", "))
		parts = append(parts, "Keep review target unchanged; do not switch to openspec/ or other directories.")
	}
	summary := strings.TrimSpace(lr.ArtifactSummary)
	if summary != "" && contract.ContractApplicable() {
		got := workmodel.VerifyDeliverableContract(contract, summary, "max_iters")
		if got.Reason != "" {
			parts = append(parts, "PriorDeliverableFailure: "+got.Reason)
		}
	}
	if workmodel.RequiresSynthesisTurn(contract) {
		parts = append(parts, "Stop exploring; synthesize findings_json from files already read.")
	}
	return strings.Join(parts, "\n")
}

// machineSpawnFeedback extracts non-prose spawn lines safe for Execute retry
// (strategic rejection, scope gate). Full SpawnRationale is omitted to avoid
// scope drift from stale path lists.
func machineSpawnFeedback(item *workmodel.WorkItem) string {
	if item == nil || item.LastRound == nil {
		return ""
	}
	var parts []string
	for _, line := range strings.Split(item.LastRound.SpawnRationale, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "strategic_plan_rejected") || strings.HasPrefix(line, "scope:") {
			parts = append(parts, line)
		}
	}
	return strings.Join(parts, "\n")
}

// AppendDeliverableExecuteHint adds deliverable contract tag for legacy schema callers.
func AppendDeliverableExecuteHint(directive string, schema workmodel.DeliverableSchema) string {
	return AppendDeliverableContractExecuteHint(directive, workmodel.ExpandLegacySchemaToContract(schema))
}

func uncertaintyReportSummary(anomalyCount int, intentKind string) string {
	if anomalyCount == 0 && intentKind == "" {
		return ""
	}
	var parts []string
	if intentKind != "" {
		parts = append(parts, "intent="+intentKind)
	}
	if anomalyCount > 0 {
		parts = append(parts, fmt.Sprintf("anomalies=%d", anomalyCount))
	}
	return strings.Join(parts, "; ")
}
