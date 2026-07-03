package workmodel

import "testing"

// T: D7-S9-A93-T15 (DM-20260701-001 T-P1-5 SchemaMonotonicNarrowing)
func TestNarrowestSchema(t *testing.T) {
	registered := FirstRegisteredDeliverableSchema()
	if registered == "" {
		t.Fatal("expected at least one registered deliverable schema")
	}
	cases := []struct {
		name      string
		inferred  DeliverableSchema
		strategic DeliverableSchema
		want      DeliverableSchema
	}{
		{"both empty", "", "", ""},
		{"both NA", DeliverableSchemaNotApplicable, DeliverableSchemaNotApplicable, DeliverableSchemaNotApplicable},
		{"inferred NA + strategic registered (LLM adds)", DeliverableSchemaNotApplicable, registered, registered},
		{"inferred registered + strategic NA (LLM drops — REJECTED)", registered, DeliverableSchemaNotApplicable, registered},
		{"inferred registered + strategic registered (equal)", registered, registered, registered},
		{"inferred registered + strategic empty (no-op)", registered, "", registered},
		{"inferred empty + strategic NA", "", DeliverableSchemaNotApplicable, DeliverableSchemaNotApplicable},
		{"inferred empty + strategic registered", "", registered, registered},
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
