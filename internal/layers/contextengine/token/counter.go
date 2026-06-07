package token

import (
	"strings"
	"unicode/utf8"

	"github.com/devrix/devrix/internal/shared/types"
)

// Counter estimates token counts (V1: char-based heuristic, ~4 chars/token).
type Counter struct{}

// NewCounter creates a token counter.
func NewCounter() *Counter {
	return &Counter{}
}

// Count estimates tokens for a single string.
func (c *Counter) Count(text string) int {
	if text == "" {
		return 0
	}
	// Heuristic: average ~4 characters per token for English/code mix.
	n := utf8.RuneCountInString(text)
	tokens := n / 4
	if tokens == 0 && n > 0 {
		return 1
	}
	return tokens
}

// CountMessages sums tokens across messages.
func (c *Counter) CountMessages(msgs []types.Message) int {
	total := 0
	for _, m := range msgs {
		total += c.Count(m.Content)
		total += 4 // role overhead
	}
	return total
}

// TruncateToTokens truncates text to approximate max tokens.
func (c *Counter) TruncateToTokens(text string, maxTokens int) string {
	if maxTokens <= 0 {
		return ""
	}
	approxChars := maxTokens * 4
	if c.Count(text) <= maxTokens {
		return text
	}
	runes := []rune(text)
	if len(runes) <= approxChars {
		return text
	}
	truncated := string(runes[:approxChars])
	return strings.TrimSpace(truncated) + "\n...[truncated]"
}
