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
	findingsJSONFenceExtractRE = regexp.MustCompile("(?is)```json\\s*(\\{.*?)```")
	planningProseMarkers       = []string{
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
	var wrapper struct {
		Findings []DeliverableFinding `json:"findings"`
	}
	if err := json.Unmarshal(rawJSON, &wrapper); err != nil || len(wrapper.Findings) == 0 {
		return nil, false
	}
	findings := normalizeDeliverableFindings(wrapper.Findings)
	if len(findings) == 0 {
		return nil, false
	}
	return &DeliverablePayload{
		Findings: findings,
		Raw:      string(rawJSON),
	}, true
}

func extractDeliverableJSONObject(summary string) []byte {
	summary = strings.TrimSpace(summary)
	if m := findingsJSONFenceExtractRE.FindStringSubmatch(summary); len(m) == 2 {
		return []byte(strings.TrimSpace(m[1]))
	}
	start := strings.Index(summary, "{")
	end := strings.LastIndex(summary, "}")
	if start < 0 || end <= start {
		return nil
	}
	candidate := summary[start : end+1]
	if !findingsJSONMarkerRE.MatchString(candidate) {
		return nil
	}
	return []byte(candidate)
}

func normalizeDeliverableFindings(in []DeliverableFinding) []DeliverableFinding {
	out := make([]DeliverableFinding, 0, len(in))
	for _, f := range in {
		f = normalizeDeliverableFinding(f)
		if !findingQualityOK(f) {
			continue
		}
		out = append(out, f)
	}
	return out
}

func normalizeDeliverableFinding(f DeliverableFinding) DeliverableFinding {
	f.Severity = strings.ToUpper(strings.TrimSpace(f.Severity))
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

func findingQualityOK(f DeliverableFinding) bool {
	for _, s := range []string{f.Title, f.Message, f.Evidence, f.Impact, f.Recommendation} {
		if DetectPlanningMeta(s) || containsPlanningProse(strings.ToLower(s)) {
			return false
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
