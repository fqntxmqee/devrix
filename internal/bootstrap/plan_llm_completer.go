package bootstrap

import (
	"context"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// planLLMCompleter adapts sessionorchestrator.LLMInvoker to workmodel.LLMCompleter.
type planLLMCompleter struct {
	invoker sessionorchestrator.LLMInvoker
	tier    string
}

func newPlanLLMCompleter(invoker sessionorchestrator.LLMInvoker, defaultTier string) workmodel.LLMCompleter {
	return &planLLMCompleter{invoker: invoker, tier: defaultTier}
}

func (p *planLLMCompleter) Complete(ctx context.Context, prompt string) (string, error) {
	if p == nil || p.invoker == nil {
		return "", workmodel.ErrLLMNotConfigured
	}
	ch, err := p.invoker.InvokeStream(ctx, sessionorchestrator.LLMInvokeRequest{
		Tier: p.tier,
		Messages: []types.Message{{
			Role:    types.MessageRoleUser,
			Content: prompt,
		}},
	})
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for chunk := range ch {
		if chunk.Content != "" {
			b.WriteString(chunk.Content)
		}
	}
	return b.String(), nil
}
