package interfaces

import "testing"

// TestNewConvergenceBudget — D7-S18-A11-T02 sub-case 1.
func TestNewConvergenceBudget(t *testing.T) {
	b := NewConvergenceBudget(FallbackPessimistic)
	if b.Policy != FallbackPessimistic {
		t.Errorf("Policy = %v, want FallbackPessimistic", b.Policy)
	}
	if b.MaxDepth != 0 || b.MaxSteps != 0 || b.MaxTokens != 0 {
		t.Errorf("default fields not zero: %+v", b)
	}
}

// TestConvergenceBudget_WithBuilders — With* builders are immutable and
// only touch the named field.
func TestConvergenceBudget_WithBuilders(t *testing.T) {
	b0 := NewConvergenceBudget(FallbackPessimistic)

	b1 := b0.WithMaxDepth(5)
	if b0.MaxDepth != 0 {
		t.Errorf("base MaxDepth mutated: %d", b0.MaxDepth)
	}
	if b1.MaxDepth != 5 {
		t.Errorf("b1.MaxDepth = %d, want 5", b1.MaxDepth)
	}

	b2 := b1.WithMaxSteps(20)
	if b2.MaxSteps != 20 {
		t.Errorf("b2.MaxSteps = %d, want 20", b2.MaxSteps)
	}
	if b1.MaxSteps != 0 {
		t.Errorf("b1.MaxSteps mutated: %d", b1.MaxSteps)
	}

	b3 := b2.WithMaxTokens(10000)
	if b3.MaxTokens != 10000 {
		t.Errorf("b3.MaxTokens = %d, want 10000", b3.MaxTokens)
	}

	b4 := b3.WithPolicy(FallbackRuleBased)
	if b3.Policy != FallbackPessimistic {
		t.Errorf("b3.Policy mutated: %v", b3.Policy)
	}
	if b4.Policy != FallbackRuleBased {
		t.Errorf("b4.Policy = %v, want FallbackRuleBased", b4.Policy)
	}
}

// TestConvergenceBudget_Validate — D7-S18-A11-T02 sub-case 2. Negative
// limits and an unrecognized policy must reject.
func TestConvergenceBudget_Validate(t *testing.T) {
	cases := []struct {
		name string
		b    ConvergenceBudget
		ok   bool
	}{
		{"zero default", NewConvergenceBudget(FallbackPessimistic), true},
		{"all positive", ConvergenceBudget{MaxDepth: 5, MaxSteps: 20, MaxTokens: 1000, Policy: FallbackRuleBased}, true},
		{"abort is valid legacy", ConvergenceBudget{Policy: FallbackAbort}, true},
		{"negative depth", ConvergenceBudget{MaxDepth: -1, Policy: FallbackPessimistic}, false},
		{"negative steps", ConvergenceBudget{MaxSteps: -1, Policy: FallbackPessimistic}, false},
		{"negative tokens", ConvergenceBudget{MaxTokens: -1, Policy: FallbackPessimistic}, false},
		{"invalid policy", ConvergenceBudget{Policy: FallbackPolicy(99)}, false},
	}
	for _, c := range cases {
		err := c.b.Validate()
		if c.ok && err != nil {
			t.Errorf("%s: Validate returned %v, want nil", c.name, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%s: Validate returned nil, want error", c.name)
		}
	}
}

// TestRemainingBelowReserve — D7-S18-A11-T02 sub-case 3. The trigger fires
// when tokensBudget - tokensUsed <= reserve, AND only when tokensBudget
// > 0 (no budget → no trigger).
func TestRemainingBelowReserve(t *testing.T) {
	cases := []struct {
		name     string
		used     int
		budget   int
		reserve  int
		expected bool
	}{
		{"no budget", 100, 0, 100, false},
		{"plenty left", 100, 1000, 100, false},
		{"at reserve", 900, 1000, 100, true},
		{"below reserve", 950, 1000, 100, true},
		{"over budget", 1500, 1000, 100, true},
		{"zero reserve under budget", 999, 1000, 0, false},
		{"zero reserve over budget", 1500, 1000, 0, true},
	}
	for _, c := range cases {
		got := RemainingBelowReserve(c.used, c.budget, c.reserve)
		if got != c.expected {
			t.Errorf("%s: RemainingBelowReserve(%d,%d,%d) = %v, want %v",
				c.name, c.used, c.budget, c.reserve, got, c.expected)
		}
	}
}

// TestConvergenceBudget_ToFields — small helper, mainly guards the field
// order so span attributes don't drift.
func TestConvergenceBudget_ToFields(t *testing.T) {
	b := ConvergenceBudget{MaxDepth: 3, MaxSteps: 12, MaxTokens: 4096, Policy: FallbackRuleBased}
	d, s, t2 := b.ToFields()
	if d != 3 || s != 12 || t2 != 4096 {
		t.Errorf("ToFields = (%d,%d,%d), want (3,12,4096)", d, s, t2)
	}
}
