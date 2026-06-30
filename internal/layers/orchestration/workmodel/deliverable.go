package workmodel

import (
	"strings"
)

// DeliverableSchema names the expected Execute output shape for Verify.
type DeliverableSchema string

const (
	DeliverableSchemaNotApplicable   DeliverableSchema = "not_applicable"
	DeliverableSchemaP0P1FileLine    DeliverableSchema = "p0_p1_file_line"
)

// DeliverableStatus is the outcome of schema verification (DM-20260630-012).
type DeliverableStatus string

const (
	DeliverableStatusNotApplicable DeliverableStatus = "not_applicable"
	DeliverableStatusComplete      DeliverableStatus = "complete"
	DeliverableStatusIncomplete    DeliverableStatus = "incomplete"
)

// DeliverableFinding is one structured review finding for upward bubble / rollup.
type DeliverableFinding struct {
	Severity string `json:"severity"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Message  string `json:"message,omitempty"`
}

// DeliverablePayload is the parsed deliverable attached to a pipeline round.
type DeliverablePayload struct {
	Schema   DeliverableSchema    `json:"schema,omitempty"`
	Findings []DeliverableFinding `json:"findings,omitempty"`
	Raw      string               `json:"raw,omitempty"`
}

// InferDeliverableSchema selects a verify schema from directive and child downlink.
func InferDeliverableSchema(item *WorkItem, directive string, expectedReturn string) DeliverableSchema {
	if item != nil && item.NeedsRollup {
		return DeliverableSchemaP0P1FileLine
	}
	text := strings.ToLower(strings.TrimSpace(directive + " " + expectedReturn))
	if text == "" {
		return DeliverableSchemaNotApplicable
	}
	reviewHints := []string{
		"p0/p1", "p0", "p1", "file:line", "review", "code review", "审查", "评审",
	}
	for _, h := range reviewHints {
		if strings.Contains(text, h) {
			return DeliverableSchemaP0P1FileLine
		}
	}
	return DeliverableSchemaNotApplicable
}

// ExpectedReturnForItem reads child downlink expected_return when present.
func ExpectedReturnForItem(tm *TaskManager, sessionID string, item *WorkItem) string {
	if tm == nil || item == nil {
		return ""
	}
	if dl, ok := tm.ChildDownlinkFor(sessionID, item.ID); ok {
		return strings.TrimSpace(dl.ExpectedReturn)
	}
	return ""
}
