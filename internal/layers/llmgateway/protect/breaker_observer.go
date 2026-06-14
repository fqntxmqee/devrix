package protect

import (
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/metrics"
)

// BreakerStateMetricGauge records the current Breaker state per provider.
//
// DSAFT: D3-S3-A01-F01 (EmitBreakerStateMetric, v1.1 D1-A 决议).
// Cardinality: 2 provider × 3 state = 6 series.
// Encoding: state="closed" → gauge=0/1/2 ; we set the active state to 2 and
// others to 0 so Grafana can compute "currently open" via max(gauge{state="open"}).
type BreakerStateMetricGauge struct {
	closed *metrics.Gauge
	open   *metrics.Gauge
	half   *metrics.Gauge
}

// BreakerTransitionCounter counts state transitions.
//
// DSAFT: D3-S3-A01-F02 (OnStateTransitionEmit, v1.1).
// Cardinality: 2 provider × 3 from × 3 to = 18 series.
type BreakerTransitionCounter struct {
	transitions map[string]metrics.Counter // keyed by "<provider>|<from>|<to>"
}

// EngineEventPublisher is the minimal D3→D7 contract for breaker events.
//
// DSAFT: D3-S3-A01-F03 (ReuseEngineEvent, v1.1 D6-A 决议).
// The full D3→D7 contract is documented in
// openspec/specs/architecture/cross-domain-boundaries.md §2.4.3.
type EngineEventPublisher interface {
	PublishBreakerState(provider string, state llmgateway.CircuitState)
}

// NewBreakerObserver constructs a StateObserver that emits metrics and
// (optionally) publishes EngineEvents on every state transition.
//
// DSAFT: D3-S3-A01-F01 + F02 + F03.
// All three sinks (gauge, counter, publisher) are independently nil-safe.
func NewBreakerObserver(obs *observability.Bridge, publisher EngineEventPublisher) llmgateway.BreakerStateObserver {
	return StateObserverFunc(func(provider string, from, to llmgateway.CircuitState) {
		if obs == nil || obs.Meter() == nil {
			return
		}
		emitBreakerStateGauge(obs.Meter(), provider, to)
		emitBreakerTransitionCounter(obs.Meter(), provider, from, to)
		if publisher != nil {
			publisher.PublishBreakerState(provider, to)
		}
	})
}

// emitBreakerStateGauge updates llm_breaker_state{provider, state}.
//
// DSAFT: D3-S3-A01-F01.
// Active state → 2.0; other states → 0.0. This avoids running counters
// in the same metric family and keeps cardinality stable.
func emitBreakerStateGauge(meter *metrics.Meter, provider string, state llmgateway.CircuitState) {
	if meter == nil {
		return
	}
	for _, s := range []llmgateway.CircuitState{llmgateway.CircuitClosed, llmgateway.CircuitOpen, llmgateway.CircuitHalfOpen} {
		labels := metrics.LabelMap{"provider": provider, "state": string(s)}
		g, err := meter.Int64UpDownCounter("llm_breaker_state", metrics.WithLabels(labels))
		if err != nil || g == nil {
			continue
		}
		if s == state {
			g.Set(2.0)
		} else {
			g.Set(0.0)
		}
	}
}

// emitBreakerTransitionCounter increments llm_breaker_transitions_total.
//
// DSAFT: D3-S3-A01-F02.
// Lazy metric registration via the meter; the meter caches by name+labels.
func emitBreakerTransitionCounter(meter *metrics.Meter, provider string, from, to llmgateway.CircuitState) {
	if meter == nil {
		return
	}
	labels := metrics.LabelMap{"provider": provider, "from": string(from), "to": string(to)}
	c, err := meter.Int64Counter("llm_breaker_transitions_total", metrics.WithLabels(labels))
	if err != nil || c == nil {
		return
	}
	c.Add(1)
}

// PublishBreakerStateDefault is a no-op publisher used when no D7 bus is wired.
//
// DSAFT: D3-S3-A01-F03.
type PublishBreakerStateDefault struct{}

// PublishBreakerState implements EngineEventPublisher.
func (PublishBreakerStateDefault) PublishBreakerState(provider string, state llmgateway.CircuitState) {}

// Compile-time interface check.
var _ EngineEventPublisher = (*PublishBreakerStateDefault)(nil)