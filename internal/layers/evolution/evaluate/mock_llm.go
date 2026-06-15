package evaluate

import "context"

// StaticLLMClient returns a fixed Judge response for CLI and tests.
type StaticLLMClient struct {
	Response string
	Cost     TokenCost
}

// NewStaticLLMClient creates a mock LLM with a default judge-style response.
func NewStaticLLMClient() *StaticLLMClient {
	return &StaticLLMClient{
		Response: "Reasoning: mock evaluation\nScore: 0.85\nConfidence: 0.8\n",
		Cost:     TokenCost{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}
}

// Chat implements LLMClient.
func (c *StaticLLMClient) Chat(_ context.Context, _ string, _ string, _ string, _ float64, _ int) (string, TokenCost, error) {
	return c.Response, c.Cost, nil
}
