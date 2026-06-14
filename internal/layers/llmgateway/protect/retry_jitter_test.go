package protect

import (
	"math"
	"math/rand"
	"testing"
	"time"

	sharedconfig "github.com/devrix/devrix/internal/shared/config"
)

// T: D3-S4-A01-T01
func TestExecutor_should_apply_full_jitter_within_cap(t *testing.T) {
	exec := NewExecutor().WithRNG(rand.New(rand.NewSource(42)))
	cfg := sharedconfig.LLMRetryConfig{
		InitialDelay: time.Second,
		MaxDelay:     10 * time.Second,
		Backoff:      2.0,
	}

	for attempt := 0; attempt < 3; attempt++ {
		delay := exec.backoffDelay(cfg, attempt)
		cap := time.Duration(float64(cfg.InitialDelay) * math.Pow(cfg.Backoff, float64(attempt)))
		if cap > cfg.MaxDelay {
			cap = cfg.MaxDelay
		}
		if delay < 0 || delay >= cap {
			t.Fatalf("attempt %d: delay=%s cap=%s", attempt, delay, cap)
		}
	}
}

// T: D3-S4-A01-T01
func TestExecutor_should_be_deterministic_with_fixed_rng(t *testing.T) {
	cfg := sharedconfig.LLMRetryConfig{InitialDelay: time.Second, MaxDelay: 5 * time.Second, Backoff: 2.0}
	a := NewExecutor().WithRNG(rand.New(rand.NewSource(7)))
	b := NewExecutor().WithRNG(rand.New(rand.NewSource(7)))

	if a.backoffDelay(cfg, 2) != b.backoffDelay(cfg, 2) {
		t.Fatal("expected deterministic jitter with same seed")
	}
}
