package metrics

import "testing"

// Covers: D5-S24-A02-T04 — d2_query_loop_legacy_invocations_total
// is registered in the observability registry and exposed on
// /metrics scraping.
func TestLegacyD2Metrics_QueryLoopInvocations_registered_in_registry(t *testing.T) {
	registry := NewRegistry(nil, nil)
	m := RegisterLegacyD2Metrics(registry)

	m.QueryLoopInvocations.Inc()
	m.QueryLoopInvocations.Inc()
	m.QueryLoopInvocations.Inc()

	if c, ok := registry.GetCounter("d2_query_loop_legacy_invocations_total", nil); !ok {
		t.Fatal("d2_query_loop_legacy_invocations_total not found in registry")
	} else if c.Value() != 3 {
		t.Errorf("counter value = %d, want 3", c.Value())
	}
}

// Covers: D5-S24-A02-T04
// Scenario: RegisterLegacyD2Metrics is idempotent across multiple
// calls — the second invocation must not panic on duplicate
// registration because init paths may wire observability more than
// once. The counter returned from the second call still observes
// subsequent Inc() calls (the second call may return a fresh counter
// instance rather than the one in the registry, but the producer
// (query.Loop) is single-wired so this is acceptable).
func TestLegacyD2Metrics_RegisterLegacyD2Metrics_idempotent(t *testing.T) {
	registry := NewRegistry(nil, nil)
	_ = RegisterLegacyD2Metrics(registry)
	// A second call must not panic on duplicate registration.
	_ = RegisterLegacyD2Metrics(registry)
	if _, ok := registry.GetCounter("d2_query_loop_legacy_invocations_total", nil); !ok {
		t.Fatal("counter should still be registered after duplicate call")
	}
}
