package workmodel

import (
	"encoding/json"
	"sort"
	"strings"
)

// Deliverable dimension enums — exhaustive category sets (DM-20260703-001).
// LLM composes a contract from these categories; Go never registers named
// business recipes like p0_p1_file_line.

type DeliverableCitation string

const (
	DeliverableCitationNone     DeliverableCitation = "none"
	DeliverableCitationFileLine DeliverableCitation = "file_line"
)

type DeliverableSeverity string

const (
	DeliverableSeverityNone DeliverableSeverity = "none"
	DeliverableSeverityP0P1 DeliverableSeverity = "p0_p1"
)

type DeliverableStructure string

const (
	DeliverableStructureFreeText     DeliverableStructure = "free_text"
	DeliverableStructureFindingsJSON DeliverableStructure = "findings_json"
)

type DeliverableReject string

const (
	DeliverableRejectPlanningMeta DeliverableReject = "planning_meta"
)

// DeliverableContract is the machine-readable deliverable I/O contract.
type DeliverableContract struct {
	Citation  DeliverableCitation  `json:"citation,omitempty"`
	Severity  DeliverableSeverity  `json:"severity,omitempty"`
	Structure DeliverableStructure `json:"structure,omitempty"`
	MinRunes  int                  `json:"min_runes,omitempty"`
	Reject    []DeliverableReject  `json:"reject,omitempty"`
}

const (
	deliverableContractOpen  = "<deliverable_contract>"
	deliverableContractClose = "</deliverable_contract>"
)

// ContractApplicable reports whether verify/execute gates apply.
func (c DeliverableContract) ContractApplicable() bool {
	n := c.Normalized()
	if n.Citation != DeliverableCitationNone && n.Citation != "" {
		return true
	}
	if n.Severity != DeliverableSeverityNone && n.Severity != "" {
		return true
	}
	if n.Structure == DeliverableStructureFindingsJSON {
		return true
	}
	if n.MinRunes > 0 {
		return true
	}
	return len(n.Reject) > 0
}

func (c DeliverableContract) IsEmpty() bool {
	return !c.ContractApplicable()
}

// Normalized returns canonical defaults for unset fields.
func (c DeliverableContract) Normalized() DeliverableContract {
	out := c
	if out.Citation == "" {
		out.Citation = DeliverableCitationNone
	}
	if out.Severity == "" {
		out.Severity = DeliverableSeverityNone
	}
	if out.Structure == "" {
		out.Structure = DeliverableStructureFreeText
	}
	out.Reject = append([]DeliverableReject(nil), out.Reject...)
	sort.Slice(out.Reject, func(i, j int) bool { return out.Reject[i] < out.Reject[j] })
	return out
}

// CacheKey is a stable string for escape hashes and logging.
func (c DeliverableContract) CacheKey() string {
	b, err := json.Marshal(c.Normalized())
	if err != nil {
		return ""
	}
	return string(b)
}

// DeliverableContractTag serializes the contract for downlink / Execute.
func DeliverableContractTag(c DeliverableContract) string {
	if !c.ContractApplicable() {
		return ""
	}
	b, err := json.Marshal(c.Normalized())
	if err != nil {
		return ""
	}
	return deliverableContractOpen + string(b) + deliverableContractClose
}

// ParseDeliverableContractTag reads <deliverable_contract>{json}</deliverable_contract>.
func ParseDeliverableContractTag(s string) DeliverableContract {
	s = strings.TrimSpace(s)
	start := strings.Index(s, deliverableContractOpen)
	if start < 0 {
		return DeliverableContract{}
	}
	end := strings.Index(s[start:], deliverableContractClose)
	if end < 0 {
		return DeliverableContract{}
	}
	raw := strings.TrimSpace(s[start+len(deliverableContractOpen) : start+end])
	var c DeliverableContract
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return DeliverableContract{}
	}
	return c.Normalized()
}

// InferDeliverableContract resolves contract from explicit tags only.
func InferDeliverableContract(_ *WorkItem, directive, expectedReturn string) DeliverableContract {
	if c := ParseDeliverableContractTag(expectedReturn); c.ContractApplicable() {
		return c
	}
	if c := ParseDeliverableContractTag(directive); c.ContractApplicable() {
		return c
	}
	if schema := ParseDeliverableSchemaTag(expectedReturn); schema != DeliverableSchemaNotApplicable {
		return ExpandLegacySchemaToContract(schema)
	}
	if schema := ParseDeliverableSchemaTag(directive); schema != DeliverableSchemaNotApplicable {
		return ExpandLegacySchemaToContract(schema)
	}
	return DeliverableContract{}
}

const legacyReviewSchemaName = "p0_p1_file_line"

// FirstRegisteredDeliverableSchema returns the legacy review schema name for tests.
func FirstRegisteredDeliverableSchema() DeliverableSchema {
	return DeliverableSchema(legacyReviewSchemaName)
}

// ExpandLegacySchemaToContract maps deprecated schema names to dimension combos.
func ExpandLegacySchemaToContract(schema DeliverableSchema) DeliverableContract {
	switch strings.ToLower(strings.TrimSpace(string(schema))) {
	case legacyReviewSchemaName:
		return DeliverableContract{
			Citation: DeliverableCitationFileLine,
			Severity: DeliverableSeverityP0P1,
			Reject:   []DeliverableReject{DeliverableRejectPlanningMeta},
		}
	default:
		return DeliverableContract{}
	}
}

// NarrowestContract keeps the more constrained contract (LLM may narrow, not widen).
func NarrowestContract(inferred, strategic DeliverableContract) DeliverableContract {
	if !strategic.ContractApplicable() {
		return inferred
	}
	if !inferred.ContractApplicable() {
		return strategic
	}
	out := inferred
	if rankCitation(strategic.Citation) > rankCitation(out.Citation) {
		out.Citation = strategic.Citation
	}
	if rankSeverity(strategic.Severity) > rankSeverity(out.Severity) {
		out.Severity = strategic.Severity
	}
	if rankStructure(strategic.Structure) > rankStructure(out.Structure) {
		out.Structure = strategic.Structure
	}
	if strategic.MinRunes > out.MinRunes {
		out.MinRunes = strategic.MinRunes
	}
	out.Reject = unionReject(out.Reject, strategic.Reject)
	return out.Normalized()
}

func unionReject(a, b []DeliverableReject) []DeliverableReject {
	seen := map[DeliverableReject]struct{}{}
	for _, r := range a {
		seen[r] = struct{}{}
	}
	for _, r := range b {
		seen[r] = struct{}{}
	}
	out := make([]DeliverableReject, 0, len(seen))
	for r := range seen {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func rankCitation(c DeliverableCitation) int {
	switch c {
	case DeliverableCitationFileLine:
		return 10
	default:
		return 0
	}
}

func rankSeverity(s DeliverableSeverity) int {
	switch s {
	case DeliverableSeverityP0P1:
		return 10
	default:
		return 0
	}
}

func rankStructure(s DeliverableStructure) int {
	switch s {
	case DeliverableStructureFindingsJSON:
		return 10
	case DeliverableStructureFreeText:
		return 5
	default:
		return 0
	}
}

func contractNarrowness(c DeliverableContract) int {
	c = c.Normalized()
	return rankCitation(c.Citation) + rankSeverity(c.Severity) + rankStructure(c.Structure) + c.MinRunes/100
}

// ContractDimensionEnum documents allowed dimension values for Strategic Plan prompts.
func ContractDimensionEnum() map[string][]string {
	return map[string][]string{
		"citation":  {string(DeliverableCitationNone), string(DeliverableCitationFileLine)},
		"severity":  {string(DeliverableSeverityNone), string(DeliverableSeverityP0P1)},
		"structure": {string(DeliverableStructureFreeText), string(DeliverableStructureFindingsJSON)},
		"reject":    {string(DeliverableRejectPlanningMeta)},
	}
}

// ContractDimensionPromptDoc serializes allowed dimension enums for Strategic Plan prompts.
func ContractDimensionPromptDoc() string {
	b, err := json.Marshal(ContractDimensionEnum())
	if err != nil {
		return "{}"
	}
	return string(b)
}

const maxStrategicContractMinRunes = 500

const (
	deliverableFormatOpen  = "<deliverable_format>"
	deliverableFormatClose = "</deliverable_format>"
)

// ClampDeliverableContract caps LLM-proposed min_runes to a sane verify bound.
func ClampDeliverableContract(c DeliverableContract) DeliverableContract {
	c = c.Normalized()
	if c.MinRunes > maxStrategicContractMinRunes {
		c.MinRunes = maxStrategicContractMinRunes
	}
	return c
}

// DeliverableFinalAnswerHint is the machine tag for Execute final-turn output shape.
func DeliverableFinalAnswerHint(c DeliverableContract) string {
	if c.Normalized().Structure == DeliverableStructureFindingsJSON {
		return deliverableFormatOpen + "findings_json_only" + deliverableFormatClose
	}
	return ""
}

// DefaultTestDeliverableContract is the standard review contract for tests.
func DefaultTestDeliverableContract() DeliverableContract {
	return DeliverableContract{
		Citation: DeliverableCitationFileLine,
		Severity: DeliverableSeverityP0P1,
		Reject:   []DeliverableReject{DeliverableRejectPlanningMeta},
	}
}

// RollupDeliverableContract is the standard rollup synthesis contract.
func RollupDeliverableContract() DeliverableContract {
	return DeliverableContract{
		Severity: DeliverableSeverityP0P1,
		MinRunes: 500,
		Reject:   []DeliverableReject{DeliverableRejectPlanningMeta},
	}
}
