package metrics

import "sync"

// MultiAgentMetrics owns the D5 metric instruments emitted by the
// multi-agent layer. It is registered through RegisterD5MultiAgent and
// counters are wired into the local observability sink in the
// multi-agent package.
//
// DM-20260611-005.
type MultiAgentMetrics struct {
	ForkSessionView *PolicyCounter
}

// PolicyCounter exposes one Counter per policy label. Callers bump
// via Inc(policy) and read the cumulative value via Value(policy).
type PolicyCounter struct {
	mu       sync.RWMutex
	registry *Registry
	name     string
	counters map[string]Counter
}

// Inc bumps the counter for the given policy label. Allocates a new
// Counter on first use and registers it with the parent registry.
func (p *PolicyCounter) Inc(policy string) {
	if p == nil {
		return
	}
	c := p.ensureCounter(policy)
	c.Inc()
}

// Value returns the cumulative counter for the given policy, or 0.
func (p *PolicyCounter) Value(policy string) int64 {
	if p == nil {
		return 0
	}
	p.mu.RLock()
	c, ok := p.counters[policy]
	p.mu.RUnlock()
	if !ok {
		return 0
	}
	return c.Value()
}

func (p *PolicyCounter) ensureCounter(policy string) Counter {
	p.mu.RLock()
	c, ok := p.counters[policy]
	p.mu.RUnlock()
	if ok {
		return c
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if c, ok := p.counters[policy]; ok {
		return c
	}
	c = NewCounter(p.name, LabelMap{"policy": policy})
	_ = p.registry.RegisterCounter(p.name, LabelMap{"policy": policy}, c)
	if p.counters == nil {
		p.counters = make(map[string]Counter, 4)
	}
	p.counters[policy] = c
	return c
}

// RegisterD5MultiAgent creates the multi-agent D5 metrics bound to the
// given registry. The returned struct is the source of truth for the
// runtime.fork_session_view_total{policy=...} metric.
//
// Idempotent at the policy-counter level; safe to call multiple times
// from different init paths.
func RegisterD5MultiAgent(registry *Registry) *MultiAgentMetrics {
	return &MultiAgentMetrics{
		ForkSessionView: &PolicyCounter{
			registry: registry,
			name:     "runtime.fork_session_view_total",
			counters: make(map[string]Counter, 4),
		},
	}
}
