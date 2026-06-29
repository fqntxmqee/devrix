package interfaces

import (
	"errors"
	"regexp"
	"sync"
	"testing"

	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// traceIDPattern matches the auto-generated "ts_<8 hex chars>" format.
var traceIDPattern = regexp.MustCompile(`^ts_[0-9a-f]{8}$`)

// TestNewTaskSpec_HappyPath — D7-S20-A01-T01 / T02 sub-case 1.
func TestNewTaskSpec_HappyPath(t *testing.T) {
	s, err := NewTaskSpec("fix the bug")
	if err != nil {
		t.Fatalf("NewTaskSpec returned unexpected error: %v", err)
	}
	if s == nil {
		t.Fatal("NewTaskSpec returned nil spec")
	}
	if s.Goal != "fix the bug" {
		t.Errorf("Goal mismatch: got %q want %q", s.Goal, "fix the bug")
	}
	if !traceIDPattern.MatchString(s.TraceID) {
		t.Errorf("TraceID %q does not match pattern %q", s.TraceID, traceIDPattern)
	}
	if s.HardConstraints == nil {
		t.Error("HardConstraints should be a non-nil empty slice (not nil)")
	}
	if s.SoftPreferences == nil {
		t.Error("SoftPreferences should be a non-nil empty slice (not nil)")
	}
	if s.ConvergenceBudget.Policy != FallbackPessimistic {
		t.Errorf("default ConvergenceBudget.Policy = %v, want FallbackPessimistic", s.ConvergenceBudget.Policy)
	}
	if s.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

// TestNewTaskSpec_EmptyGoal — D7-S20-A01-T01 sub-case 2.
func TestNewTaskSpec_EmptyGoal(t *testing.T) {
	for _, goal := range []string{"", " ", "\t", "\n"} {
		s, err := NewTaskSpec(goal)
		if err == nil {
			t.Errorf("NewTaskSpec(%q) succeeded; want ErrTaskSpecGoalEmpty", goal)
			continue
		}
		if s != nil {
			t.Errorf("NewTaskSpec(%q) returned non-nil spec with error", goal)
		}
		if !errors.Is(err, ErrTaskSpecGoalEmpty) {
			t.Errorf("NewTaskSpec(%q) error chain missing ErrTaskSpecGoalEmpty: %v", goal, err)
		}
		// Code must be the canonical 7xxx range sentinel.
		var sErr *sharederrors.SentinelError
		if !errors.As(err, &sErr) {
			t.Errorf("NewTaskSpec(%q) error is not a *sharederrors.SentinelError: %T", goal, err)
		} else if sErr.Code != "ORCH_TASK_SPEC_GOAL_EMPTY_7100" {
			t.Errorf("NewTaskSpec(%q) code = %q, want ORCH_TASK_SPEC_GOAL_EMPTY_7100", goal, sErr.Code)
		}
	}
}

// TestNewTaskSpec_UniqueTraceIDs — D7-S20-A01-T01 sub-case 4.
func TestNewTaskSpec_UniqueTraceIDs(t *testing.T) {
	const n = 1000
	seen := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		s, err := NewTaskSpec("a")
		if err != nil {
			t.Fatalf("iteration %d: NewTaskSpec: %v", i, err)
		}
		if seen[s.TraceID] {
			t.Fatalf("iteration %d: duplicate TraceID %q", i, s.TraceID)
		}
		seen[s.TraceID] = true
	}
}

// TestTaskSpec_Validate — D7-S20-A01-T01 sub-case 3.
func TestTaskSpec_Validate(t *testing.T) {
	s, err := NewTaskSpec("ok")
	if err != nil {
		t.Fatalf("NewTaskSpec: %v", err)
	}
	if err := s.Validate(); err != nil {
		t.Errorf("Validate on freshly constructed spec returned %v, want nil", err)
	}

	// Empty Goal.
	bad := *s
	bad.Goal = ""
	if err := bad.Validate(); !errors.Is(err, ErrTaskSpecGoalEmpty) {
		t.Errorf("Validate with empty Goal returned %v, want ErrTaskSpecGoalEmpty", err)
	}

	// Empty TraceID.
	bad2 := *s
	bad2.TraceID = ""
	if err := bad2.Validate(); !errors.Is(err, ErrTaskSpecTraceIDEmpty) {
		t.Errorf("Validate with empty TraceID returned %v, want ErrTaskSpecTraceIDEmpty", err)
	}

	// Nil receiver.
	var nilSpec *TaskSpec
	if err := nilSpec.Validate(); !errors.Is(err, ErrTaskSpecGoalEmpty) {
		t.Errorf("Validate on nil spec returned %v, want ErrTaskSpecGoalEmpty", err)
	}
}

// TestTaskSpec_WithImmutability — D7-S20-A01-T02.
func TestTaskSpec_WithImmutability(t *testing.T) {
	base, err := NewTaskSpec("base")
	if err != nil {
		t.Fatalf("NewTaskSpec: %v", err)
	}
	originalConstraints := len(base.HardConstraints)
	originalTraceID := base.TraceID

	// WithConstraint must not mutate the receiver.
	derived := base.WithConstraint("scope_out", "filesystem", true)
	if len(base.HardConstraints) != originalConstraints {
		t.Errorf("base.HardConstraints length changed: %d → %d", originalConstraints, len(base.HardConstraints))
	}
	if base.TraceID != originalTraceID {
		t.Errorf("base.TraceID changed: %q → %q", originalTraceID, base.TraceID)
	}
	if len(derived.HardConstraints) != 1 {
		t.Errorf("derived.HardConstraints length = %d, want 1", len(derived.HardConstraints))
	}
	if derived.HardConstraints[0].Key != "scope_out" {
		t.Errorf("derived constraint key = %q, want scope_out", derived.HardConstraints[0].Key)
	}

	// Multiple With* should chain and produce distinct objects.
	s1 := base.WithPreference("tone", "formal", 0.8)
	s2 := s1.WithCostBudget(CostQuota{Tokens: 100})
	s3 := s2.WithConvergenceBudget(ConvergenceBudget{MaxDepth: 3, Policy: FallbackRuleBased})

	if s1 == s2 || s2 == s3 || s1 == s3 {
		t.Error("With* chain produced aliasing (two pointers equal)")
	}
	if len(base.SoftPreferences) != 0 {
		t.Errorf("base.SoftPreferences mutated: %d", len(base.SoftPreferences))
	}
	if s2.CostBudget.Tokens != 100 {
		t.Errorf("s2.CostBudget.Tokens = %d, want 100", s2.CostBudget.Tokens)
	}
	if s3.ConvergenceBudget.MaxDepth != 3 {
		t.Errorf("s3.ConvergenceBudget.MaxDepth = %d, want 3", s3.ConvergenceBudget.MaxDepth)
	}

	// Append on derived must not leak back into base.
	derived2 := derived.WithConstraint("scope_in", "network", true)
	if len(derived.HardConstraints) != 1 {
		t.Errorf("derived.HardConstraints length changed after appending derived2: %d", len(derived.HardConstraints))
	}
	if len(derived2.HardConstraints) != 2 {
		t.Errorf("derived2.HardConstraints length = %d, want 2", len(derived2.HardConstraints))
	}
}

// TestTaskSpec_ConcurrentWith — D7-S20-A01-T02 concurrency check.
// Run under -race to confirm the immutable design has no data race.
func TestTaskSpec_ConcurrentWith(t *testing.T) {
	base, err := NewTaskSpec("concurrent")
	if err != nil {
		t.Fatalf("NewTaskSpec: %v", err)
	}
	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			s := base.WithConstraint("k", "v", true).WithPreference("p", "q", 0.5)
			if len(s.HardConstraints) != 1 || len(s.SoftPreferences) != 1 {
				t.Errorf("goroutine %d: derived spec fields wrong: %+v", i, s)
			}
		}(i)
	}
	wg.Wait()
	// Base must remain untouched.
	if len(base.HardConstraints) != 0 || len(base.SoftPreferences) != 0 {
		t.Errorf("base mutated under concurrency: %+v", base)
	}
}

// TestFallbackPolicy_String — defensive: String must be stable for spans.
func TestFallbackPolicy_String(t *testing.T) {
	cases := []struct {
		p    FallbackPolicy
		want string
	}{
		{FallbackAbort, "abort"},
		{FallbackPessimistic, "pessimistic"},
		{FallbackRuleBased, "rule_based"},
		{FallbackPolicy(99), "unknown(99)"},
	}
	for _, c := range cases {
		if got := c.p.String(); got != c.want {
			t.Errorf("FallbackPolicy(%d).String() = %q, want %q", int(c.p), got, c.want)
		}
	}
}