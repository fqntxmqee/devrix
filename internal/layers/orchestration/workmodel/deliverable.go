package workmodel

import (
	"strings"
)

// DeliverableSchema names legacy deliverable markers; prefer DeliverableContract.
type DeliverableSchema string

const DeliverableSchemaNotApplicable DeliverableSchema = "not_applicable"

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

// InferDeliverableSchema resolves legacy schema tag; prefer InferDeliverableContract.
func InferDeliverableSchema(_ *WorkItem, directive string, expectedReturn string) DeliverableSchema {
	if schema := ParseDeliverableSchemaTag(expectedReturn); schema != DeliverableSchemaNotApplicable {
		return schema
	}
	if schema := ParseDeliverableSchemaTag(directive); schema != DeliverableSchemaNotApplicable {
		return schema
	}
	if c := ParseDeliverableContractTag(expectedReturn); c.ContractApplicable() {
		return schemaForContract(c)
	}
	if c := ParseDeliverableContractTag(directive); c.ContractApplicable() {
		return schemaForContract(c)
	}
	return DeliverableSchemaNotApplicable
}

func schemaForContract(c DeliverableContract) DeliverableSchema {
	legacy := ExpandLegacySchemaToContract(FirstRegisteredDeliverableSchema())
	if c.Normalized().CacheKey() == legacy.CacheKey() {
		return FirstRegisteredDeliverableSchema()
	}
	return DeliverableSchema("legacy_contract")
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

// NarrowestSchema narrows deliverable contracts via legacy schema wrapper.
func NarrowestSchema(inferred, strategic DeliverableSchema) DeliverableSchema {
	if inferred == "" && strategic == "" {
		return ""
	}
	inC := ExpandLegacySchemaToContract(inferred)
	stC := ExpandLegacySchemaToContract(strategic)
	if !inC.ContractApplicable() && !stC.ContractApplicable() {
		if inferred == "" || inferred == DeliverableSchemaNotApplicable {
			if strategic == "" || strategic == DeliverableSchemaNotApplicable {
				return DeliverableSchemaNotApplicable
			}
		}
	}
	out := NarrowestContract(inC, stC)
	if !out.ContractApplicable() {
		return DeliverableSchemaNotApplicable
	}
	if inferred != "" && inferred != DeliverableSchemaNotApplicable {
		return inferred
	}
	if strategic != "" && strategic != DeliverableSchemaNotApplicable {
		return strategic
	}
	return schemaForContract(out)
}

// IsRegisteredDeliverableSchema reports whether a deliverable gate applies.
func IsRegisteredDeliverableSchema(s DeliverableSchema) bool {
	if s == "" || s == DeliverableSchemaNotApplicable {
		return false
	}
	return ExpandLegacySchemaToContract(s).ContractApplicable()
}

// IsApplicableDeliverableContract reports whether round owes deliverable verification.
func IsApplicableDeliverableContract(c DeliverableContract) bool {
	return c.ContractApplicable()
}
