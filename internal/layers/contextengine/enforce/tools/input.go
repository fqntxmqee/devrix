package tools

import (
	"encoding/json"
	"strconv"
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

// ToolInputIntDefault parses an integer field from a tool argument
// string (JSON or plain). Returns def when the field is missing,
// empty, or unparseable. Mirrors ToolInputString for ints.
//
// DM-20260702-008 / D2-S15-A02-T10: used by read_file offset/limit
// defaults (Offset=0, Limit=8192). The plain-string fallback is fine
// because ParseToolInput coerces JSON ints to their string repr.
func ToolInputIntDefault(input, field string, def int) int {
	fields := ParseToolInput(input)
	v, ok := fields[field]
	if !ok || strings.TrimSpace(v) == "" {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return n
}
