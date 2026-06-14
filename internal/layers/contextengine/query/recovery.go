package query

import (
	"context"
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

// IsContextLengthError returns true if the error indicates the LLM rejected
// the request because the prompt was too long (HTTP 413 / OpenAI
// `context_length_exceeded` / Anthropic-equivalent / our own
// CTX_EXCEEDED_4001 sentinel).
//
// TD-QL-01: QueryLoop uses this signal to trigger a one-shot
// messages-only compression pass and retry the LLM call once.
//
// We deliberately keep the surface narrow: only the most common
// context-length signals are matched. Rate limits, 5xx overload, and
// gateway errors are not in scope here — TD-QL-03 owns the fallback
// model retry path.
func IsContextLengthError(err error) bool {
	if err == nil {
		return false
	}
	// Our own sentinel: `CTX_EXCEEDED_4001`.
	if code := errors.ErrorCode(err); code == errors.CodeContextExceeded {
		return true
	}
	// String-match the well-known LLM vendor signals.
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "context_length_exceeded") ||
		strings.Contains(lower, "prompt too long") ||
		strings.Contains(lower, "maximum context length") ||
		strings.Contains(lower, " 413")
}

// runWithContextLengthRecovery wraps a single LLM.Call with a 413
// recovery pass: if the first call fails with a context-length error
// and a CompressFn is configured, compress once and retry once.
//
// Returns the chunks channel from whichever call succeeded. If both
// attempts fail, the error from the *second* call is returned (it
// usually has the most up-to-date detail).
//
// `messagesRef` is a pointer to the messages slice in the caller; the
// recovery pass swaps it with the compressed version so the LLM
// request is retried against the smaller payload. The caller is
// expected to read from `*messagesRef` after this returns.
func runWithContextLengthRecovery(
	ctx context.Context,
	llm LLMCaller,
	req LLMRequest,
	compress CompressFunc,
	messagesRef *[]types.Message,
) (<-chan LLMChunk, error) {
	chunks, err := llm.Call(ctx, req)
	if err == nil {
		return chunks, nil
	}
	if !IsContextLengthError(err) || compress == nil {
		return nil, err
	}
	compressed, cerr := compress(ctx, *messagesRef)
	if cerr != nil {
		// Compression itself failed; surface the original LLM error
		// (it's the more useful signal for the upstream caller).
		return nil, err
	}
	compressed = conversation.RepairToolMessageChain(compressed)
	*messagesRef = compressed
	req.Messages = compressed
	return llm.Call(ctx, req)
}

// IsOverloadOr5xx (TD-QL-03) returns true when the error is an overload
// (HTTP 529 / "overloaded") or a 5xx-class failure that the
// gateway-retry path should handle by switching to a fallback model.
//
// We intentionally exclude 4xx (except 429) and 413:
//   - 413 → 413 recovery (TD-QL-01)
//   - 4xx other than 429 → bad request, no point retrying on a
//     different model.
//   - 429 → rate limit; treat as fallback-worthy so a fallback model
//     might have spare quota.
func IsOverloadOr5xx(err error) bool { return isOverloadOr5xx(err) }

func isOverloadOr5xx(err error) bool {
	if err == nil {
		return false
	}
	// 413 is owned by the 413 recovery path.
	if IsContextLengthError(err) {
		return false
	}
	lower := strings.ToLower(err.Error())
	// 5xx
	if strings.Contains(lower, "500") || strings.Contains(lower, "502") ||
		strings.Contains(lower, "503") || strings.Contains(lower, "504") {
		return true
	}
	// 529 / overload
	if strings.Contains(lower, "overloaded") || strings.Contains(lower, " 529") {
		return true
	}
	// 429 / rate limit
	if strings.Contains(lower, " 429") || strings.Contains(lower, "rate limit") || strings.Contains(lower, "too many requests") {
		return true
	}
	// timeout
	if strings.Contains(lower, "deadline exceeded") || strings.Contains(lower, "timeout") {
		return true
	}
	return false
}
