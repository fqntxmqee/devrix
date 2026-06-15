package metrics

import "testing"

// Covers: D5 runtime.fork_session_view_total{policy=cow|snapshot|shared}
func TestMultiAgentMetrics_ForkSessionView_should_count_per_policy(t *testing.T) {
	registry := NewRegistry(nil, nil)
	m := RegisterMultiAgentMetrics(registry)

	m.ForkSessionView.Inc("cow")
	m.ForkSessionView.Inc("cow")
	m.ForkSessionView.Inc("snapshot")

	if v := m.ForkSessionView.Value("cow"); v != 2 {
		t.Errorf("cow = %d, want 2", v)
	}
	if v := m.ForkSessionView.Value("snapshot"); v != 1 {
		t.Errorf("snapshot = %d, want 1", v)
	}
	if v := m.ForkSessionView.Value("shared"); v != 0 {
		t.Errorf("shared = %d, want 0 (never incremented)", v)
	}
}

func TestMultiAgentMetrics_ForkSessionView_nil_safe(t *testing.T) {
	var p *PolicyCounter
	p.Inc("cow") // must not panic
	if v := p.Value("cow"); v != 0 {
		t.Errorf("nil Value = %d, want 0", v)
	}
}

func TestMultiAgentMetrics_ForkSessionView_registered_in_registry(t *testing.T) {
	registry := NewRegistry(nil, nil)
	m := RegisterMultiAgentMetrics(registry)
	m.ForkSessionView.Inc("cow")
	if c, ok := registry.GetCounter("runtime.fork_session_view_total", LabelMap{"policy": "cow"}); !ok {
		t.Error("runtime.fork_session_view_total{policy=cow} not found in registry")
	} else if c.Value() != 1 {
		t.Errorf("counter value = %d, want 1", c.Value())
	}
}
