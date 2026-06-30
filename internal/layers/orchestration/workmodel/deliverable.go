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

// SchemaNarrowness rank orders the deliverable schemas by how much
// shape they impose on the producer. Higher = more constrained. The
// strategic proposer may NARROW a schema (inferred=p0_p1_file_line,
// strategic=not_applicable is rejected; inferred=not_applicable,
// strategic=p0_p1_file_line is accepted because there is nothing to
// narrow). Monotonic-narrowing is the schema analog of "monotonic
// uncertainty convergence" in C1.
//
// RH-MUPS-11 (DM-20260701-001, T-P1-5): without this contract the
// strategic proposer could "downgrade" a p0_p1_file_line directive
// into not_applicable, skipping verify and silently passing short
// reviews. The narrowest-non-empty rule preserves the inferred
// shape when the proposer disagrees.
//
// Future schemas append here so the order is canonical.
var schemaNarrowness = map[DeliverableSchema]int{
	DeliverableSchemaNotApplicable: 0,
	DeliverableSchemaP0P1FileLine:  10,
}

// NarrowestSchema returns the most-constrained schema of the two.
// Empty / NotApplicable as a strategic override is rejected when the
// inference picked something more specific — the LLM cannot widen.
// Empty / NotApplicable as the inference is freely overridable.
//
// Cases (inferred, strategic) → result:
//   (NA, *)     → strategic  // inference says "no shape"; LLM may add one
//   (X, NA)     → X          // inference says X; LLM cannot drop
//   (X, X)      → X
//   (X, Y!=X)   → X          // unrelated: keep the more specific inference
func NarrowestSchema(inferred, strategic DeliverableSchema) DeliverableSchema {
	if strategic == "" {
		return inferred
	}
	if inferred == "" || inferred == DeliverableSchemaNotApplicable {
		return strategic
	}
	// Both non-empty: keep the higher-narrowness one. Equal narrowness
	// → prefer inferred (the deterministic baseline).
	in, sok := schemaNarrowness[inferred]
	st, stok := schemaNarrowness[strategic]
	if !sok || !stok {
		return inferred
	}
	if st > in {
		return strategic
	}
	return inferred
}
