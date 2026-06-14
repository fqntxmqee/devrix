package adapter

import (
	"context"
	"net/http"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	sharedconfig "github.com/devrix/devrix/internal/shared/config"
)

const deepseekProvider = "deepseek"

// DeepSeekAdapter streams responses from the DeepSeek OpenAI-compatible API.
type DeepSeekAdapter struct {
	client *OpenAIStreamClient
}

// NewDeepSeekAdapter creates a DeepSeek adapter.
func NewDeepSeekAdapter(cfg sharedconfig.LLMProviderRuntimeConfig) *DeepSeekAdapter {
	return &DeepSeekAdapter{
		client: NewOpenAIStreamClient(deepseekProvider, cfg),
	}
}

// WithHTTPClient overrides HTTP client (tests).
func (a *DeepSeekAdapter) WithHTTPClient(client *http.Client) *DeepSeekAdapter {
	return &DeepSeekAdapter{
		client: a.client.WithHTTPClient(client),
	}
}

// Provider returns the provider name.
func (a *DeepSeekAdapter) Provider() string {
	return deepseekProvider
}

// Protocol returns the wire protocol identifier.
//
// DSAFT: D3-S2-A01-F04 (AdapterProtocolMethod, v1.1).
// DeepSeek uses an OpenAI-compatible API.
func (a *DeepSeekAdapter) Protocol() string {
	return ProtocolOpenAICompatible
}

// Stream performs a streaming chat completion.
func (a *DeepSeekAdapter) Stream(ctx context.Context, req *llmgateway.Request) (<-chan *llmgateway.AdapterChunk, error) {
	if req != nil && !req.Stream {
		cp := *req
		cp.Stream = true
		req = &cp
	}
	return a.client.Stream(ctx, req)
}
