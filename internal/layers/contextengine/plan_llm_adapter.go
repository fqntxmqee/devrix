package contextengine

import (
	"context"
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/pev"
	"github.com/devrix/devrix/internal/shared/types"
)

type planLLMAdapter struct {
	llm ILLMGateway
}

func newPlanLLMAdapter(llm ILLMGateway) pev.LLMCompleter {
	return &planLLMAdapter{llm: llm}
}

func (a *planLLMAdapter) Complete(ctx context.Context, req pev.PlanLLMRequest) (string, error) {
	chunks, err := a.llm.ChatStream(ctx, &LLMRequest{
		Model:        req.Model,
		SystemPrompt: req.SystemPrompt,
		Messages: []types.Message{
			{Role: types.MessageRoleUser, Content: req.UserPrompt},
		},
	})
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for chunk := range chunks {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		sb.WriteString(chunk.Content)
	}
	return sb.String(), nil
}
