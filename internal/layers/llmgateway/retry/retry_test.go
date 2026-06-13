package retry_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/llmgateway/retry"
	sharedconfig "github.com/devrix/devrix/internal/shared/config"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// T: D3-S4-A01-T01
func TestExecutor_should_retry_retryable_errors(t *testing.T) {
	exec := retry.NewExecutor()
	cfg := sharedconfig.LLMRetryConfig{
		MaxAttempts:  3,
		InitialDelay: time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Backoff:      2.0,
	}
	attempts := 0
	ch, err := exec.Stream(context.Background(), func(ctx context.Context, model string) (<-chan *llmgateway.AdapterChunk, error) {
		attempts++
		if attempts < 3 {
			return nil, sharederrors.NewLLMTimeoutError(errors.New("timeout"))
		}
		out := make(chan *llmgateway.AdapterChunk, 1)
		out <- &llmgateway.AdapterChunk{Parsed: &llmgateway.Chunk{Content: "ok", Done: true}}
		close(out)
		return out, nil
	}, "deepseek-v4-pro", "", cfg)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	<-ch
	if attempts != 3 {
		t.Errorf("attempts: %d", attempts)
	}
}

// T: D3-S4-A01-T01
func TestExecutor_should_not_retry_auth_errors(t *testing.T) {
	exec := retry.NewExecutor()
	cfg := sharedconfig.LLMRetryConfig{MaxAttempts: 3, InitialDelay: time.Millisecond}
	attempts := 0
	_, err := exec.Stream(context.Background(), func(ctx context.Context, model string) (<-chan *llmgateway.AdapterChunk, error) {
		attempts++
		return nil, sharederrors.NewLLMAuthFailedError(nil)
	}, "minimax-3", "", cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 1 {
		t.Errorf("attempts: %d", attempts)
	}
}

// T: D3-S4-A01-T02, D3-S4-A01-T03
func TestExecutor_should_fallback_to_secondary_model(t *testing.T) {
	exec := retry.NewExecutor()
	cfg := sharedconfig.LLMRetryConfig{MaxAttempts: 1, InitialDelay: time.Millisecond}
	var models []string
	ch, err := exec.Stream(context.Background(), func(ctx context.Context, model string) (<-chan *llmgateway.AdapterChunk, error) {
		models = append(models, model)
		if model == "deepseek-v4-pro" {
			return nil, sharederrors.NewProviderUnavailableError(errors.New("down"))
		}
		out := make(chan *llmgateway.AdapterChunk, 1)
		out <- &llmgateway.AdapterChunk{Parsed: &llmgateway.Chunk{Content: "fb", Done: true}}
		close(out)
		return out, nil
	}, "deepseek-v4-pro", "deepseek-v4-flash", cfg)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	<-ch
	if len(models) != 2 || models[1] != "deepseek-v4-flash" {
		t.Errorf("models: %v", models)
	}
}
