// Tests for the cross-layer contract registry.
//
// Covers: L5-0-0-02  (contract registry resolves every cross-layer interface)
// Domain: shared/contracts
// Stage: s0_unit
package contracts

import "testing"

// TestRegistry_RegistersAndResolves verifies that Register + Lookup work as a
// round-trip and that an unknown name returns nil.
func TestRegistry_RegistersAndResolves(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}
	r.Register(Contract{
		Name:   "IEngine",
		Owner:  "shared/contracts",
		Source: "engine.go",
	})
	got, ok := r.Lookup("IEngine")
	if !ok {
		t.Fatal("expected lookup to find IEngine")
	}
	if got.Owner != "shared/contracts" {
		t.Errorf("owner = %q, want shared/contracts", got.Owner)
	}
	if _, ok := r.Lookup("IDoNotExist"); ok {
		t.Error("expected IDoNotExist to be absent")
	}
}

// TestRegistry_SelfCheck_NoOrphans verifies that the default seed catalog
// passes SelfCheck (every contract is registered and resolvable).
func TestRegistry_SelfCheck_NoOrphans(t *testing.T) {
	r := NewRegistry()
	r.RegisterAll(DefaultCatalog())
	problems := r.SelfCheck()
	if len(problems) != 0 {
		t.Fatalf("default catalog should be self-consistent, problems: %v", problems)
	}
}

// TestRegistry_SelfCheck_DetectsMissingContract verifies that registering a
// consumer without a matching contract surfaces a problem.
func TestRegistry_SelfCheck_DetectsMissingContract(t *testing.T) {
	r := NewRegistry()
	r.Register(Contract{Name: "IEngine", Owner: "shared/contracts"})
	r.Register(Contract{Name: "IUnknown", Owner: "shared/contracts"})
	r.Register(Contract{Name: "IToolRunner", Owner: "shared/contracts"})
	r.RegisterConsumer(Consumer{Contract: "IMissing", Layer: "D2"})
	problems := r.SelfCheck()
	if len(problems) == 0 {
		t.Fatal("expected at least one self-check problem")
	}
	foundMissing := false
	for _, p := range problems {
		if p == "consumer D2 references unregistered contract IMissing" {
			foundMissing = true
		}
	}
	if !foundMissing {
		t.Errorf("expected missing-contract problem, got: %v", problems)
	}
}

// TestRegistry_DefaultCatalog_HasCoreContracts guards against silent drops
// from the seed catalog (e.g. when someone refactors engine.go).
func TestRegistry_DefaultCatalog_HasCoreContracts(t *testing.T) {
	cat := DefaultCatalog()
	want := map[string]bool{
		"IEngine":           false,
		"EngineEvent":       false,
		"ITokenCounter":     false,
		"ExecutionFlowHub":  false,
	}
	for _, c := range cat {
		if _, ok := want[c.Name]; ok {
			want[c.Name] = true
		}
	}
	for name, present := range want {
		if !present {
			t.Errorf("DefaultCatalog missing core contract %q", name)
		}
	}
}
