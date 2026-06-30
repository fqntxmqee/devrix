package sessionorchestrator

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

var (
	p0p1SeverityRE = regexp.MustCompile(`(?i)\bP[01]\b|"severity"\s*:\s*"P[01]"`)
)

// DeliverableVerifyResult is the output of VerifyDeliverable.
type DeliverableVerifyResult struct {
	Status  workmodel.DeliverableStatus
	Payload *workmodel.DeliverablePayload
	Reason  string
}

// VerifyDeliverable checks artifact summary against the registered deliverable schema.
func VerifyDeliverable(
	schema workmodel.DeliverableSchema,
	art *wavescheduler.Artifact,
) DeliverableVerifyResult {
	if schema == "" || schema == workmodel.DeliverableSchemaNotApplicable {
		return DeliverableVerifyResult{Status: workmodel.DeliverableStatusNotApplicable}
	}
	if art == nil {
		return DeliverableVerifyResult{
			Status: workmodel.DeliverableStatusIncomplete,
			Reason: "missing artifact",
		}
	}
	summary := strings.TrimSpace(art.Summary)
	stopReason, _ := art.Metadata["stop_reason"].(string)

	switch schema {
	case workmodel.DeliverableSchemaP0P1FileLine:
		return verifyP0P1FileLine(summary, stopReason)
	default:
		return DeliverableVerifyResult{Status: workmodel.DeliverableStatusNotApplicable}
	}
}

func verifyP0P1FileLine(summary, stopReason string) DeliverableVerifyResult {
	hasCitation := fileLineCitationRE.MatchString(summary)
	hasSeverity := p0p1SeverityRE.MatchString(summary)
	payload := parseDeliverablePayload(summary)

	if hasCitation && hasSeverity {
		return DeliverableVerifyResult{
			Status:  workmodel.DeliverableStatusComplete,
			Payload: payload,
		}
	}
	if stopReason == "max_iters" && !hasCitation {
		return DeliverableVerifyResult{
			Status:  workmodel.DeliverableStatusIncomplete,
			Payload: payload,
			Reason:  "max_iters without file:line deliverable",
		}
	}
	if isExplorationTransition(summary) {
		return DeliverableVerifyResult{
			Status:  workmodel.DeliverableStatusIncomplete,
			Payload: payload,
			Reason:  "exploration transition text",
		}
	}
	if hasCitation || hasSeverity {
		return DeliverableVerifyResult{
			Status:  workmodel.DeliverableStatusIncomplete,
			Payload: payload,
			Reason:  "partial p0/p1 deliverable",
		}
	}
	return DeliverableVerifyResult{
		Status:  workmodel.DeliverableStatusIncomplete,
		Payload: payload,
		Reason:  "missing p0/p1 file:line deliverable",
	}
}

func parseDeliverablePayload(summary string) *workmodel.DeliverablePayload {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return nil
	}
	start := strings.Index(summary, "{")
	end := strings.LastIndex(summary, "}")
	if start >= 0 && end > start {
		var payload workmodel.DeliverablePayload
		if err := json.Unmarshal([]byte(summary[start:end+1]), &payload); err == nil && len(payload.Findings) > 0 {
			payload.Schema = workmodel.DeliverableSchemaP0P1FileLine
			return &payload
		}
	}
	var findings []workmodel.DeliverableFinding
	for _, m := range fileLineCitationRE.FindAllString(summary, -1) {
		file, line := splitFileLineCitation(m)
		sev := "P1"
		if strings.Contains(strings.ToUpper(summary), "P0") {
			sev = "P0"
		}
		findings = append(findings, workmodel.DeliverableFinding{
			Severity: sev,
			File:     file,
			Line:     line,
			Message:  strings.TrimSpace(summary),
		})
		if len(findings) >= 8 {
			break
		}
	}
	if len(findings) == 0 {
		return &workmodel.DeliverablePayload{
			Schema: workmodel.DeliverableSchemaP0P1FileLine,
			Raw:    summary,
		}
	}
	return &workmodel.DeliverablePayload{
		Schema:   workmodel.DeliverableSchemaP0P1FileLine,
		Findings: findings,
		Raw:      summary,
	}
}

func splitFileLineCitation(citation string) (file string, line int) {
	parts := strings.Split(citation, ":")
	if len(parts) < 2 {
		return citation, 0
	}
	file = strings.Join(parts[:len(parts)-1], ":")
	var n int
	for _, ch := range parts[len(parts)-1] {
		if ch < '0' || ch > '9' {
			break
		}
		n = n*10 + int(ch-'0')
	}
	return file, n
}

func isExplorationTransition(summary string) bool {
	lower := strings.ToLower(strings.TrimSpace(summary))
	if len(lower) == 0 {
		return false
	}
	for _, m := range explorationTransitionMarkers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

var explorationTransitionMarkers = []string{
	"let me continue",
	"let me read",
	"let me explore",
	"i'll examine",
	"i will examine",
	"继续探索",
	"继续查看",
	"继续阅读",
}
