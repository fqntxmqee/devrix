package prompttags

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Wrap serializes v into <name>...</name> per the tag's EncodingProfile.
// Returns empty string when v is the zero value for scalar string tags.
func Wrap[T any](name TagName, v T) string {
	spec, ok := Lookup(name)
	if !ok || spec.Profile == EncodingWholeBody {
		return ""
	}
	payload, ok := encodePayload(spec, v)
	if !ok || payload == "" {
		return ""
	}
	return envelopeOpen(name) + payload + envelopeClose(name)
}

func encodePayload[T any](spec TagSpec, v T) (string, bool) {
	switch spec.Profile {
	case EncodingLineField:
		lines, ok := any(v).([]string)
		if !ok {
			return "", false
		}
		var trimmed []string
		for _, line := range lines {
			if s := strings.TrimSpace(line); s != "" {
				trimmed = append(trimmed, s)
			}
		}
		if len(trimmed) == 0 {
			return "", false
		}
		return strings.Join(trimmed, "\n"), true
	case EncodingEnvelope:
		switch val := any(v).(type) {
		case string:
			s := strings.TrimSpace(val)
			if s == "" {
				return "", false
			}
			return s, true
		default:
			b, err := json.Marshal(v)
			if err != nil {
				return "", false
			}
			if string(b) == "null" {
				return "", false
			}
			return string(b), true
		}
	default:
		return "", false
	}
}

// ExtractOne parses the first <name>...</name> block from content.
func ExtractOne[T any](name TagName, content string) (T, bool) {
	var zero T
	spec, ok := Lookup(name)
	if !ok {
		return zero, false
	}
	raw, ok := extractRawEnvelope(name, content)
	if !ok {
		return zero, false
	}
	out, ok := decodePayload[T](spec, raw)
	if !ok {
		return zero, false
	}
	return out, true
}

func decodePayload[T any](spec TagSpec, raw string) (T, bool) {
	var zero T
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return zero, false
	}
	switch spec.Profile {
	case EncodingLineField:
		lines := splitNonEmptyLines(raw)
		if len(lines) == 0 {
			return zero, false
		}
		switch any(zero).(type) {
		case []string:
			return any(lines).(T), true
		default:
			return zero, false
		}
	case EncodingEnvelope:
		switch any(zero).(type) {
		case string:
			return any(raw).(T), true
		default:
			var out T
			if err := json.Unmarshal([]byte(raw), &out); err != nil {
				return zero, false
			}
			return out, true
		}
	default:
		return zero, false
	}
}

// ExtractAll returns raw inner payloads for every registered envelope tag found in content.
// When phase is non-empty, only tags applicable to that MUPS phase are extracted.
func ExtractAll(content string, phase string) map[TagName]string {
	out := make(map[TagName]string)
	for name, spec := range MUPSRegistry {
		if spec.Profile == EncodingWholeBody {
			continue
		}
		if !TagAppliesToPhase(name, phase) {
			continue
		}
		if raw, ok := extractRawEnvelope(name, content); ok {
			out[name] = raw
		}
	}
	return out
}

func envelopeOpen(name TagName) string  { return "<" + string(name) + ">" }
func envelopeClose(name TagName) string { return "</" + string(name) + ">" }

func envelopeRE(name TagName) *regexp.Regexp {
	return regexp.MustCompile(`(?s)<` + regexp.QuoteMeta(string(name)) + `>(.*?)</` + regexp.QuoteMeta(string(name)) + `>`)
}

func extractRawEnvelope(name TagName, content string) (string, bool) {
	m := envelopeRE(name).FindStringSubmatch(content)
	if len(m) < 2 {
		return "", false
	}
	raw := strings.TrimSpace(m[1])
	if raw == "" {
		return "", false
	}
	return raw, true
}

func splitNonEmptyLines(raw string) []string {
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// FormatEnvelopeBlock wraps pre-encoded payload (used when callers pre-validate).
func FormatEnvelopeBlock(name TagName, payload string) string {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return ""
	}
	return fmt.Sprintf("%s%s%s", envelopeOpen(name), payload, envelopeClose(name))
}
