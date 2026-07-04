package textutil

import (
	"regexp"
	"strings"
)

var (
	findingsJSONMarkerRE = regexp.MustCompile(`(?i)"(?:findings|files_reviewed|scope)"\s*:`)
	findingsJSONFenceRE  = regexp.MustCompile("(?is)```json\\s*.*?(?:```|$)")
)

// LooksLikeFindingsJSONStream reports whether a streaming chunk is likely raw
// findings_json machine output that should not appear on IM reply cards.
func LooksLikeFindingsJSONStream(chunk string) bool {
	chunk = strings.TrimSpace(chunk)
	if chunk == "" {
		return false
	}
	if strings.HasPrefix(chunk, "{") || strings.HasPrefix(chunk, "[") {
		return true
	}
	if strings.HasPrefix(chunk, "}") || strings.HasPrefix(chunk, "]") {
		return true
	}
	if strings.HasPrefix(chunk, `"`) && findingsJSONMarkerRE.MatchString(chunk) {
		return true
	}
	if strings.Contains(chunk, `"severity"`) && strings.Contains(chunk, `"`) {
		return true
	}
	return findingsJSONMarkerRE.MatchString(chunk)
}

// StripFindingsJSONBlocks removes fenced or bare findings_json blobs from
// static assistant text before IM render.
func StripFindingsJSONBlocks(text string) string {
	out := findingsJSONFenceRE.ReplaceAllString(text, "")
	for {
		start := strings.Index(out, "{")
		if start < 0 {
			break
		}
		end := strings.LastIndex(out, "}")
		if end <= start || !findingsJSONMarkerRE.MatchString(out[start:end+1]) {
			break
		}
		prefix := strings.TrimSpace(out[:start])
		suffix := strings.TrimSpace(out[end+1:])
		switch {
		case prefix == "":
			out = suffix
		case suffix == "":
			out = prefix
		default:
			out = prefix + "\n" + suffix
		}
	}
	return strings.TrimSpace(out)
}
