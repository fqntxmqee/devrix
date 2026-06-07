package retry

import (
	"context"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	sharedconfig "github.com/devrix/devrix/internal/shared/config"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// StreamFunc starts a streaming call for a model.
type StreamFunc func(ctx context.Context, model string) (<-chan *llmgateway.AdapterChunk, error)

// Executor runs retry with exponential backoff and optional fallback model.
type Executor struct{}

// NewExecutor creates a retry executor.
func NewExecutor() *Executor {
	return &Executor{}
}

// Stream executes call with retries on primary then fallback model.
func (e *Executor) Stream(
	ctx context.Context,
	call StreamFunc,
	primary string,
	fallback string,
	cfg sharedconfig.LLMRetryConfig,
) (<-chan *llmgateway.AdapterChunk, error) {
	models := []string{primary}
	if fallback != "" && fallback != primary {
		models = append(models, fallback)
	}

	maxAttempts := cfg.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	var lastErr error
	for mi, model := range models {
		for attempt := 0; attempt < maxAttempts; attempt++ {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if attempt > 0 || mi > 0 && attempt == 0 && lastErr != nil {
				delay := backoffDelay(cfg, attempt)
				if mi > 0 && attempt == 0 {
					delay = cfg.InitialDelay
					if delay <= 0 {
						delay = time.Second
					}
				}
				if delay > 0 {
					timer := time.NewTimer(delay)
					select {
					case <-ctx.Done():
						timer.Stop()
						return nil, ctx.Err()
					case <-timer.C:
					}
				}
			}

			ch, err := call(ctx, model)
			if err == nil {
				return ch, nil
			}
			lastErr = err
			if !sharederrors.IsRetryable(err) {
				break
			}
		}
	}
	if lastErr == nil {
		lastErr = sharederrors.NewProviderUnavailableError(nil)
	}
	return nil, lastErr
}

func backoffDelay(cfg sharedconfig.LLMRetryConfig, attempt int) time.Duration {
	initial := cfg.InitialDelay
	if initial <= 0 {
		initial = time.Second
	}
	maxDelay := cfg.MaxDelay
	if maxDelay <= 0 {
		maxDelay = 10 * time.Second
	}
	backoff := cfg.Backoff
	if backoff <= 0 {
		backoff = 2.0
	}
	delay := float64(initial) * pow(backoff, float64(attempt))
	if delay > float64(maxDelay) {
		delay = float64(maxDelay)
	}
	return time.Duration(delay)
}

func pow(base, exp float64) float64 {
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	return result
}
