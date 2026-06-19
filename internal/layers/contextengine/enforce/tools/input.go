package tools

import (
	"encoding/json"
	"strings"
)

// ParseToolInput parses JSON or plain tool argument strings.
func ParseToolInput(input string) map[string]string {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}
	if !strings.HasPrefix(input, "{") {
		return map[string]string{"command": input, "path": input}
	}

	var generic map[string]json.RawMessage
	if err := json.Unmarshal([]byte(input), &generic); err != nil {
		return map[string]string{"command": input, "path": input}
	}

	out := make(map[string]string, len(generic))
	for k, raw := range generic {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			out[k] = s
			continue
		}
		out[k] = strings.TrimSpace(string(raw))
	}
	return out
}

// ToolInputString returns the first non-empty field for the given keys.
func ToolInputString(input string, keys ...string) string {
	fields := ParseToolInput(input)
	for _, key := range keys {
		if v := strings.TrimSpace(fields[key]); v != "" {
			return v
		}
	}
	return ""
}

func firstNonEmpty(fields map[string]string, keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(fields[key]); v != "" {
			return v
		}
	}
	return ""
}
