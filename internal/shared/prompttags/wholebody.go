package prompttags

import (
	"encoding/json"
	"regexp"
	"strings"
)

var wholeBodyFenceRE = regexp.MustCompile("(?is)^```(?:json)?\\s*(.*?)\\s*```$")

// ParseWholeBody unmarshals JSON from bare or fenced whole-body content.
// Accepts payloads starting with { or [ after optional ```json fence stripping.
func ParseWholeBody[T any](content string) (T, bool) {
	var zero T
	raw := stripWholeBodyFence(strings.TrimSpace(content))
	if raw == "" {
		return zero, false
	}
	if !strings.HasPrefix(raw, "{") && !strings.HasPrefix(raw, "[") {
		return zero, false
	}
	var out T
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return zero, false
	}
	return out, true
}

func stripWholeBodyFence(s string) string {
	if m := wholeBodyFenceRE.FindStringSubmatch(s); len(m) >= 2 {
		return strings.TrimSpace(m[1])
	}
	return strings.TrimSpace(s)
}
