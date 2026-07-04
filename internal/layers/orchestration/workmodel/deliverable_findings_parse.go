package workmodel

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const maxFindingFieldRunes = 800

var (
	findingsJSONMarkerRE = regexp.MustCompile(`(?i)"(?:findings|files_reviewed|scope)"\s*:`)
	lineRangeRE          = regexp.MustCompile(`(?i)(?:line_range|lines?)\s*[:=]?\s*"?(\d+)`)
	planningProseMarkers = []string{
		"the user wants me to",
		"let me start by",
		"let me first",
		"findings_json only",
		"i need to use only evidence",
	}
)

// ContainsFindingsJSONDraft reports whether text mixes planning prose with a
// findings_json blob (unsafe to stream verbatim to IM).
func ContainsFindingsJSONDraft(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	if !findingsJSONMarkerRE.MatchString(lower) && !strings.Contains(lower, "```json") {
		return false
	}
	return DetectPlanningMeta(text) || containsPlanningProse(lower)
}

// ShouldSuppressFindingsArtifactStream hides findings_json execute output from
// live IM text events when it is machine JSON or planning-heavy draft text.
func ShouldSuppressFindingsArtifactStream(round *WorkItemPipelineRound) bool {
	if round == nil || round.DeliverableContract.Normalized().Structure != DeliverableStructureFindingsJSON {
		return false
	}
	summary := strings.TrimSpace(round.ArtifactSummary)
	if summary == "" {
		return false
	}
	return ContainsFindingsJSONDraft(summary) || LooksLikeBareFindingsJSON(summary)
}

func LooksLikeBareFindingsJSON(text string) bool {
	text = strings.TrimSpace(text)
	return strings.HasPrefix(text, "{") && findingsJSONMarkerRE.MatchString(text)
}

func containsPlanningProse(lower string) bool {
	for _, m := range planningProseMarkers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

func parseFindingsJSONPayload(summary string) (*DeliverablePayload, bool) {
	rawJSON := extractDeliverableJSONObject(summary)
	if rawJSON == nil {
		return nil, false
	}
	findings, ok := decodeFindingsFromJSONObject(rawJSON, FindingParseStrict)
	if !ok {
		return nil, false
	}
	return &DeliverablePayload{
		Findings: findings,
		Raw:      string(rawJSON),
	}, true
}

func extractDeliverableJSONObject(summary string) []byte {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return nil
	}
	if body := extractJSONObjectFromMarkdownFence(summary); body != nil {
		return body
	}
	if body := extractJSONObjectWithFindingsMarker(summary); body != nil {
		return body
	}
	return extractFindingsJSONArrayFromCorruptSummary(summary)
}

func extractJSONObjectFromMarkdownFence(summary string) []byte {
	lower := strings.ToLower(summary)
	const fence = "```json"
	start := strings.Index(lower, fence)
	if start < 0 {
		return nil
	}
	rest := summary[start+len(fence):]
	if idx := strings.Index(rest, "{"); idx >= 0 {
		rest = rest[idx:]
	}
	return extractJSONObjectWithFindingsMarker(rest)
}

func extractJSONObjectWithFindingsMarker(summary string) []byte {
	summary = strings.TrimSpace(summary)
	for i := 0; i < len(summary); i++ {
		if summary[i] != '{' {
			continue
		}
		end := strings.LastIndex(summary[i:], "}")
		if end < 0 {
			continue
		}
		end += i
		candidate := summary[i : end+1]
		if !findingsJSONMarkerRE.MatchString(candidate) {
			continue
		}
		if !json.Valid([]byte(candidate)) {
			continue
		}
		return []byte(candidate)
	}
	return nil
}

func extractFindingsJSONArrayFromCorruptSummary(summary string) []byte {
	idx := strings.Index(strings.ToLower(summary), `"findings"`)
	if idx < 0 {
		return nil
	}
	rest := summary[idx:]
	arrStart := strings.Index(rest, "[")
	if arrStart < 0 {
		return nil
	}
	rest = rest[arrStart+1:]
	var items []json.RawMessage
	for len(rest) > 0 {
		rest = strings.TrimLeft(rest, " \t\r\n,")
		if len(rest) == 0 || rest[0] == ']' {
			break
		}
		if rest[0] != '{' {
			break
		}
		end := matchingJSONObjectEnd(rest)
		if end < 0 {
			break
		}
		candidate := rest[:end+1]
		if json.Valid([]byte(candidate)) {
			items = append(items, json.RawMessage(candidate))
		}
		rest = rest[end+1:]
	}
	if len(items) == 0 {
		return nil
	}
	wrapper := struct {
		Findings []json.RawMessage `json:"findings"`
	}{Findings: items}
	out, err := json.Marshal(wrapper)
	if err != nil {
		return nil
	}
	return out
}

func matchingJSONObjectEnd(s string) int {
	depth := 0
	inString := false
	escaped := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func normalizeFindingSeverity(sev string) string {
	switch strings.ToLower(strings.TrimSpace(sev)) {
	case "critical", "high", "p0":
		return "P0"
	case "medium", "moderate", "low", "minor", "p1", "p2":
		return "P1"
	default:
		return strings.ToUpper(strings.TrimSpace(sev))
	}
}

func normalizeDeliverableFindings(in []DeliverableFinding) []DeliverableFinding {
	out := make([]DeliverableFinding, 0, len(in))
	for _, f := range in {
		f = normalizeDeliverableFinding(f)
		if !findingQualityOK(f, FindingParseStrict) {
			continue
		}
		out = append(out, f)
	}
	return out
}

func normalizeDeliverableFinding(f DeliverableFinding) DeliverableFinding {
	f.Severity = normalizeFindingSeverity(f.Severity)
	f.Title = strings.TrimSpace(f.Title)
	f.Message = strings.TrimSpace(f.Message)
	f.Evidence = strings.TrimSpace(f.Evidence)
	f.Impact = strings.TrimSpace(f.Impact)
	f.Recommendation = strings.TrimSpace(f.Recommendation)
	f.File = strings.TrimSpace(f.File)
	if f.Line <= 0 {
		if n, err := strconv.Atoi(strings.TrimSpace(f.Citation)); err == nil && n > 0 {
			f.Line = n
		}
	}
	if f.Line <= 0 && strings.Contains(f.Citation, "-") {
		if n, err := strconv.Atoi(strings.TrimSpace(strings.Split(f.Citation, "-")[0])); err == nil && n > 0 {
			f.Line = n
		}
	}
	if f.Line <= 0 {
		for _, field := range []string{f.Evidence, f.Message, f.Title} {
			if m := lineRangeRE.FindStringSubmatch(field); len(m) > 1 {
				if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
					f.Line = n
					break
				}
			}
		}
	}
	if f.File == "" {
		if m := fileLineCitationRE.FindString(f.Evidence); m != "" {
			f.File, f.Line = splitFileLineCitation(m)
		}
	}
	if f.Title == "" {
		f.Title = f.Message
	}
	return f
}

func findingQualityOK(f DeliverableFinding, mode FindingParseMode) bool {
	for _, s := range []string{f.Title, f.Message, f.Evidence, f.Impact, f.Recommendation} {
		if mode == FindingParseStrict {
			if DetectPlanningMeta(s) || containsPlanningProse(strings.ToLower(s)) {
				return false
			}
		}
		if utf8.RuneCountInString(s) > maxFindingFieldRunes {
			return false
		}
	}
	if strings.TrimSpace(f.File) == "" {
		return false
	}
	sev := strings.ToUpper(strings.TrimSpace(f.Severity))
	if sev != "P0" && sev != "P1" {
		return false
	}
	if strings.TrimSpace(f.Title) == "" {
		return false
	}
	return true
}

// FindingsPayloadPresentable reports whether structured findings are safe to
// show on IM cards (complete or best-effort partial).
func FindingsPayloadPresentable(p *DeliverablePayload) bool {
	return p != nil && len(normalizeDeliverableFindings(p.Findings)) > 0
}
