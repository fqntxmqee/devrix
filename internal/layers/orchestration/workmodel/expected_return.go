package workmodel

import (
	"strings"
)

// DefaultStructuralExpectedReturn is the fallback when no deliverable contract applies.
const DefaultStructuralExpectedReturn = "<expected_return>same_as_parent_directive</expected_return>"

// DefaultChildExpectedReturn propagates parent's deliverable contract tag when present.
func DefaultChildExpectedReturn(parent *WorkItem, directive string) string {
	if parent != nil && parent.LastRound != nil {
		if parent.LastRound.DeliverableContract.ContractApplicable() {
			return DeliverableContractTag(parent.LastRound.DeliverableContract)
		}
		if tag := DeliverableSchemaTag(parent.LastRound.DeliverableSchema); tag != "" {
			return tag
		}
	}
	if c := ParseDeliverableContractTag(directive); c.ContractApplicable() {
		return DeliverableContractTag(c)
	}
	if schema := ParseDeliverableSchemaTag(directive); schema != DeliverableSchemaNotApplicable {
		return DeliverableContractTag(ExpandLegacySchemaToContract(schema))
	}
	return DefaultStructuralExpectedReturn
}

// DeliverableSchemaTag returns legacy schema marker (deprecated — prefer DeliverableContractTag).
func DeliverableSchemaTag(schema DeliverableSchema) string {
	if schema == "" || schema == DeliverableSchemaNotApplicable {
		return ""
	}
	c := ExpandLegacySchemaToContract(schema)
	if c.ContractApplicable() {
		return DeliverableContractTag(c)
	}
	return "<deliverable_schema>" + string(schema) + "</deliverable_schema>"
}

// ParseDeliverableSchemaTag reads legacy schema tag; maps known names to contract dimensions.
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
	if raw == string(DeliverableSchemaNotApplicable) {
		return DeliverableSchemaNotApplicable
	}
	if c := ExpandLegacySchemaToContract(DeliverableSchema(raw)); c.ContractApplicable() {
		return DeliverableSchema(raw)
	}
	return DeliverableSchemaNotApplicable
}

// AcceptanceCriteriaFor returns machine-readable criteria for legacy schema or contract key.
func AcceptanceCriteriaFor(schema DeliverableSchema) string {
	return AcceptanceCriteriaForContract(ExpandLegacySchemaToContract(schema))
}

// RegisteredDeliverableSchemaEnum is deprecated; Strategic Plan uses deliverable_contract JSON.
func RegisteredDeliverableSchemaEnum() string {
	return "use_deliverable_contract_json"
}

// LookupRegisteredDeliverableSchema is deprecated.
func LookupRegisteredDeliverableSchema(raw string) (DeliverableSchema, bool) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == string(DeliverableSchemaNotApplicable) {
		return DeliverableSchemaNotApplicable, true
	}
	c := ExpandLegacySchemaToContract(DeliverableSchema(raw))
	if c.ContractApplicable() {
		return DeliverableSchema(raw), true
	}
	return "", false
}

// VerifyDeliverableSchema dispatches via legacy schema → contract expansion.
func VerifyDeliverableSchema(schema DeliverableSchema, summary, stopReason string) deliverableVerifyResult {
	return VerifyDeliverableContract(ExpandLegacySchemaToContract(schema), summary, stopReason)
}

func schemaNarrownessRank(schema DeliverableSchema) (int, bool) {
	if schema == "" || schema == DeliverableSchemaNotApplicable {
		return 0, true
	}
	c := ExpandLegacySchemaToContract(schema)
	if !c.ContractApplicable() {
		return 0, false
	}
	return contractNarrowness(c), true
}
