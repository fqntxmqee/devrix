package metrics

// LegacyD2Metrics owns the observability (D5) metric instruments that
// track use of deprecated D2 paths from the DM-20260617-001
// "D2 QueryLoop Legacy Decommission" change.
//
// Currently registered:
//
//	d2_query_loop_legacy_invocations_total — bumped once per
//	  `internal/layers/contextengine/query.Loop.Run` invocation, which
//	  is the canonical D2-S10 entry point that loopFirst=false still
//	  reaches. The counter must remain 0 in production when the
//	  default loopFirst=true is honored (see D7-S2-A06).
type LegacyD2Metrics struct {
	QueryLoopInvocations Counter
}

// RegisterLegacyD2Metrics creates the D2 legacy-path metrics bound to
// the given registry. Safe to call multiple times; the meter-level
// counter is idempotent through Registry.RegisterCounter semantics.
//
// DM-20260617-001.
func RegisterLegacyD2Metrics(registry *Registry) *LegacyD2Metrics {
	return &LegacyD2Metrics{
		QueryLoopInvocations: mustRegisterCounter(registry,
			"d2_query_loop_legacy_invocations_total", nil),
	}
}

// mustRegisterCounter registers a counter and returns the existing
// instance on duplicate registration. Errors are silently dropped
// because registry collisions in init paths must not crash the
// process; callers can re-lookup the counter via registry.GetCounter
// if they need to recover the value.
func mustRegisterCounter(registry *Registry, name string, labels LabelMap) Counter {
	c := NewCounter(name, labels)
	_ = registry.RegisterCounter(name, labels, c)
	return c
}
