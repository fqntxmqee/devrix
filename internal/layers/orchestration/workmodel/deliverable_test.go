package workmodel

import "testing"

// T: D7-S9-A93-T15 (DM-20260701-001 T-P1-5 SchemaMonotonicNarrowing)
//
// NarrowestSchema enforces the "LLM may narrow, never widen" contract.
// The strategic proposer can only constrain the producer further, never
// loosen the inferred shape. Cases covered:
//
//   (NA, NA)         → NA       // both empty baseline
//   (NA, P0P1)       → P0P1     // LLM adds a schema on top of no-infer
//   (P0P1, NA)       → P0P1     // LLM tries to drop — REJECTED, keep P0P1
//   (P0P1, P0P1)     → P0P1     // equal
//   (P0P1, "")       → P0P1     // empty strategic = no-op
//   ("", NA)         → NA       // empty inferred + empty strategic
//   ("", P0P1)       → P0P1     // empty inferred, strategic has shape
func TestNarrowestSchema(t *testing.T) {
	cases := []struct {
		name      string
		inferred  DeliverableSchema
		strategic DeliverableSchema
		want      DeliverableSchema
	}{
		{"both empty", "", "", ""},
		{"both NA", DeliverableSchemaNotApplicable, DeliverableSchemaNotApplicable, DeliverableSchemaNotApplicable},
		{"inferred NA + strategic P0P1 (LLM adds)", DeliverableSchemaNotApplicable, DeliverableSchemaP0P1FileLine, DeliverableSchemaP0P1FileLine},
		{"inferred P0P1 + strategic NA (LLM drops — REJECTED)", DeliverableSchemaP0P1FileLine, DeliverableSchemaNotApplicable, DeliverableSchemaP0P1FileLine},
		{"inferred P0P1 + strategic P0P1 (equal)", DeliverableSchemaP0P1FileLine, DeliverableSchemaP0P1FileLine, DeliverableSchemaP0P1FileLine},
		{"inferred P0P1 + strategic empty (no-op)", DeliverableSchemaP0P1FileLine, "", DeliverableSchemaP0P1FileLine},
		{"inferred empty + strategic NA", "", DeliverableSchemaNotApplicable, DeliverableSchemaNotApplicable},
		{"inferred empty + strategic P0P1", "", DeliverableSchemaP0P1FileLine, DeliverableSchemaP0P1FileLine},
		{"inferred NA + strategic empty", DeliverableSchemaNotApplicable, "", DeliverableSchemaNotApplicable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NarrowestSchema(c.inferred, c.strategic); got != c.want {
				t.Errorf("NarrowestSchema(%q, %q) = %q, want %q", c.inferred, c.strategic, got, c.want)
			}
		})
	}
}
