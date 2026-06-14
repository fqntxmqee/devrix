package breaker

import (
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	sharedconfig "github.com/devrix/devrix/internal/shared/config"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// Clock returns the current time (injectable for tests).
type Clock func() time.Time

// CircuitBreaker implements provider-scoped circuit breaking.
type CircuitBreaker struct {
	cfg      sharedconfig.LLMCircuitBreakerConfig
	circuits map[string]*circuitRecord
	mu       sync.Mutex
	now      Clock
	// observer is called outside the breaker's lock when state transitions occur.
	// DSAFT: D3-S3-A01-F02 (OnStateTransitionEmit, v1.1). Nil-safe.
	observer llmgateway.BreakerStateObserver
}

// New creates a circuit breaker with the given configuration.
func New(cfg sharedconfig.LLMCircuitBreakerConfig) *CircuitBreaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 5
	}
	if cfg.SuccessThreshold <= 0 {
		cfg.SuccessThreshold = 2
	}
	if cfg.OpenDuration <= 0 {
		cfg.OpenDuration = 30 * time.Second
	}
	if cfg.HalfOpenMaxProbes <= 0 {
		cfg.HalfOpenMaxProbes = 1
	}
	if cfg.Scope == "" {
		cfg.Scope = "provider"
	}
	return &CircuitBreaker{
		cfg:      cfg,
		circuits: make(map[string]*circuitRecord),
		now:      time.Now,
	}
}

// WithClock overrides the time source (tests).
func (b *CircuitBreaker) WithClock(clock Clock) *CircuitBreaker {
	if clock != nil {
		b.now = clock
	}
	return b
}

// WithObserver attaches a state-change observer.
//
// DSAFT: D3-S3-A01-F02 (OnStateTransitionEmit, v1.1).
// Returns the receiver for chaining. Passing nil clears the observer.
func (b *CircuitBreaker) WithObserver(observer llmgateway.BreakerStateObserver) *CircuitBreaker {
	b.observer = observer
	return b
}

// Allow reports whether a request may proceed for the circuit key.
func (b *CircuitBreaker) Allow(circuitKey string) (bool, error) {
	b.mu.Lock()
	rec := b.circuit(circuitKey)
	from := rec.state
	allowed, err := b.allowLocked(rec)
	b.mu.Unlock()

	// DSAFT: D3-S3-A01-F02 (OnStateTransitionEmit, v1.1).
	// Observer called outside lock to avoid re-entering the breaker mutex.
	NotifyStateChange(b.observer, circuitKey, from, rec.state)
	return allowed, err
}

func (b *CircuitBreaker) allowLocked(rec *circuitRecord) (bool, error) {
	switch rec.state {
	case llmgateway.CircuitClosed:
		return true, nil
	case llmgateway.CircuitOpen:
		if b.now().Sub(rec.openedAt) >= b.cfg.OpenDuration {
			rec.state = llmgateway.CircuitHalfOpen
			rec.halfOpenSuccesses = 0
			rec.halfOpenInFlight = 0
		} else {
			return false, sharederrors.NewCircuitOpenError(rec.providerKey)
		}
		fallthrough
	case llmgateway.CircuitHalfOpen:
		if rec.halfOpenInFlight >= b.cfg.HalfOpenMaxProbes {
			return false, sharederrors.NewCircuitOpenError(rec.providerKey)
		}
		rec.halfOpenInFlight++
		return true, nil
	default:
		return true, nil
	}
}

// RecordSuccess records a successful call.
func (b *CircuitBreaker) RecordSuccess(circuitKey string) {
	b.mu.Lock()
	rec := b.circuit(circuitKey)
	from := rec.state
	b.recordSuccessLocked(rec)
	b.mu.Unlock()

	// DSAFT: D3-S3-A01-F02 (OnStateTransitionEmit, v1.1).
	NotifyStateChange(b.observer, circuitKey, from, rec.state)
}

func (b *CircuitBreaker) recordSuccessLocked(rec *circuitRecord) {
	switch rec.state {
	case llmgateway.CircuitClosed:
		rec.failureCount = 0
	case llmgateway.CircuitHalfOpen:
		rec.halfOpenSuccesses++
		if rec.halfOpenSuccesses >= b.cfg.SuccessThreshold {
			rec.state = llmgateway.CircuitClosed
			rec.failureCount = 0
			rec.halfOpenSuccesses = 0
			rec.halfOpenInFlight = 0
		}
	}
	b.finalize(rec)
}

// RecordFailure records a failed call.
func (b *CircuitBreaker) RecordFailure(circuitKey string) {
	b.mu.Lock()
	rec := b.circuit(circuitKey)
	from := rec.state
	b.recordFailureLocked(rec)
	b.mu.Unlock()

	// DSAFT: D3-S3-A01-F02 (OnStateTransitionEmit, v1.1).
	NotifyStateChange(b.observer, circuitKey, from, rec.state)
}

func (b *CircuitBreaker) recordFailureLocked(rec *circuitRecord) {
	switch rec.state {
	case llmgateway.CircuitClosed:
		rec.failureCount++
		if rec.failureCount >= b.cfg.FailureThreshold {
			b.open(rec)
		}
	case llmgateway.CircuitHalfOpen:
		b.open(rec)
	}
	b.finalize(rec)
}

// State returns the current circuit state.
func (b *CircuitBreaker) State(circuitKey string) llmgateway.CircuitState {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.circuit(circuitKey).state
}

func (b *CircuitBreaker) circuit(key string) *circuitRecord {
	rec, ok := b.circuits[key]
	if !ok {
		rec = newCircuitRecord(key)
		b.circuits[key] = rec
	}
	return rec
}

func (b *CircuitBreaker) open(rec *circuitRecord) {
	rec.state = llmgateway.CircuitOpen
	rec.openedAt = b.now()
	rec.failureCount = b.cfg.FailureThreshold
	rec.halfOpenSuccesses = 0
	rec.halfOpenInFlight = 0
}

func (b *CircuitBreaker) finalize(rec *circuitRecord) {
	if rec.state == llmgateway.CircuitHalfOpen && rec.halfOpenInFlight > 0 {
		rec.halfOpenInFlight--
	}
}

var _ llmgateway.ICircuitBreaker = (*CircuitBreaker)(nil)
