package adapter

import (
	"context"
	"net/http"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	sharedconfig "github.com/devrix/devrix/internal/shared/config"
)

const minimaxProvider = "minimax"

// MiniMaxAdapter streams responses from the MiniMax OpenAI-compatible API.
type MiniMaxAdapter struct {
	client *OpenAIStreamClient
}

// NewMiniMaxAdapter creates a MiniMax adapter.
func NewMiniMaxAdapter(cfg sharedconfig.LLMProviderRuntimeConfig) *MiniMaxAdapter {
	return &MiniMaxAdapter{
		client: NewOpenAIStreamClient(minimaxProvider, cfg),
	}
}

// WithHTTPClient overrides HTTP client (tests).
func (a *MiniMaxAdapter) WithHTTPClient(client *http.Client) *MiniMaxAdapter {
	return &MiniMaxAdapter{
		client: a.client.WithHTTPClient(client),
	}
}

// Provider returns the provider name.
func (a *MiniMaxAdapter) Provider() string {
	return minimaxProvider
}

// Stream performs a streaming chat completion.
func (a *MiniMaxAdapter) Stream(ctx context.Context, req *llmgateway.Request) (<-chan *llmgateway.AdapterChunk, error) {
	if req != nil && !req.Stream {
		cp := *req
		cp.Stream = true
		req = &cp
	}
	return a.client.Stream(ctx, req)
}
