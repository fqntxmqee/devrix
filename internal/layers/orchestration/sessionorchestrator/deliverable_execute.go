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
	var hint strings.Builder
	hint.WriteString(tag)
	if crit != "" {
		hint.WriteString("\n")
		hint.WriteString(crit)
	}
	return strings.TrimSpace(directive) + "\n\n" + hint.String()
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
