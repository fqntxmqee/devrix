package token

import (
	"strings"
	"sync"

	"github.com/devrix/devrix/internal/shared/contracts"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/pkoukk/tiktoken-go"
)

const (
	defaultEncoding   = "cl100k_base"
	tokensPerMessage = 3
	tokensPerName    = 1
)

// Counter implements contracts.ITokenCounter using cl100k_base (tiktoken).
type Counter struct {
	mu            sync.RWMutex
	encoding      *tiktoken.Tiktoken
	cjkMultiplier float64
}

// NewCounter creates a cl100k_base token counter (embedded BPE, no network).
func NewCounter() (*Counter, error) {
	ensureEmbeddedBPELoader()
	enc, err := tiktoken.GetEncoding(defaultEncoding)
	if err != nil {
		return nil, err
	}
	return &Counter{encoding: enc, cjkMultiplier: 1.0}, nil
}

// WithCJKMultiplier sets a multiplier applied when text contains CJK characters.
func (c *Counter) WithCJKMultiplier(multiplier float64) *Counter {
	if multiplier > 0 {
		c.cjkMultiplier = multiplier
	}
	return c
}

// CountText returns the token count for plain text.
func (c *Counter) CountText(text string) int {
	if text == "" {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	count := len(c.encoding.Encode(text, nil, nil))
	if c.cjkMultiplier > 1.0 && containsCJK(text) {
		count = int(float64(count) * c.cjkMultiplier)
	}
	return count
}

func containsCJK(s string) bool {
	for _, r := range s {
		if (r >= 0x4E00 && r <= 0x9FFF) ||
			(r >= 0x3400 && r <= 0x4DBF) ||
			(r >= 0x3000 && r <= 0x303F) {
			return true
		}
	}
	return false
}

// CountMessages sums tokens across chat messages including role overhead.
func (c *Counter) CountMessages(messages []types.Message) int {
	total := 0
	for _, m := range messages {
		total += tokensPerMessage
		total += c.CountText(string(m.Role))
		total += c.CountText(m.Content)
		if name, ok := m.Metadata["name"]; ok && name != "" {
			total += tokensPerName
			total += c.CountText(name)
		}
	}
	total += 3 // reply priming
	return total
}

// CountWithSystemPrompt includes system prompt and messages.
func (c *Counter) CountWithSystemPrompt(systemPrompt string, messages []types.Message) int {
	total := c.CountMessages(messages)
	if systemPrompt != "" {
		total += tokensPerMessage
		total += c.CountText(string(types.MessageRoleSystem))
		total += c.CountText(systemPrompt)
	}
	return total
}

// TruncateToTokens truncates text to at most maxTokens.
func (c *Counter) TruncateToTokens(text string, maxTokens int) string {
	if maxTokens <= 0 {
		return ""
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	tokens := c.encoding.Encode(text, nil, nil)
	if len(tokens) <= maxTokens {
		return text
	}
	truncated := c.encoding.Decode(tokens[:maxTokens])
	return strings.TrimSpace(truncated) + "\n...[truncated]"
}

// EncodingForModel returns the tokenizer encoding name for a model.
func (c *Counter) EncodingForModel(model string) string {
	_ = model
	return defaultEncoding
}

// CheckBudget returns an error when count exceeds budget.
func (c *Counter) CheckBudget(count, budget int) error {
	if budget <= 0 || count <= budget {
		return nil
	}
	return sharederrors.NewTokenBudgetExceededError(count, budget)
}

// EstimateRemaining returns tokens left before hitting max.
func EstimateRemaining(current, max int) int {
	remaining := max - current
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Compile-time interface check.
var _ contracts.ITokenCounter = (*Counter)(nil)
