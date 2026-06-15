package turn

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// CompressionSummarizerDeps holds dependencies for the D2 compression.Summarizer adapter.
type CompressionSummarizerDeps struct {
	Gateway      llmgateway.IGateway
	TierResolver llmgateway.ITierResolver
	DefaultTier  string
	Timeout      time.Duration
}

// CompressionSummarizer implements shared/contracts.Summarizer (D2 compression 拆面)
// by routing through the D3 gateway. This is the DM-020拆面出口 for autocompact.
//
// DSAFT: D7-S2-A07 (InvokeLLM) — D2→D3 拆面 adapter (compression path).
type CompressionSummarizer struct {
	gateway      llmgateway.IGateway
	tierResolver llmgateway.ITierResolver
	defaultTier  string
	timeout      time.Duration
}

// NewCompressionSummarizer constructs a CompressionSummarizer adapter.
func NewCompressionSummarizer(deps CompressionSummarizerDeps) *CompressionSummarizer {
	return &CompressionSummarizer{
		gateway:      deps.Gateway,
		tierResolver: deps.TierResolver,
		defaultTier:  deps.DefaultTier,
		timeout:      deps.Timeout,
	}
}

// Compile-time assertion: CompressionSummarizer satisfies contracts.Summarizer.
var _ contracts.Summarizer = (*CompressionSummarizer)(nil)

// Summarize collects streaming D3 chunks into a single summary string.
func (s *CompressionSummarizer) Summarize(ctx context.Context, model, prompt string, maxTokens int) (string, error) {
	if s.gateway == nil {
		return "", fmt.Errorf("turn.CompressionSummarizer: gateway is nil")
	}

	resolvedModel := model
	if s.tierResolver != nil {
		resolved, err := s.tierResolver.ResolveTier(model)
		if err != nil {
			return "", fmt.Errorf("tier resolve %q: %w", model, err)
		}
		resolvedModel = resolved
	}

	runCtx := ctx
	if s.timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}

	ch, err := s.gateway.Stream(runCtx, &llmgateway.Request{
		Model: resolvedModel,
		Messages: []types.Message{
			{Role: types.MessageRoleUser, Content: prompt},
		},
		Stream: true,
	})
	if err != nil {
		return "", err
	}

	var b strings.Builder
	for {
		select {
		case <-runCtx.Done():
			return "", runCtx.Err()
		case chunk, ok := <-ch:
			if !ok {
				_ = maxTokens
				return strings.TrimSpace(b.String()), nil
			}
			if chunk.Content != "" {
				b.WriteString(chunk.Content)
			}
			if chunk.Done {
				_ = maxTokens
				return strings.TrimSpace(b.String()), nil
			}
		}
	}
}
