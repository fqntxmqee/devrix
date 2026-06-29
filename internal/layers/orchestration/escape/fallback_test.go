package escape

import (
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
)

// makeReport is a small builder that returns a TaskReport pre-populated
// with sensible defaults so each test can override just the fields it
// cares about.
func makeReport(t *testing.T, kind interfaces.ResultKind) *interfaces.TaskReport {
	t.Helper()
	r, err := interfaces.NewTaskReport("ts_fallback_test")
	if err != nil {
		t.Fatalf("NewTaskReport: %v", err)
	}
	return r.WithResult(interfaces.Result{
		Kind:       kind,
		Confidence: 0.5,
		Message:    "test report",
		At:         time.Now(),
	})
}

// TestDefaultPessimisticCommitGuard_Disabled — D7-S18-A11-T03 sub-case 1.
// When the Feature Flag is off, Evaluate is a no-op pass-through.
func TestDefaultPessimisticCommitGuard_Disabled(t *testing.T) {
	g := NewDefaultPessimisticCommitGuard() // Enabled = false by default
	r := makeReport(t, interfaces.ResultKindIndeterminate)
	r = r.WithBlockage(interfaces.Blockage{
		Kind:        interfaces.BlockageInfeasible,
		Description: "should be ignored when disabled",
		Source:      "test",
	})
	ok, reason, err := g.Evaluate(nil, r, interfaces.NewConvergenceBudget(interfaces.FallbackPessimistic))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !ok {
		t.Errorf("ok = false, want true (guard disabled)")
	}
	if reason != "" {
		t.Errorf("reason = %q, want empty", reason)
	}
}

// TestDefaultPessimisticCommitGuard_Enabled_ResourceExhausted — D7-S18-A11-T03.
func TestDefaultPessimisticCommitGuard_Enabled_ResourceExhausted(t *testing.T) {
	g := NewDefaultPessimisticCommitGuard()
	g.Enabled = true
	r := makeReport(t, interfaces.ResultKindPartial)
	r, _ = r.WithResource(interfaces.Resource{
		TokensUsed:   950,
		TokensBudget: 1000,
	})
	ok, reason, err := g.Evaluate(nil, r, interfaces.NewConvergenceBudget(interfaces.FallbackPessimistic))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if ok {
		t.Errorf("ok = true, want false (resource exhausted)")
	}
	if reason != interfaces.TriggerResourceExhausted {
		t.Errorf("reason = %q, want %q", reason, interfaces.TriggerResourceExhausted)
	}
}

// TestDefaultPessimisticCommitGuard_Enabled_CircuitBreakerL1 — D7-S18-A11-T03.
func TestDefaultPessimisticCommitGuard_Enabled_CircuitBreakerL1(t *testing.T) {
	g := NewDefaultPessimisticCommitGuard()
	g.Enabled = true
	r := makeReport(t, interfaces.ResultKindPartial)
	r = r.WithBlockage(interfaces.Blockage{
		Kind:        interfaces.BlockageInfeasible,
		Description: "CB L1 fired",
		Source:      "circuit_breaker_l1",
	})
	ok, reason, err := g.Evaluate(nil, r, interfaces.NewConvergenceBudget(interfaces.FallbackPessimistic))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if ok {
		t.Error("ok = true, want false (CB L1 fired)")
	}
	if reason != interfaces.TriggerCircuitBreakerL1 {
		t.Errorf("reason = %q, want %q", reason, interfaces.TriggerCircuitBreakerL1)
	}
}

// TestDefaultPessimisticCommitGuard_Enabled_Indeterminate3x — D7-S18-A11-T03.
func TestDefaultPessimisticCommitGuard_Enabled_Indeterminate3x(t *testing.T) {
	g := NewDefaultPessimisticCommitGuard()
	g.Enabled = true
	r := makeReport(t, interfaces.ResultKindIndeterminate)
	for i := 0; i < 3; i++ {
		r, _ = r.AppendDissent(interfaces.DissentEntry{
			Source:   "scenario-X",
			Decision: "abort",
			Reason:   "stuck",
			Summary:  "sum",
		})
	}
	ok, reason, err := g.Evaluate(nil, r, interfaces.NewConvergenceBudget(interfaces.FallbackPessimistic))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if ok {
		t.Error("ok = true, want false (3x indeterminate)")
	}
	if reason != interfaces.TriggerIndeterminate3x {
		t.Errorf("reason = %q, want %q", reason, interfaces.TriggerIndeterminate3x)
	}
}

// TestDefaultPessimisticCommitGuard_Enabled_EmptyEvidence — D7-S18-A11-T03.
func TestDefaultPessimisticCommitGuard_Enabled_EmptyEvidence(t *testing.T) {
	g := NewDefaultPessimisticCommitGuard()
	g.Enabled = true
	r := makeReport(t, interfaces.ResultKindPass) // all 3 evidence fields empty
	ok, reason, err := g.Evaluate(nil, r, interfaces.NewConvergenceBudget(interfaces.FallbackPessimistic))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if ok {
		t.Error("ok = true, want false (empty evidence on Pass)")
	}
	if reason != interfaces.TriggerEmptyEvidence {
		t.Errorf("reason = %q, want %q", reason, interfaces.TriggerEmptyEvidence)
	}
}

// TestDefaultPessimisticCommitGuard_Enabled_ManualAbort — D7-S18-A11-T03.
func TestDefaultPessimisticCommitGuard_Enabled_ManualAbort(t *testing.T) {
	g := NewDefaultPessimisticCommitGuard()
	g.Enabled = true
	spec, _ := interfaces.NewTaskSpec("manual abort test")
	spec = spec.WithConvergenceBudget(interfaces.NewConvergenceBudget(interfaces.FallbackAbort))
	ok, reason, err := g.Evaluate(spec, makeReport(t, interfaces.ResultKindPartial), interfaces.NewConvergenceBudget(interfaces.FallbackAbort))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if ok {
		t.Error("ok = true, want false (manual abort)")
	}
	if reason != interfaces.TriggerManualAbort {
		t.Errorf("reason = %q, want %q", reason, interfaces.TriggerManualAbort)
	}
}

// TestDefaultPessimisticCommitGuard_Enabled_HappyPath — none of the 5
// triggers fire, the report passes through.
func TestDefaultPessimisticCommitGuard_Enabled_HappyPath(t *testing.T) {
	g := NewDefaultPessimisticCommitGuard()
	g.Enabled = true
	r := makeReport(t, interfaces.ResultKindPass)
	r, _ = r.WithResource(interfaces.Resource{TokensUsed: 100, TokensBudget: 2000})
	r = r.WithEvidence(interfaces.Evidence{TestResult: "5/5", ArtifactHash: "abc"})
	ok, reason, err := g.Evaluate(nil, r, interfaces.NewConvergenceBudget(interfaces.FallbackPessimistic))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !ok {
		t.Errorf("ok = false (reason=%q), want true", reason)
	}
}

// TestDefaultPessimisticCommitGuard_ResolveFallback_Default — D7-S18-A12-T02.
func TestDefaultPessimisticCommitGuard_ResolveFallback_Default(t *testing.T) {
	g := NewDefaultPessimisticCommitGuard()
	g.Enabled = true
	r := makeReport(t, interfaces.ResultKindIndeterminate)
	policy, rule := g.ResolveFallback(r)
	if policy != interfaces.FallbackPessimistic {
		t.Errorf("policy = %v, want FallbackPessimistic", policy)
	}
	if rule != "" {
		t.Errorf("rule = %q, want empty for non-RuleBased policy", rule)
	}
}

// TestDefaultPessimisticCommitGuard_ResolveFallback_PolicyOverride —
// the report carries a Blockage.Source == "policy_override" hint.
func TestDefaultPessimisticCommitGuard_ResolveFallback_PolicyOverride(t *testing.T) {
	g := NewDefaultPessimisticCommitGuard()
	g.Enabled = true
	r := makeReport(t, interfaces.ResultKindIndeterminate)
	r = r.WithBlockage(interfaces.Blockage{
		Kind:        interfaces.BlockageRequiredExternal,
		Description: "rule_based",
		Source:      "policy_override",
	})
	policy, rule := g.ResolveFallback(r)
	if policy != interfaces.FallbackRuleBased {
		t.Errorf("policy = %v, want FallbackRuleBased", policy)
	}
	if rule != interfaces.DefaultFallbackRule {
		t.Errorf("rule = %q, want %q", rule, interfaces.DefaultFallbackRule)
	}
}

// TestDefaultPessimisticCommitGuard_BuildMVPArtifact — D7-S18-A11-T03 end-to-end.
func TestDefaultPessimisticCommitGuard_BuildMVPArtifact(t *testing.T) {
	g := NewDefaultPessimisticCommitGuard()
	g.Enabled = true
	r := makeReport(t, interfaces.ResultKindPartial)
	r = r.WithBlockage(interfaces.Blockage{
		Kind:        interfaces.BlockageInfeasible,
		Description: "tool X not available",
		Source:      "verifier-1",
	})
	mvp := g.BuildMVPArtifact(r, interfaces.TriggerCircuitBreakerL1)
	if mvp.Output == "" {
		t.Error("Output should not be empty (report had a Message + Blockage)")
	}
	if mvp.Trigger != interfaces.TriggerCircuitBreakerL1 {
		t.Errorf("Trigger = %q, want %q", mvp.Trigger, interfaces.TriggerCircuitBreakerL1)
	}
	if len(mvp.RiskWarnings) < 2 {
		t.Errorf("RiskWarnings len = %d, want >= 2 (header + per-blockage)", len(mvp.RiskWarnings))
	}
	if mvp.ChainHash == "" {
		t.Error("ChainHash should not be empty")
	}
}

// TestDefaultPessimisticCommitGuard_BuildMVPArtifact_NilReport — defensive.
func TestDefaultPessimisticCommitGuard_BuildMVPArtifact_NilReport(t *testing.T) {
	g := NewDefaultPessimisticCommitGuard()
	mvp := g.BuildMVPArtifact(nil, "manual")
	if mvp.Output != "" {
		t.Errorf("Output should be empty for nil report, got %q", mvp.Output)
	}
	if mvp.Trigger != "manual" {
		t.Errorf("Trigger = %q, want manual", mvp.Trigger)
	}
}

// TestDefaultPessimisticCommitGuard_NilReceiver — methods must be
// nil-safe so a misconfigured bootstrap can't crash the engine.
func TestDefaultPessimisticCommitGuard_NilReceiver(t *testing.T) {
	var g *DefaultPessimisticCommitGuard
	ok, reason, err := g.Evaluate(nil, makeReport(t, interfaces.ResultKindPass), interfaces.NewConvergenceBudget(interfaces.FallbackPessimistic))
	if err != nil {
		t.Fatalf("nil.Evaluate: %v", err)
	}
	if !ok || reason != "" {
		t.Errorf("nil.Evaluate = (%v,%q), want (true,\"\")", ok, reason)
	}
	policy, rule := g.ResolveFallback(nil)
	if policy != interfaces.FallbackPessimistic || rule != "" {
		t.Errorf("nil.ResolveFallback = (%v,%q), want (FallbackPessimistic,\"\")", policy, rule)
	}
}

// TestDefaultPessimisticCommitGuard_BuildMVPArtifact_Traceback — covers
// the traceback assembly + 256-char truncation branch.
func TestDefaultPessimisticCommitGuard_BuildMVPArtifact_Traceback(t *testing.T) {
	g := NewDefaultPessimisticCommitGuard()
	r := makeReport(t, interfaces.ResultKindPartial)
	long := make([]byte, 300)
	for i := range long {
		long[i] = 'x'
	}
	r = r.WithBlockage(interfaces.Blockage{
		Kind:        interfaces.BlockageInfeasible,
		Description: "d1",
		Source:      "v1",
		Traceback:   string(long),
	})
	r = r.WithBlockage(interfaces.Blockage{
		Kind:        interfaces.BlockageMissing,
		Description: "d2",
		Source:      "v2",
		Traceback:   "short",
	})
	mvp := g.BuildMVPArtifact(r, interfaces.TriggerCircuitBreakerL1)
	if mvp.Output == "" {
		t.Error("Output should not be empty")
	}
	if mvp.ChainHash == "" {
		t.Error("ChainHash should not be empty")
	}
	if len(mvp.RiskWarnings) < 3 {
		t.Errorf("RiskWarnings len = %d, want >= 3 (header + 2 blockages)", len(mvp.RiskWarnings))
	}
}

// TestBuildChainHash_Stable — ChainHash is part of the audit contract;
// different inputs must produce different hashes, and the same input
// must produce a stable length.
func TestBuildChainHash_Stable(t *testing.T) {
	h1 := buildChainHash("traceback A", 1700000000000)
	h2 := buildChainHash("traceback A", 1700000000000)
	if h1 != h2 {
		t.Errorf("same input produced different hash: %q vs %q", h1, h2)
	}
	h3 := buildChainHash("traceback B", 1700000000000)
	if h1 == h3 {
		t.Errorf("different input produced same hash: %q", h1)
	}
	if len(h1) != 16 {
		t.Errorf("hash length = %d, want 16", len(h1))
	}
}
