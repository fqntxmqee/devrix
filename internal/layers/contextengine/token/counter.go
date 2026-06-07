package token

import (
	"strings"
	"unicode/utf8"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

const heuristicEncoding = "heuristic-char-div-4"

// Counter estimates token counts (V1 heuristic: ~4 chars/token).
type Counter struct{}

// NewCounter creates a heuristic token counter.
func NewCounter() *Counter {
	return &Counter{}
}

// CountText estimates tokens for a single string.
func (c *Counter) CountText(text string) int {
	return c.countRunes(text)
}

// Count estimates tokens for a single string (alias for CountText).
func (c *Counter) Count(text string) int {
	return c.CountText(text)
}

// CountMessages sums tokens across messages.
func (c *Counter) CountMessages(msgs []types.Message) int {
	total := 0
	for _, m := range msgs {
		total += c.CountText(m.Content)
		total += 4 // role overhead
	}
	return total
}

// CountWithSystemPrompt includes system prompt and messages.
func (c *Counter) CountWithSystemPrompt(systemPrompt string, messages []types.Message) int {
	total := c.CountMessages(messages)
	if systemPrompt != "" {
		total += c.CountText(systemPrompt) + 4
	}
	return total
}

// TruncateToTokens truncates text to approximate max tokens.
func (c *Counter) TruncateToTokens(text string, maxTokens int) string {
	if maxTokens <= 0 {
		return ""
	}
	approxChars := maxTokens * 4
	if c.CountText(text) <= maxTokens {
		return text
	}
	runes := []rune(text)
	if len(runes) <= approxChars {
		return text
	}
	truncated := string(runes[:approxChars])
	return strings.TrimSpace(truncated) + "\n...[truncated]"
}

// EncodingForModel returns the heuristic encoding identifier.
func (c *Counter) EncodingForModel(model string) string {
	_ = model
	return heuristicEncoding
}

func (c *Counter) countRunes(text string) int {
	if text == "" {
		return 0
	}
	n := utf8.RuneCountInString(text)
	tokens := n / 4
	if tokens == 0 && n > 0 {
		return 1
	}
	return tokens
}

var _ contracts.ITokenCounter = (*Counter)(nil)
