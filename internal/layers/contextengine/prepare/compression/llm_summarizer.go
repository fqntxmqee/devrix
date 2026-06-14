package compression

import (
	"context"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/shared/types"
)

// LLMSummarizer adapts llmgateway.ILLMGateway for autocompact summarization.
type LLMSummarizer struct {
	LLM     llmgateway.ILLMGateway
	Timeout time.Duration
}

// Summarize calls ChatStream and collects the response text.
func (s *LLMSummarizer) Summarize(ctx context.Context, model, prompt string, maxTokens int) (string, error) {
	if s.LLM == nil {
		return "", context.Canceled
	}
	runCtx := ctx
	if s.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, s.Timeout)
		defer cancel()
	}
	ch, err := s.LLM.ChatStream(runCtx, &llmgateway.Request{
		Model:    model,
		Messages: []types.Message{{Role: types.MessageRoleUser, Content: prompt}},
		Stream:   true,
	})
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for chunk := range ch {
		if chunk.Content != "" {
			b.WriteString(chunk.Content)
		}
		if chunk.Done {
			break
		}
	}
	_ = maxTokens
	return strings.TrimSpace(b.String()), nil
}
