package sessionorchestrator

import (
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

// AppendDeliverableExecuteHint adds a machine-readable schema tag when Verify expects
// a registered deliverable schema. Prose instructions belong in D2/i18n, not Go.
func AppendDeliverableExecuteHint(directive string, schema workmodel.DeliverableSchema) string {
	tag := workmodel.DeliverableSchemaTag(schema)
	if tag == "" {
		return directive
	}
	if strings.Contains(directive, tag) {
		return directive
	}
	return strings.TrimSpace(directive) + "\n\n" + tag
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
