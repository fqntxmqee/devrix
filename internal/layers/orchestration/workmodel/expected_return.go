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
