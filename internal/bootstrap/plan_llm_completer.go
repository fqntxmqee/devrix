package bootstrap

import (
	"context"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/turn"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// planLLMCompleter adapts turn.LLMInvoker to workmodel.LLMCompleter.
type planLLMCompleter struct {
	invoker turn.LLMInvoker
	tier    string
}

func newPlanLLMCompleter(invoker turn.LLMInvoker, defaultTier string) workmodel.LLMCompleter {
	return &planLLMCompleter{invoker: invoker, tier: defaultTier}
}

func (p *planLLMCompleter) Complete(ctx context.Context, prompt string) (string, error) {
	if p == nil || p.invoker == nil {
		return "", workmodel.ErrLLMNotConfigured
	}
	ch, err := p.invoker.InvokeStream(ctx, turn.LLMInvokeRequest{
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
