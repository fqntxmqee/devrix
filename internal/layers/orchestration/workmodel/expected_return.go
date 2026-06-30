package workmodel

import "strings"

// DefaultStructuralExpectedReturn is the fallback when no deliverable schema applies.
const DefaultStructuralExpectedReturn = "<expected_return>same_as_parent_directive</expected_return>"

// DefaultChildExpectedReturn is the machine-readable ExpectedReturn for structural
// decompose fallback. Uses deliverable schema tags, not prose instructions.
func DefaultChildExpectedReturn(item *WorkItem, directive string) string {
	schema := InferDeliverableSchema(item, directive, "")
	if tag := DeliverableSchemaTag(schema); tag != "" {
		return tag
	}
	return DefaultStructuralExpectedReturn
}

// DeliverableSchemaTag returns a machine-readable schema marker for Materialize / Verify.
func DeliverableSchemaTag(schema DeliverableSchema) string {
	if schema == "" || schema == DeliverableSchemaNotApplicable {
		return ""
	}
	return "<deliverable_schema>" + string(schema) + "</deliverable_schema>"
}

// ParseDeliverableSchemaTag reads a schema tag from ExpectedReturn or directive tail.
func ParseDeliverableSchemaTag(s string) DeliverableSchema {
	s = strings.TrimSpace(s)
	const open = "<deliverable_schema>"
	const close = "</deliverable_schema>"
	start := strings.Index(s, open)
	if start < 0 {
		return DeliverableSchemaNotApplicable
	}
	end := strings.Index(s[start:], close)
	if end < 0 {
		return DeliverableSchemaNotApplicable
	}
	raw := strings.TrimSpace(s[start+len(open) : start+end])
	switch DeliverableSchema(raw) {
	case DeliverableSchemaP0P1FileLine:
		return DeliverableSchemaP0P1FileLine
	default:
		return DeliverableSchemaNotApplicable
	}
}

// AcceptanceCriteriaFor returns a human-readable description of what the
// LLM producer must produce to satisfy the deliverable schema.
//
// RH-MUPS-10 (DM-20260701-001): before this function, the producer-side
// execute prompt only carried a machine tag like `<deliverable_schema>
// p0_p1_file_line</deliverable_schema>`. The actual acceptance bar —
// "≥500 runes / include P0/P1 severity / cite file:line / no planning
// meta" — lived in the deterministic verify regex and was invisible to
// the LLM until it failed. The producer couldn't self-correct because
// it didn't know what verify would check for.
//
// Now the criteria are appended to the execute hint as readable text.
// Returns empty string when the schema is empty or not_applicable —
// callers should skip the section in that case.
func AcceptanceCriteriaFor(schema DeliverableSchema) string {
	switch schema {
	case DeliverableSchemaP0P1FileLine:
		return "Acceptance: include at least one file:line citation (e.g. `path/to/file.go:42`); tag findings with severity P0 or P1; avoid planning meta phrases like `let me continue` / `我将要` / `parallel explore`; produce a substantive summary (≥500 runes for rollup)."
	case "":
		return ""
	default:
		// Unknown schemas (forward-compatibility): return empty rather than
		// fabricate criteria. The producer can still see the schema tag.
		return ""
	}
}
