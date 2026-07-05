package prompttags

import (
	"encoding/json"
	"strings"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// ParseRejectCode classifies a cross-round MUPS parse/budget rejection.
type ParseRejectCode string

const (
	RejectParseFail        ParseRejectCode = "parse_fail"
	RejectBudgetCap        ParseRejectCode = "budget_cap"
	RejectUncertaintyGate  ParseRejectCode = "uncertainty_gate"
	RejectScopeGate        ParseRejectCode = "scope_gate"
	RejectValidateEmpty    ParseRejectCode = "validate_empty"
)

// ParseRejectRecord is machine-readable feedback for the next-round user frame.
type ParseRejectRecord struct {
	Phase      string          `json:"phase"`
	Code       ParseRejectCode `json:"code"`
	Field      string          `json:"field,omitempty"`
	Message    string          `json:"message,omitempty"`
	Requested  int             `json:"requested,omitempty"`
	MaxAllowed int             `json:"max_allowed,omitempty"`
	Snippet    string          `json:"snippet,omitempty"`
}

// CompactJSON serializes the record as a single-line JSON object for lineframes.
func (r ParseRejectRecord) CompactJSON() string {
	if r.Phase == "" && r.Code == "" && r.Message == "" {
		return ""
	}
	b, err := json.Marshal(r)
	if err != nil {
		return ""
	}
	return string(b)
}

// ParseRejectRecordFromJSON parses a compact JSON line from a user frame field.
func ParseRejectRecordFromJSON(line string) (ParseRejectRecord, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return ParseRejectRecord{}, false
	}
	var r ParseRejectRecord
	if err := json.Unmarshal([]byte(line), &r); err != nil {
		return ParseRejectRecord{}, false
	}
	return r, true
}

// NewObserveParseReject builds an Observe-phase reject record.
func NewObserveParseReject(code ParseRejectCode, message, snippet string) ParseRejectRecord {
	return ParseRejectRecord{
		Phase:   string(contracts.MUPSPhaseObserve),
		Code:    code,
		Message: truncateRejectSnippet(message, 240),
		Snippet: truncateRejectSnippet(snippet, 120),
	}
}

// NewPlanParseReject builds a Plan-phase reject record.
func NewPlanParseReject(code ParseRejectCode, field, message string, requested, maxAllowed int) ParseRejectRecord {
	return ParseRejectRecord{
		Phase:      string(contracts.MUPSPhasePlan),
		Code:       code,
		Field:      field,
		Message:    truncateRejectSnippet(message, 240),
		Requested:  requested,
		MaxAllowed: maxAllowed,
	}
}

func truncateRejectSnippet(s string, max int) string {
	s = strings.TrimSpace(s)
	if s == "" || max <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}
