//go:build integration

package integration

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/llmgateway/adapter"
	sharedconfig "github.com/devrix/devrix/internal/shared/config"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
	llmresponses "github.com/devrix/devrix/tests/fixtures/llm_responses"
)

func moduleFixturePath(t *testing.T, rel string) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return filepath.Join(wd, "tests", "fixtures", "llm_responses", rel)
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatal("go.mod not found while resolving fixture path")
		}
		wd = parent
	}
}

func deepseekAdapterWithFixture(t *testing.T, fixture string) *adapter.DeepSeekAdapter {
	t.Helper()
	t.Setenv("DEEPSEEK_TEST_KEY", "test-key")
	cfg := sharedconfig.LLMProviderRuntimeConfig{
		BaseURL:      "http://fixture.local/v1",
		APIKeyEnv:    "DEEPSEEK_TEST_KEY",
		DefaultModel: "deepseek-v4-flash",
	}
	client := &http.Client{
		Transport: &llmresponses.ReplayTransport{
			FixturePath: moduleFixturePath(t, fixture),
		},
	}
	return adapter.NewDeepSeekAdapter(cfg).WithHTTPClient(client)
}

func minimaxAdapterWithFixture(t *testing.T, fixture string) *adapter.MiniMaxAdapter {
	t.Helper()
	t.Setenv("MINIMAX_TEST_KEY", "test-key")
	cfg := sharedconfig.LLMProviderRuntimeConfig{
		BaseURL:      "http://fixture.local/v1",
		APIKeyEnv:    "MINIMAX_TEST_KEY",
		DefaultModel: "minimax-3",
	}
	client := &http.Client{
		Transport: &llmresponses.ReplayTransport{
			FixturePath: moduleFixturePath(t, fixture),
		},
	}
	return adapter.NewMiniMaxAdapter(cfg).WithHTTPClient(client)
}

// Covers: L5-LLM-18
func TestIntegration_DeepSeekVCR_SSEParseError(t *testing.T) {
	ad := deepseekAdapterWithFixture(t, "deepseek/truncated_frame.json")
	ch, err := ad.Stream(context.Background(), &llmgateway.Request{
		Model:   "deepseek-v4-flash",
		Stream:  true,
		Messages: []types.Message{{Role: types.MessageRoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	var parseErr error
	for chunk := range ch {
		if chunk.Error != nil {
			parseErr = chunk.Error
		}
	}
	var llmErr *sharederrors.LLMError
	if !errors.As(parseErr, &llmErr) {
		t.Fatalf("expected LLMError, got %v", parseErr)
	}
	if llmErr.Code != sharederrors.CodeLLMParseError {
		t.Fatalf("code: got %s want %s", llmErr.Code, sharederrors.CodeLLMParseError)
	}
}

// Covers: L5-LLM-17
func TestIntegration_MiniMaxVCR_RateLimit429(t *testing.T) {
	ad := minimaxAdapterWithFixture(t, "minimax/rate_limit_429.json")
	_, err := ad.Stream(context.Background(), &llmgateway.Request{
		Model:   "minimax-3",
		Stream:  true,
		Messages: []types.Message{{Role: types.MessageRoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected provider unavailable error")
	}
	var llmErr *sharederrors.LLMError
	if !errors.As(err, &llmErr) {
		t.Fatalf("expected LLMError, got %T %v", err, err)
	}
	if llmErr.Code != sharederrors.CodeLLMProviderUnavailable {
		t.Fatalf("code: got %s want %s", llmErr.Code, sharederrors.CodeLLMProviderUnavailable)
	}
}

// Covers: L5-LLM-17
func TestIntegration_MiniMaxVCR_ServerError500(t *testing.T) {
	ad := minimaxAdapterWithFixture(t, "minimax/error_500.json")
	_, err := ad.Stream(context.Background(), &llmgateway.Request{
		Model:   "minimax-3",
		Stream:  true,
		Messages: []types.Message{{Role: types.MessageRoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected provider unavailable error")
	}
	var llmErr *sharederrors.LLMError
	if !errors.As(err, &llmErr) {
		t.Fatalf("expected LLMError, got %T", err)
	}
	if llmErr.Code != sharederrors.CodeLLMProviderUnavailable {
		t.Fatalf("code: got %s", llmErr.Code)
	}
}
