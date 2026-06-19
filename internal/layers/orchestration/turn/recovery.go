package turn

import (
	"context"
	"strings"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

// IsContextLengthError reports prompt-too-long failures (TD-QL-01, D7 path).
func IsContextLengthError(err error) bool {
	if err == nil {
		return false
	}
	if code := errors.ErrorCode(err); code == errors.CodeContextExceeded {
		return true
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "context_length_exceeded") ||
		strings.Contains(lower, "prompt too long") ||
		strings.Contains(lower, "maximum context length") ||
		strings.Contains(lower, " 413")
}

// IsOverloadOr5xx reports overload / 5xx / rate-limit errors suitable for
// gateway-level fallback retry (TD-QL-03 — handled in llmgateway.Stream).
func IsOverloadOr5xx(err error) bool {
	if err == nil || IsContextLengthError(err) {
		return false
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "500") || strings.Contains(lower, "502") ||
		strings.Contains(lower, "503") || strings.Contains(lower, "504") {
		return true
	}
	if strings.Contains(lower, "overloaded") || strings.Contains(lower, " 529") {
		return true
	}
	if strings.Contains(lower, " 429") || strings.Contains(lower, "rate limit") || strings.Contains(lower, "too many requests") {
		return true
	}
	if strings.Contains(lower, "deadline exceeded") || strings.Contains(lower, "timeout") {
		return true
	}
	return false
}

// compressMessagesForRecovery applies a one-shot D7-side compress when LLM
// rejects the prompt for length. Uses the same runCompress degradation ladder.
func (o *DefaultOrchestrator) compressMessagesForRecovery(ctx context.Context, req TurnRequest, messages []types.Message) []types.Message {
	if o == nil || len(messages) == 0 {
		return messages
	}
	result := o.runCompress(ctx, req, &CompressHint{
		MessagesToSummarize: messages,
		TargetTokenBudget:   maxTruncatedMessages * 256,
	})
	if result.Summary == "" {
		return messages
	}
	return []types.Message{{
		SessionID: req.SessionID,
		Role:      types.MessageRoleSystem,
		Content:   result.Summary,
	}}
}

func (o *DefaultOrchestrator) invokeStreamWithRecovery(
	ctx context.Context,
	req TurnRequest,
	invokeReq LLMInvokeRequest,
) (<-chan llmgateway.Chunk, error) {
	ch, err := o.llm.InvokeStream(ctx, invokeReq)
	if err == nil || !IsContextLengthError(err) {
		return ch, err
	}
	compressed := o.compressMessagesForRecovery(ctx, req, invokeReq.Messages)
	if len(compressed) == len(invokeReq.Messages) {
		return nil, err
	}
	invokeReq.Messages = compressed
	return o.llm.InvokeStream(ctx, invokeReq)
}
