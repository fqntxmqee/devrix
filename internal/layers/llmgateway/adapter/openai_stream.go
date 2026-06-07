package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	llmconfig "github.com/devrix/devrix/internal/layers/llmgateway/config"
	sharedconfig "github.com/devrix/devrix/internal/shared/config"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

const streamChannelBuffer = 32

// OpenAIStreamClient performs OpenAI-compatible streaming HTTP calls.
type OpenAIStreamClient struct {
	provider string
	cfg      sharedconfig.LLMProviderRuntimeConfig
	client   *http.Client
}

// NewOpenAIStreamClient creates a streaming HTTP client for a provider.
func NewOpenAIStreamClient(provider string, cfg sharedconfig.LLMProviderRuntimeConfig) *OpenAIStreamClient {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &OpenAIStreamClient{
		provider: provider,
		cfg:      cfg,
		client:   &http.Client{Timeout: timeout},
	}
}

// WithHTTPClient overrides the HTTP client (tests).
func (c *OpenAIStreamClient) WithHTTPClient(client *http.Client) *OpenAIStreamClient {
	if client != nil {
		c.client = client
	}
	return c
}

// Stream calls POST /chat/completions and parses SSE output.
func (c *OpenAIStreamClient) Stream(ctx context.Context, req *llmgateway.Request) (<-chan *llmgateway.AdapterChunk, error) {
	apiKey, ok := llmconfig.APIKey(c.cfg)
	if !ok {
		return nil, sharederrors.NewLLMAuthFailedError(sharederrors.ErrLLMAuthFailed)
	}

	body, err := buildOpenAIChatRequest(req)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(c.cfg.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	for k, v := range c.cfg.Headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, sharederrors.NewLLMTimeoutError(err)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		_ = resp.Body.Close()
		return nil, sharederrors.NewLLMAuthFailedError(fmt.Errorf("status %d", resp.StatusCode))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		return nil, sharederrors.NewProviderUnavailableError(
			fmt.Errorf("provider %s status %d: %s", c.provider, resp.StatusCode, string(bodyBytes)),
		)
	}

	out := make(chan *llmgateway.AdapterChunk, streamChannelBuffer)
	go func() {
		defer close(out)
		defer resp.Body.Close()

		err := streamOpenAISSE(resp.Body, func(chunk *llmgateway.Chunk) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case out <- &llmgateway.AdapterChunk{Parsed: chunk}:
				return nil
			}
		})
		if err != nil && ctx.Err() == nil {
			select {
			case <-ctx.Done():
			case out <- &llmgateway.AdapterChunk{Error: mapStreamError(err)}:
			}
		}
	}()

	return out, nil
}

func mapStreamError(err error) error {
	if err == nil {
		return nil
	}
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		return sharederrors.NewLLMParseError(err)
	}
	return sharederrors.NewLLMParseError(err)
}
