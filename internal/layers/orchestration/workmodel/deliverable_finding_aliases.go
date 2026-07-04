package workmodel

import (
	"encoding/json"
	"strconv"
	"strings"
)

// DeliverableFindingFieldAliases maps canonical DeliverableFinding fields to
// accepted JSON keys (single registry — extend here, not at call sites).
// DM-20260704-001 CC-U2: avoids non-exhaustive per-task field hardcoding.
var DeliverableFindingFieldAliases = map[string][]string{
	"severity":       {"severity", "level", "priority"},
	"title":          {"title", "issue", "name", "summary"},
	"message":        {"message", "description", "detail", "details"},
	"file":           {"file", "path", "filepath", "file_path", "location"},
	"line":           {"line", "lineno", "line_no", "line_number", "line_range"},
	"evidence":       {"evidence", "proof", "citation_text"},
	"impact":         {"impact", "risk", "effect"},
	"recommendation": {"recommendation", "fix", "suggestion", "action"},
	"citation":       {"citation", "location"},
	"id":             {"id", "finding_id"},
}

// FindingParseMode selects strict verify vs salvage extraction quality gates.
type FindingParseMode int

const (
	FindingParseStrict FindingParseMode = iota
	FindingParseSalvage
)

func decodeFindingsFromJSONObject(raw []byte, mode FindingParseMode) ([]DeliverableFinding, bool) {
	raw = []byte(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return nil, false
	}
	var wrapper struct {
		Findings []json.RawMessage `json:"findings"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil || len(wrapper.Findings) == 0 {
		return nil, false
	}
	out := make([]DeliverableFinding, 0, len(wrapper.Findings))
	for _, item := range wrapper.Findings {
		f, ok := coalesceFindingJSON(item)
		if !ok {
			continue
		}
		f = normalizeDeliverableFinding(f)
		if !findingQualityOK(f, mode) {
			continue
		}
		out = append(out, f)
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

func coalesceFindingJSON(raw json.RawMessage) (DeliverableFinding, bool) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil || len(m) == 0 {
		return DeliverableFinding{}, false
	}
	var f DeliverableFinding
	_ = json.Unmarshal(raw, &f)
	for canonical, aliases := range DeliverableFindingFieldAliases {
		if findingFieldPopulated(f, canonical) {
			continue
		}
		for _, alias := range aliases {
			v, ok := m[alias]
			if !ok {
				continue
			}
			applyFindingField(&f, canonical, v)
			break
		}
	}
	if findingStructEmpty(f) {
		return DeliverableFinding{}, false
	}
	return f, true
}

func findingFieldPopulated(f DeliverableFinding, canonical string) bool {
	switch canonical {
	case "severity":
		return strings.TrimSpace(f.Severity) != ""
	case "title":
		return strings.TrimSpace(f.Title) != ""
	case "message":
		return strings.TrimSpace(f.Message) != ""
	case "file":
		return strings.TrimSpace(f.File) != ""
	case "line":
		return f.Line > 0
	case "evidence":
		return strings.TrimSpace(f.Evidence) != ""
	case "impact":
		return strings.TrimSpace(f.Impact) != ""
	case "recommendation":
		return strings.TrimSpace(f.Recommendation) != ""
	case "citation":
		return strings.TrimSpace(f.Citation) != ""
	case "id":
		return strings.TrimSpace(f.ID) != ""
	default:
		return false
	}
}

func findingStructEmpty(f DeliverableFinding) bool {
	return strings.TrimSpace(f.Title) == "" &&
		strings.TrimSpace(f.Message) == "" &&
		strings.TrimSpace(f.File) == "" &&
		strings.TrimSpace(f.Evidence) == "" &&
		strings.TrimSpace(f.Severity) == ""
}

func applyFindingField(f *DeliverableFinding, canonical string, raw json.RawMessage) {
	if f == nil || len(raw) == 0 {
		return
	}
	s := strings.TrimSpace(string(raw))
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		var str string
		if json.Unmarshal(raw, &str) == nil {
			s = strings.TrimSpace(str)
		}
	}
	switch canonical {
	case "severity", "title", "message", "file", "evidence", "impact", "recommendation", "citation", "id":
		setFindingString(f, canonical, s)
	case "line":
		if n, ok := parseFlexibleInt(raw); ok {
			f.Line = n
		} else if strings.Contains(s, "-") {
			if n, err := strconv.Atoi(strings.TrimSpace(strings.Split(s, "-")[0])); err == nil && n > 0 {
				f.Line = n
			}
		}
	}
}

func setFindingString(f *DeliverableFinding, canonical, s string) {
	switch canonical {
	case "severity":
		f.Severity = s
	case "title":
		f.Title = s
	case "message":
		f.Message = s
	case "file":
		f.File = s
	case "evidence":
		f.Evidence = s
	case "impact":
		f.Impact = s
	case "recommendation":
		f.Recommendation = s
	case "citation":
		f.Citation = s
	case "id":
		f.ID = s
	}
}

func parseFlexibleInt(raw json.RawMessage) (int, bool) {
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, true
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return int(f), true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			return n, true
		}
	}
	return 0, false
}
