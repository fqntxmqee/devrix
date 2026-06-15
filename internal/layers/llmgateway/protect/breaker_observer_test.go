package protect_test

import (
	"sync"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/llmgateway/protect"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/metrics"
	sharedconfig "github.com/devrix/devrix/internal/shared/config"
)

// newObsForTest returns a fresh Observability + Bridge for unit tests.
func newObsForTest(t *testing.T) (*observability.Observability, *observability.Bridge) {
	t.Helper()
	obs, err := observability.New(observability.DefaultConfig())
	if err != nil {
		t.Fatalf("observability.New: %v", err)
	}
	return obs, observability.NewBridge(obs)
}

// DSAFT: D3-S3-A01-T13 (Breaker State Metric Emit + Counter, v1.1 F1 + F2).
// Verifies that the observer updates llm_breaker_state gauge and
// llm_breaker_transitions_total counter on every state transition.
func TestBreakerObserver_emits_state_gauge_and_transition_counter(t *testing.T) {
	_, obs := newObsForTest(t)
	registry := obs.Meter().Registry()

	clock := &fakeClock{t: time.Unix(0, 0)}
	cfg := sharedconfig.LLMCircuitBreakerConfig{
		FailureThreshold:  1,
		SuccessThreshold:  1,
		OpenDuration:      30 * time.Second,
		HalfOpenMaxProbes: 1,
		Scope:             "provider",
	}
	cb := protect.New(cfg).
		WithClock(clock.Now).
		WithObserver(protect.NewBreakerObserver(obs, protect.PublishBreakerStateDefault{}))

	const provider = "deepseek"

	// Initial state: Closed. Allow returns true without firing observer (no transition).
	cb.Allow(provider)
	// Drive Closed→Open with one failure.
	cb.RecordFailure(provider)

	// DSAFT: D3-S3-A01-F01 — llm_breaker_state gauge should be (closed=0, open=2, half-open=0).
	if got := gaugeValueByName(registry, "devrix_llm_breaker_state", metrics.LabelMap{"provider": provider, "state": "open"}); got != 2.0 {
		t.Errorf("gauge(state=open) = %v, want 2.0", got)
	}
	if got := gaugeValueByName(registry, "devrix_llm_breaker_state", metrics.LabelMap{"provider": provider, "state": "closed"}); got != 0.0 {
		t.Errorf("gauge(state=closed) = %v, want 0.0", got)
	}
	if got := gaugeValueByName(registry, "devrix_llm_breaker_state", metrics.LabelMap{"provider": provider, "state": "half-open"}); got != 0.0 {
		t.Errorf("gauge(state=half-open) = %v, want 0.0", got)
	}

	// DSAFT: D3-S3-A01-F02 — devrix_llm_breaker_transitions_total{closed->open} = 1.
	if got := counterValueByName(registry, "devrix_llm_breaker_transitions_total", metrics.LabelMap{
		"provider": provider, "from": "closed", "to": "open",
	}); got != 1 {
		t.Errorf("counter(closed→open) = %d, want 1", got)
	}

	// Drive Open→HalfOpen (advance clock past OpenDuration, then Allow).
	clock.Advance(31 * time.Second)
	cb.Allow(provider)
	if got := gaugeValueByName(registry, "devrix_llm_breaker_state", metrics.LabelMap{"provider": provider, "state": "half-open"}); got != 2.0 {
		t.Errorf("after Allow: gauge(state=half-open) = %v, want 2.0", got)
	}
	if got := counterValueByName(registry, "devrix_llm_breaker_transitions_total", metrics.LabelMap{
		"provider": provider, "from": "open", "to": "half-open",
	}); got != 1 {
		t.Errorf("counter(open→half-open) = %d, want 1", got)
	}

	// Drive HalfOpen→Closed (via RecordSuccess + SuccessThreshold=1).
	cb.RecordSuccess(provider)
	if got := gaugeValueByName(registry, "devrix_llm_breaker_state", metrics.LabelMap{"provider": provider, "state": "closed"}); got != 2.0 {
		t.Errorf("after RecordSuccess: gauge(state=closed) = %v, want 2.0", got)
	}
	if got := counterValueByName(registry, "devrix_llm_breaker_transitions_total", metrics.LabelMap{
		"provider": provider, "from": "half-open", "to": "closed",
	}); got != 1 {
		t.Errorf("counter(half-open→closed) = %d, want 1", got)
	}
}

// DSAFT: D3-S3-A01-T13 (Breaker nil observer = no panic, v1.1 F1+F2 OFF behavior).
// Without WithObserver, breaker must not panic and gauge/counter must not be emitted.
func TestBreakerObserver_nil_observer_is_noop(t *testing.T) {
	_, obs := newObsForTest(t)
	registry := obs.Meter().Registry()

	cfg := sharedconfig.LLMCircuitBreakerConfig{
		FailureThreshold:  1,
		SuccessThreshold:  1,
		OpenDuration:      30 * time.Second,
		HalfOpenMaxProbes: 1,
		Scope:             "provider",
	}
	cb := protect.New(cfg) // no observer

	cb.RecordFailure("deepseek")
	cb.RecordFailure("deepseek")
	cb.Allow("deepseek")

	if c, ok := registry.GetCounter("devrix_llm_breaker_transitions_total", metrics.LabelMap{
		"provider": "deepseek", "from": "closed", "to": "open",
	}); ok {
		t.Errorf("expected counter not registered without observer, got value=%d", c.Value())
	}
}

// fakePublisher captures EngineEvent calls for the F3 assertion.
type fakePublisher struct {
	mu    sync.Mutex
	transitions []fakeBreakerTransition
}

type fakeBreakerTransition struct {
	provider string
	state    llmgateway.CircuitState
}

func (p *fakePublisher) PublishBreakerState(provider string, state llmgateway.CircuitState) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.transitions = append(p.transitions, fakeBreakerTransition{provider: provider, state: state})
}

func (p *fakePublisher) last() (fakeBreakerTransition, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.transitions) == 0 {
		return fakeBreakerTransition{}, false
	}
	return p.transitions[len(p.transitions)-1], true
}

// DSAFT: D3-S3-A01-T14 (Breaker State → EngineEvent publish, v1.1 F3, D6-A 决议).
// Verifies that on every state transition the observer calls the publisher
// with the new state. Combined with the no-op default, this is the seam
// that D7 will later wire into a real EngineEvent bus.
func TestBreakerObserver_publishes_engine_event_on_transition(t *testing.T) {
	_, obs := newObsForTest(t)
	pub := &fakePublisher{}

	clock := &fakeClock{t: time.Unix(0, 0)}
	cfg := sharedconfig.LLMCircuitBreakerConfig{
		FailureThreshold:  1,
		SuccessThreshold:  1,
		OpenDuration:      30 * time.Second,
		HalfOpenMaxProbes: 1,
		Scope:             "provider",
	}
	cb := protect.New(cfg).
		WithClock(clock.Now).
		WithObserver(protect.NewBreakerObserver(obs, pub))

	const provider = "deepseek"

	// Drive Closed→Open.
	cb.RecordFailure(provider)
	if got, ok := pub.last(); !ok || got.provider != provider || got.state != llmgateway.CircuitOpen {
		t.Errorf("after Closed→Open: last = %+v ok=%v, want provider=%s state=open", got, ok, provider)
	}

	// Drive Open→HalfOpen.
	clock.Advance(31 * time.Second)
	cb.Allow(provider)
	if got, ok := pub.last(); !ok || got.provider != provider || got.state != llmgateway.CircuitHalfOpen {
		t.Errorf("after Open→HalfOpen: last = %+v ok=%v, want provider=%s state=half-open", got, ok, provider)
	}

	// Drive HalfOpen→Closed.
	cb.RecordSuccess(provider)
	if got, ok := pub.last(); !ok || got.provider != provider || got.state != llmgateway.CircuitClosed {
		t.Errorf("after HalfOpen→Closed: last = %+v ok=%v, want provider=%s state=closed", got, ok, provider)
	}
}

// helpers ————————————————————————————————————————————————————————————

// gaugeValueByName searches the registry for a gauge matching name+labels and
// returns its current value. Returns -1 if not found.
func gaugeValueByName(reg *metrics.Registry, name string, labels metrics.LabelMap) float64 {
	for _, m := range reg.List() {
		if m.Type() != metrics.MetricTypeGauge || m.Name() != name {
			continue
		}
		if labelMapEqual(m.Labels(), labels) {
			if g, ok := m.(metrics.Gauge); ok {
				return g.Value()
			}
		}
	}
	return -1
}

// counterValueByName returns the counter value matching name+labels. Returns -1 if missing.
func counterValueByName(reg *metrics.Registry, name string, labels metrics.LabelMap) int64 {
	c, ok := reg.GetCounter(name, labels)
	if !ok {
		return -1
	}
	return c.Value()
}

func labelMapEqual(a, b metrics.LabelMap) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// Compile-time check that we still import llmgateway (used implicitly via tests).
var _ = llmgateway.CircuitClosed