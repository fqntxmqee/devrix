package contracts

import "github.com/devrix/devrix/internal/shared/types"

// ITokenCounter counts tokens for context budgeting (L2 Context Engine + L3 LLM Gateway).
type ITokenCounter interface {
	CountText(text string) int
	CountMessages(messages []types.Message) int
	CountWithSystemPrompt(systemPrompt string, messages []types.Message) int
	TruncateToTokens(text string, maxTokens int) string
	EncodingForModel(model string) string
}
