package protect

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/llmgateway/configure"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// StreamFunc starts a streaming call for a model.
type StreamFunc func(ctx context.Context, model string) (<-chan *llmgateway.AdapterChunk, error)

// Executor runs retry with exponential backoff and optional fallback model.
type Executor struct {
	rng *rand.Rand
}

// NewExecutor creates a retry executor.
func NewExecutor() *Executor {
	return &Executor{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// WithRNG injects a random source (tests).
func (e *Executor) WithRNG(rng *rand.Rand) *Executor {
	next := *e
	if rng != nil {
		next.rng = rng
	}
	return &next
}

// Stream executes call with retries on primary then fallback model.
func (e *Executor) Stream(
	ctx context.Context,
	call StreamFunc,
	primary string,
	fallback string,
	cfg configure.LLMRetryConfig,
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
				delay := e.backoffDelay(cfg, attempt)
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
		lastErr = sharederrors.NewProviderUnavailableError(
			errors.New("retry loop completed without recording an error: all attempts returned nil"))
	}
	return nil, lastErr
}

func (e *Executor) backoffDelay(cfg configure.LLMRetryConfig, attempt int) time.Duration {
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

	cap := float64(initial) * math.Pow(backoff, float64(attempt))
	if cap > float64(maxDelay) {
		cap = float64(maxDelay)
	}
	if cap <= 0 {
		return 0
	}
	jitter := time.Duration(e.rng.Int63n(int64(cap)))
	if jitter > maxDelay {
		return maxDelay
	}
	return jitter
}

