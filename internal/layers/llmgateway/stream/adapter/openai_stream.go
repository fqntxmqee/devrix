package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/llmgateway/configure"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

const streamChannelBuffer = 32

// OpenAIStreamClient performs OpenAI-compatible streaming HTTP calls.
type OpenAIStreamClient struct {
	provider string
	cfg      configure.LLMProviderRuntimeConfig
	client   *http.Client
}

// NewOpenAIStreamClient creates a streaming HTTP client for a provider.
func NewOpenAIStreamClient(provider string, cfg configure.LLMProviderRuntimeConfig) *OpenAIStreamClient {
	return &OpenAIStreamClient{
		provider: provider,
		cfg:      cfg,
		client:   &http.Client{Timeout: 0},
	}
}

// WithHTTPClient overrides the HTTP client (tests).
func (c *OpenAIStreamClient) WithHTTPClient(client *http.Client) *OpenAIStreamClient {
	next := *c
	if client != nil {
		next.client = client
	}
	return &next
}

// Stream calls POST /chat/completions and parses SSE output.
func (c *OpenAIStreamClient) Stream(ctx context.Context, req *llmgateway.Request) (<-chan *llmgateway.AdapterChunk, error) {
	apiKey, ok := configure.APIKey(c.cfg)
	if !ok {
		// DM-20260628-001 (T4): wrap with APIError so sharederrors.Code()
		// surfaces APICodeAuthenticationFailed via the APICodeProvider interface.
		apiErr := llmgateway.NewAPIErrorWithCause(http.StatusUnauthorized,
			"missing api key", sharederrors.ErrLLMAuthFailed)
		return nil, sharederrors.NewLLMAuthFailedError(apiErr)
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
		// DM-20260628-001 (T4): route 401/403 through NewAPIError so the
		// sharederrors.Code() chain surfaces APICodeAuthenticationFailed.
		apiErr := llmgateway.NewAPIErrorWithCause(resp.StatusCode,
			fmt.Sprintf("status %d", resp.StatusCode), sharederrors.ErrLLMAuthFailed)
		return nil, sharederrors.NewLLMAuthFailedError(apiErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		// DM-20260628-001 (T4): replace string-concat provider error with
		// NewAPIError; the APIError is the inner cause so its APICode() flows
		// up through the sentinel Error() chain.
		apiErr := llmgateway.NewAPIErrorWithCause(resp.StatusCode,
			fmt.Sprintf("provider %s status %d: %s", c.provider, resp.StatusCode, string(bodyBytes)),
			fmt.Errorf("body: %s", string(bodyBytes)))
		slog.Warn("llm: provider HTTP error",
			"provider", c.provider,
			"status", resp.StatusCode,
			"code", apiErr.Code.String(),
			"body", string(bodyBytes))
		return nil, sharederrors.NewProviderUnavailableError(apiErr)
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
	return sharederrors.NewLLMParseError(err)
}
