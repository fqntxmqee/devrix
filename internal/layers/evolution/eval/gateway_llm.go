package eval

import (
	"context"
	"strings"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/shared/types"
)

// GatewayLLMClient routes Judge calls through the D3 LLM Gateway.
type GatewayLLMClient struct {
	gw           llmgateway.IGateway
	defaultModel string
	provider     string
}

// NewGatewayLLMClient creates an LLMClient backed by llmgateway.IGateway.
func NewGatewayLLMClient(gw llmgateway.IGateway, model, provider string) *GatewayLLMClient {
	return &GatewayLLMClient{
		gw:           gw,
		defaultModel: model,
		provider:     provider,
	}
}

// Chat implements LLMClient by aggregating a streaming gateway response.
func (c *GatewayLLMClient) Chat(
	ctx context.Context,
	model string,
	systemPrompt string,
	userMsg string,
	temperature float64,
	maxTokens int,
) (string, TokenCost, error) {
	if model == "" {
		model = c.defaultModel
	}
	req := &llmgateway.Request{
		Provider:     c.provider,
		Model:        model,
		SystemPrompt: systemPrompt,
		Messages:     []types.Message{{Role: "user", Content: userMsg}},
		Temperature:  temperature,
		MaxTokens:    maxTokens,
		Stream:       true,
	}

	ch, err := c.gw.Stream(ctx, req)
	if err != nil {
		return "", TokenCost{}, err
	}

	var sb strings.Builder
	cost := TokenCost{}
	for chunk := range ch {
		sb.WriteString(chunk.Content)
		if chunk.Usage.TotalTokens > 0 {
			cost = TokenCost{
				PromptTokens:     chunk.Usage.PromptTokens,
				CompletionTokens: chunk.Usage.CompletionTokens,
				TotalTokens:      chunk.Usage.TotalTokens,
			}
		}
	}
	return sb.String(), cost, nil
}
