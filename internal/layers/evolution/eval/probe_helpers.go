package eval

import (
	"strings"
	"unicode"
)

func stringFromInput(input map[string]any, key string) string {
	if input == nil {
		return ""
	}
	v, ok := input[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func stringSliceFromInput(input map[string]any, key string) []string {
	if input == nil {
		return nil
	}
	return coerceStringSlice(input[key])
}

func stringSliceFromExpectation(expectation map[string]any, key string) []string {
	if expectation == nil {
		return nil
	}
	return coerceStringSlice(expectation[key])
}

func coerceStringSlice(raw any) []string {
	switch v := raw.(type) {
	case []string:
		return append([]string(nil), v...)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				continue
			}
			out = append(out, s)
		}
		return out
	default:
		return nil
	}
}

func messagesFromInput(input map[string]any, key string) string {
	if s := stringFromInput(input, key); s != "" {
		return s
	}
	parts := stringSliceFromInput(input, key)
	return strings.Join(parts, "\n")
}

func instructionFollowingRate(text string, mustFollow []string) float64 {
	if len(mustFollow) == 0 {
		return 1
	}
	if text == "" {
		return 0
	}
	lower := strings.ToLower(text)
	kept := 0
	for _, item := range mustFollow {
		if strings.Contains(lower, strings.ToLower(item)) {
			kept++
		}
	}
	return float64(kept) / float64(len(mustFollow))
}

func isolationRate(text string, forbidden []string) float64 {
	if len(forbidden) == 0 {
		return 1
	}
	if text == "" {
		return 1
	}
	lower := strings.ToLower(text)
	leaks := 0
	for _, item := range forbidden {
		if strings.Contains(lower, strings.ToLower(item)) {
			leaks++
		}
	}
	return 1 - float64(leaks)/float64(len(forbidden))
}

func wordJaccard(a, b string) float64 {
	setA := tokenSet(a)
	setB := tokenSet(b)
	if len(setA) == 0 && len(setB) == 0 {
		return 1
	}
	if len(setA) == 0 || len(setB) == 0 {
		return 0
	}
	intersection := 0
	for token := range setA {
		if setB[token] {
			intersection++
		}
	}
	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func tokenSet(text string) map[string]bool {
	tokens := make(map[string]bool)
	for _, word := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	}) {
		if word == "" {
			continue
		}
		tokens[word] = true
	}
	return tokens
}

func conservativeMin(scores ...float64) float64 {
	if len(scores) == 0 {
		return 0
	}
	min := scores[0]
	for _, s := range scores[1:] {
		if s < min {
			min = s
		}
	}
	return min
}

func avgScore(scores ...float64) float64 {
	if len(scores) == 0 {
		return 0
	}
	total := 0.0
	for _, s := range scores {
		total += s
	}
	return total / float64(len(scores))
}
