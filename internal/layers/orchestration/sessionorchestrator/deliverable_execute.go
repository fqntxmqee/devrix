package sessionorchestrator

import (
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

// AppendDeliverableExecuteHint adds a machine-readable schema tag AND a
// human-readable acceptance-criteria section when Verify expects a
// registered deliverable schema. Prose instructions belong in D2/i18n,
// not Go — the criteria text below is sourced from workmodel.AcceptanceCriteriaFor
// which itself is the single canonical mapping from schema → bar.
//
// RH-MUPS-10 (DM-20260701-001): previously only the tag was injected; the
// actual acceptance bar (length threshold / file:line citation / P0/P1
// severity tags / planning-meta denylist) lived only in the verify regex
// and was invisible to the LLM. The producer had no way to self-correct
// until verify failed. Now the criteria ride along with the tag.
func AppendDeliverableExecuteHint(directive string, schema workmodel.DeliverableSchema) string {
	tag := workmodel.DeliverableSchemaTag(schema)
	if tag == "" {
		return directive
	}
	if strings.Contains(directive, tag) {
		return directive
	}
	crit := workmodel.AcceptanceCriteriaFor(schema)
	var hint strings.Builder
	hint.WriteString(tag)
	if crit != "" {
		hint.WriteString("\n")
		hint.WriteString(crit)
	}
	return strings.TrimSpace(directive) + "\n\n" + hint.String()
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
