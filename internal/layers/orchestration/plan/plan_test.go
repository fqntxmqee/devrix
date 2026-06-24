package plan

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// -----------------------------------------------------------------------------
// Test fixtures
// -----------------------------------------------------------------------------

// fakeObservation is a minimal ObservationLookup for ReverseLookup tests.
type fakeObservation struct {
	id   string
	data string
}

func (f fakeObservation) GetID() string { return f.id }

// validPlanSteps returns a single-step slice sufficient for Validate() to pass
// (Steps is required to be non-empty).
func validPlanSteps() []Step {
	return []Step{{ID: "step_1", Directive: "do thing", ToolName: "noop"}}
}

// validPlan constructs a Plan that passes Validate() with default opts. The
// caller can further mutate via With* methods.
func validPlan(t *testing.T) *Plan {
	t.Helper()
	planID := "plan_test"
	sessionID := "sess_test"
	obs := []string{"obs_1", "obs_2"}
	p := NewPlan(planID, sessionID, CommitmentPlan, obs, validPlanSteps(), 0.85).
		WithFailureCriteria([]FailureCriterion{
			{Field: "exit_code", Op: "eq", Value: 0},
		}).
		WithBlastRadius(BlastRadius{FileCount: 1, APICallCount: 1, TokenCost: 100, PersistScope: PersistSession})
	if err := p.Validate(); err != nil {
		t.Fatalf("validPlan: unexpected validation error: %v", err)
	}
	return &p
}

// -----------------------------------------------------------------------------
// D7-S8-A22-T01: Plan 4 类 enum 互斥
// -----------------------------------------------------------------------------

// TestPlanKind_4Types_AreDistinct guards the Phase 2 PR-B1 invariant that all
// 4 named PlanKind values occupy distinct uint8 slots so D5 dashboards can
// filter on wire-format strings without collision.
func TestPlanKind_4Types_AreDistinct(t *testing.T) {
	kinds := []PlanKind{CommitmentPlan, ProtocolPlan, ScenarioPlan, ExplorationPlan}
	seen := make(map[PlanKind]bool, len(kinds))
	wireSeen := make(map[string]PlanKind, len(kinds))
	for _, k := range kinds {
		if !k.IsKnown() {
			t.Errorf("PlanKind=%d should be known", uint8(k))
		}
		s := k.String()
		if dup := seen[k]; dup {
			t.Errorf("PlanKind=%d seen twice — enum slot collision", uint8(k))
		}
		if prev, dup := wireSeen[s]; dup {
			t.Errorf("PlanKind=%d and %d collide on String=%q", uint8(prev), uint8(k), s)
		}
		seen[k] = true
		wireSeen[s] = k
	}
	if len(seen) != 4 {
		t.Errorf("expected 4 distinct PlanKind values, got %d", len(seen))
	}
}

// TestPlanKind_String_SnakeCase verifies the wire-format convention used by D5
// dashboards for filtering. Changing any of these strings is a breaking change.
func TestPlanKind_String_SnakeCase(t *testing.T) {
	cases := []struct {
		k    PlanKind
		want string
	}{
		{CommitmentPlan, "commitment_plan"},
		{ProtocolPlan, "protocol_plan"},
		{ScenarioPlan, "scenario_plan"},
		{ExplorationPlan, "exploration_plan"},
	}
	for _, c := range cases {
		if got := c.k.String(); got != c.want {
			t.Errorf("PlanKind(%d).String()=%q, want %q", uint8(c.k), got, c.want)
		}
	}
}

// TestPlanKind_KindUnset_DefaultsFromEmpty covers the zero-value path so a
// freshly constructed Plan (without WithKind) trips Validate() with
// ErrPlanKindUnset rather than silently defaulting.
func TestPlanKind_KindUnset_DefaultsFromEmpty(t *testing.T) {
	var k PlanKind
	if k.IsKnown() {
		t.Error("zero PlanKind must NOT be known")
	}
	if !strings.Contains(k.String(), "unknown_plan_kind") {
		t.Errorf("zero PlanKind String()=%q, want unknown_plan_kind(...)", k.String())
	}
}

// TestPlanKind_MarshalJSON_KnownValues guards wire-format stability.
func TestPlanKind_MarshalJSON_KnownValues(t *testing.T) {
	for k, want := range map[PlanKind]string{
		CommitmentPlan:  `"commitment_plan"`,
		ProtocolPlan:    `"protocol_plan"`,
		ScenarioPlan:    `"scenario_plan"`,
		ExplorationPlan: `"exploration_plan"`,
	} {
		got, err := json.Marshal(k)
		if err != nil {
			t.Errorf("Marshal(%v) unexpected error: %v", k, err)
			continue
		}
		if string(got) != want {
			t.Errorf("Marshal(%v)=%s, want %s", k, got, want)
		}
	}
}

// TestPlanKind_MarshalJSON_KindUnset_Omits checks the omitempty behavior so
// pre-classification Plans do not leak a sentinel string to D5 dashboards.
func TestPlanKind_MarshalJSON_KindUnset_Omits(t *testing.T) {
	got, err := json.Marshal(KindUnset)
	if err != nil {
		t.Fatalf("Marshal(KindUnset) unexpected error: %v", err)
	}
	if string(got) != "null" {
		t.Errorf("Marshal(KindUnset)=%s, want null", got)
	}
}

// TestPlanKind_UnmarshalJSON_RoundTrip exercises Marshal→Unmarshal for every
// named kind to guarantee wire-format stability.
func TestPlanKind_UnmarshalJSON_RoundTrip(t *testing.T) {
	for _, k := range []PlanKind{CommitmentPlan, ProtocolPlan, ScenarioPlan, ExplorationPlan} {
		data, err := json.Marshal(k)
		if err != nil {
			t.Fatalf("Marshal(%v) err: %v", k, err)
		}
		var got PlanKind
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal(%v) err: %v", k, err)
		}
		if got != k {
			t.Errorf("round-trip mismatch: in=%v out=%v", k, got)
		}
	}
}

// TestPlanKind_UnmarshalJSON_UnknownFailsFast matches the PR-RF C3 contract —
// silent coercion to KindUnset would mask malformed wire payloads.
func TestPlanKind_UnmarshalJSON_UnknownFailsFast(t *testing.T) {
	var k PlanKind
	err := json.Unmarshal([]byte(`"future_plan_kind"`), &k)
	if err == nil {
		t.Fatal("expected error on unknown PlanKind, got nil")
	}
	if !strings.Contains(err.Error(), "unknown PlanKind") {
		t.Errorf("error message should mention unknown PlanKind, got: %v", err)
	}
}

// TestParsePlanKind covers the CLI parser inverse-of-String contract.
func TestParsePlanKind(t *testing.T) {
	cases := []struct {
		in      string
		want    PlanKind
		wantErr bool
	}{
		{"commitment_plan", CommitmentPlan, false},
		{"PROTOCOL_PLAN", ProtocolPlan, false}, // case-insensitive
		{"  scenario_plan  ", ScenarioPlan, false},
		{"", KindUnset, true},
		{"future_plan_kind", KindUnset, true},
	}
	for _, c := range cases {
		got, err := ParsePlanKind(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParsePlanKind(%q): expected error, got nil", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParsePlanKind(%q): unexpected error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParsePlanKind(%q)=%v, want %v", c.in, got, c.want)
		}
	}
}

// -----------------------------------------------------------------------------
// D7-S8-A22-T02: SourceObservationIDs 必填 + 血缘
// -----------------------------------------------------------------------------

// TestPlan_SourceObservationIDs_Required asserts PP lineage: an empty slice
// fails Validate() with ErrPlanSourceObservationIDsRequired (Phase 4 Verify
// cannot reverse-lookup without these IDs).
func TestPlan_SourceObservationIDs_Required(t *testing.T) {
	p := NewPlan("plan_x", "sess_x", CommitmentPlan, nil, validPlanSteps(), 0.9).
		WithFailureCriteria([]FailureCriterion{
			{Field: "exit_code", Op: "eq", Value: 0},
		})
	if err := p.Validate(); err == nil {
		t.Fatal("expected validation error for empty SourceObservationIDs, got nil")
	} else if !errors.Is(err, ErrPlanSourceObservationIDsRequired) {
		t.Errorf("expected ErrPlanSourceObservationIDsRequired, got %v", err)
	} else if code := sharederrors.ErrorCode(err); code != "PLAN_LINEAGE_8002" {
		t.Errorf("expected error code PLAN_LINEAGE_8002, got %q", code)
	}
}

// TestPlan_NewPlan_CopiesObservationIDs guards the immutability contract —
// callers mutating the input slice must not affect the Plan.
func TestPlan_NewPlan_CopiesObservationIDs(t *testing.T) {
	original := []string{"obs_a", "obs_b"}
	p := NewPlan("plan_x", "sess_x", CommitmentPlan, original, validPlanSteps(), 0.9)
	original[0] = "MUTATED"
	if got := p.SourceObservationIDs[0]; got != "obs_a" {
		t.Errorf("Plan reflected external mutation: got %q, want obs_a", got)
	}
}

// TestPlan_SourceObservationIDs_ReverseLookup_Exact covers the Phase 4 Verify
// reverse-traceability primitive: given a list of Observations, only the
// matching IDs come back.
func TestPlan_SourceObservationIDs_ReverseLookup_Exact(t *testing.T) {
	p := validPlan(t)
	observations := []ObservationLookup{
		fakeObservation{id: "obs_unrelated", data: "noise"},
		fakeObservation{id: "obs_1", data: "match-1"},
		fakeObservation{id: "obs_3", data: "noise-3"},
		fakeObservation{id: "obs_2", data: "match-2"},
	}
	got := p.ReverseLookupObservations(observations)
	if len(got) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(got))
	}
	ids := map[string]string{}
	for _, o := range got {
		ids[o.GetID()] = o.(fakeObservation).data
	}
	if ids["obs_1"] != "match-1" || ids["obs_2"] != "match-2" {
		t.Errorf("unexpected match data: %+v", ids)
	}
}

// TestPlan_SourceObservationIDs_ReverseLookup_DuplicateIDs covers the
// defensive copy in NewPlan — duplicate IDs in SourceObservationIDs do not
// produce duplicate reverse-lookup hits.
func TestPlan_SourceObservationIDs_ReverseLookup_DuplicateIDs(t *testing.T) {
	p := NewPlan("plan_x", "sess_x", CommitmentPlan,
		[]string{"obs_1", "obs_1", "obs_2"}, validPlanSteps(), 0.9)
	observations := []ObservationLookup{
		fakeObservation{id: "obs_1", data: "only-one"},
		fakeObservation{id: "obs_2", data: "second"},
	}
	got := p.ReverseLookupObservations(observations)
	if len(got) != 2 {
		t.Errorf("expected 2 unique results even with duplicate source IDs, got %d", len(got))
	}
}

// TestPlan_SourceObservationIDs_ReverseLookup_Empty guards the degenerate
// input path so Verify does not crash when observation store is empty.
func TestPlan_SourceObservationIDs_ReverseLookup_Empty(t *testing.T) {
	p := validPlan(t)
	if got := p.ReverseLookupObservations(nil); got != nil {
		t.Errorf("ReverseLookupObservations(nil)=%v, want nil", got)
	}
	if got := p.ReverseLookupObservations([]ObservationLookup{}); got != nil {
		t.Errorf("ReverseLookupObservations([])=%v, want nil", got)
	}
}

// TestPlan_WithBlastRadius_Immutable guarantees With* never mutates the
// receiver — Phase 2 PR-A1 WithKind contract.
func TestPlan_WithBlastRadius_Immutable(t *testing.T) {
	p1 := validPlan(t)
	p2 := p1.WithBlastRadius(BlastRadius{FileCount: 99, APICallCount: 1, TokenCost: 1, PersistScope: PersistPermanent})
	if p1.BlastRadius.FileCount == 99 {
		t.Error("WithBlastRadius mutated original Plan")
	}
	if p2.BlastRadius.FileCount != 99 || p2.BlastRadius.PersistScope != PersistPermanent {
		t.Errorf("WithBlastRadius copy broken: %+v", p2.BlastRadius)
	}
}

// -----------------------------------------------------------------------------
// D7-S8-A22-T03: Kind 匹配规则 (4 Rules + Tie-break + DefaultPlanner.Plan)
// -----------------------------------------------------------------------------

// TestMatchKind_4Rules asserts the 4-rule deterministic classifier from
// doc 43 §4.5.
func TestMatchKind_4Rules(t *testing.T) {
	cases := []struct {
		name           string
		quantizedKind  string
		stepCount      int
		anomaliesCount int
		want           PlanKind
	}{
		// Rule 1: intent_orchestrate → Exploration
		{"intent_orchestrate", "intent_orchestrate", 5, 0, ExplorationPlan},
		// Rule 1: high anomaly → Exploration
		{"anomaly_threshold", "intent_fast", 5, 3, ExplorationPlan},
		// Rule 2: single step → Commitment
		{"single_step_default", "intent_fast", 1, 0, CommitmentPlan},
		// Rule 3: intent_command → Protocol
		{"intent_command", "intent_command", 5, 0, ProtocolPlan},
		// Rule 3: 2-3 steps → Protocol
		{"two_steps", "intent_fast", 2, 0, ProtocolPlan},
		{"three_steps", "intent_fast", 3, 0, ProtocolPlan},
		// Rule 4: exploratory multi-step → Scenario
		{"exploratory_multi", "intent_fast", 5, 0, ScenarioPlan},
		// Rule 1 still wins over Rule 2 (uncertainty first)
		{"uncertainty_beats_commitment", "intent_orchestrate", 1, 0, ExplorationPlan},
		{"anomaly_beats_commitment", "intent_fast", 1, 3, ExplorationPlan},
		// Rule 1 still wins over Rule 3
		{"uncertainty_beats_command", "intent_orchestrate", 2, 0, ExplorationPlan},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := MatchKind(c.quantizedKind, c.stepCount, c.anomaliesCount)
			if got != c.want {
				t.Errorf("MatchKind(%q, %d, %d)=%v, want %v",
					c.quantizedKind, c.stepCount, c.anomaliesCount, got, c.want)
			}
		})
	}
}

// TestDefaultPlanner_Plan_EmptyObservationIDs_FailsFast enforces the PR-B1
// invariant that Plan cannot be constructed without lineage.
func TestDefaultPlanner_Plan_EmptyObservationIDs_FailsFast(t *testing.T) {
	planner := NewDefaultPlanner()
	_, err := planner.Plan(PlanInput{
		SessionID:       "sess_1",
		ObservationIDs:  nil,
		QuantizedKind:   "intent_fast",
		AnomaliesCount:  0,
		Steps:           validPlanSteps(),
		FailureCriteria: []FailureCriterion{{Field: "exit_code", Op: "eq", Value: 0}},
	})
	if err == nil {
		t.Fatal("expected error for empty ObservationIDs, got nil")
	}
	if !errors.Is(err, ErrPlanSourceObservationIDsRequired) {
		t.Errorf("expected ErrPlanSourceObservationIDsRequired, got %v", err)
	}
}

// TestDefaultPlanner_Plan_CommitmentFromSingleStep covers Rule 2 in the full
// Plan-input contract.
func TestDefaultPlanner_Plan_CommitmentFromSingleStep(t *testing.T) {
	planner := NewDefaultPlanner()
	steps := []Step{{ID: "step_1", Directive: "deploy", ToolName: "deploy"}}
	out, err := planner.Plan(PlanInput{
		SessionID:       "sess_1",
		ObservationIDs:  []string{"obs_1"},
		QuantizedKind:   "intent_fast",
		AnomaliesCount:  0,
		Steps:           steps,
		FailureCriteria: []FailureCriterion{{Field: "exit_code", Op: "eq", Value: 0}},
		BlastRadius:     BlastRadius{PersistScope: PersistSession},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Kind != CommitmentPlan {
		t.Errorf("Kind=%v, want CommitmentPlan", out.Kind)
	}
	if len(out.Steps) != 1 {
		t.Errorf("Steps len=%d, want 1", len(out.Steps))
	}
	if out.SessionID != "sess_1" {
		t.Errorf("SessionID=%q, want sess_1", out.SessionID)
	}
}

// TestDefaultPlanner_Plan_ExplorationFromAnomalies covers Rule 1.
func TestDefaultPlanner_Plan_ExplorationFromAnomalies(t *testing.T) {
	planner := NewDefaultPlanner()
	out, err := planner.Plan(PlanInput{
		SessionID:       "sess_1",
		ObservationIDs:  []string{"obs_1"},
		QuantizedKind:   "intent_fast",
		AnomaliesCount:  5,
		Steps:           []Step{{ID: "step_1", Directive: "explore"}},
		FailureCriteria: []FailureCriterion{{Field: "exit_code", Op: "eq", Value: 0}},
		BlastRadius:     BlastRadius{PersistScope: PersistTransient},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Kind != ExplorationPlan {
		t.Errorf("Kind=%v, want ExplorationPlan", out.Kind)
	}
	if out.AnomaliesCount != 5 {
		t.Errorf("AnomaliesCount=%d, want 5", out.AnomaliesCount)
	}
}

// TestDefaultPlanner_Plan_StrengthMatchesFormula exercises the strengthFloor
// formula: 0.7 base − 0.1·anomalies + min(observations·0.02, 0.2).
// obs=0 is invalid input (PP-1 lineage) and is covered by TestDefaultPlanner_Plan_EmptyObservationIDs_FailsFast.
func TestDefaultPlanner_Plan_StrengthMatchesFormula(t *testing.T) {
	planner := NewDefaultPlanner()
	cases := []struct {
		anomalies int
		obs       int
		wantFloor float64
	}{
		{0, 1, 0.72}, // 0.7 + 0.02
		{0, 10, 0.9}, // 0.7 + min(0.2, 0.2)
		{0, 50, 0.9}, // capped at 0.9
		{3, 1, 0.42}, // 0.4 + 0.02
		{2, 5, 0.6},  // 0.5 + 0.1
	}
	for _, c := range cases {
		obsIDs := make([]string, c.obs)
		for i := range obsIDs {
			obsIDs[i] = "obs"
		}
		out, err := planner.Plan(PlanInput{
			SessionID:       "sess_1",
			ObservationIDs:  obsIDs,
			QuantizedKind:   "intent_fast",
			AnomaliesCount:  c.anomalies,
			Steps:           validPlanSteps(),
			FailureCriteria: []FailureCriterion{{Field: "exit_code", Op: "eq", Value: 0}},
			BlastRadius:     BlastRadius{PersistScope: PersistSession},
		})
		if err != nil {
			t.Fatalf("anomalies=%d obs=%d unexpected error: %v", c.anomalies, c.obs, err)
		}
		// Compare with tolerance — float math can drift slightly.
		if got := out.Strength; got < c.wantFloor-1e-9 || got > c.wantFloor+1e-9 {
			t.Errorf("anomalies=%d obs=%d Strength=%v, want %v",
				c.anomalies, c.obs, got, c.wantFloor)
		}
	}
}

// TestDefaultPlanner_Plan_ValidationFailurePropagates ensures a Plan that
// fails Validate() (e.g. missing FailureCriteria) is surfaced rather than
// silently dispatched.
func TestDefaultPlanner_Plan_ValidationFailurePropagates(t *testing.T) {
	planner := NewDefaultPlanner()
	_, err := planner.Plan(PlanInput{
		SessionID:      "sess_1",
		ObservationIDs: []string{"obs_1"},
		QuantizedKind:  "intent_fast",
		AnomaliesCount: 0,
		Steps:          validPlanSteps(),
		BlastRadius:    BlastRadius{PersistScope: PersistSession},
		// FailureCriteria empty → PP-2 violation
	})
	if err == nil {
		t.Fatal("expected PP-2 validation error, got nil")
	}
	if !errors.Is(err, ErrPlanFailureCriteriaEmpty) {
		t.Errorf("expected ErrPlanFailureCriteriaEmpty, got %v", err)
	}
	if code := sharederrors.ErrorCode(err); code != "PLAN_PP2_EMPTY_8020" {
		t.Errorf("expected error code PLAN_PP2_EMPTY_8020, got %q", code)
	}
}

// TestDefaultPlanner_Plan_BlastRadiusPropagated asserts the BlastRadius
// input flows through to the constructed Plan unchanged.
func TestDefaultPlanner_Plan_BlastRadiusPropagated(t *testing.T) {
	planner := NewDefaultPlanner()
	br := BlastRadius{FileCount: 10, APICallCount: 5, TokenCost: 1000, PersistScope: PersistSession}
	out, err := planner.Plan(PlanInput{
		SessionID:       "sess_1",
		ObservationIDs:  []string{"obs_1"},
		QuantizedKind:   "intent_fast",
		AnomaliesCount:  0,
		Steps:           validPlanSteps(),
		FailureCriteria: []FailureCriterion{{Field: "exit_code", Op: "eq", Value: 0}},
		BlastRadius:     br,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.BlastRadius != br {
		t.Errorf("BlastRadius=%+v, want %+v", out.BlastRadius, br)
	}
}

// TestStrengthFloor_Unit is a direct white-box check on the formula edge
// cases (capped at 1, floored at 0). Tolerates float drift near cap (0.7 +
// 0.2 = 0.8999... in IEEE 754).
func TestStrengthFloor_Unit(t *testing.T) {
	if got := strengthFloor(0, 0); got != 0.7 {
		t.Errorf("strengthFloor(0,0)=%v, want 0.7", got)
	}
	if got := strengthFloor(100, 0); got != 0 {
		t.Errorf("strengthFloor(100,0)=%v, want 0", got)
	}
	if got := strengthFloor(0, 100); got > 1.0 {
		t.Errorf("strengthFloor(0,100)=%v, want ≤ 1.0", got)
	}
	if got := strengthFloor(0, 100); got < 0.89 {
		t.Errorf("strengthFloor(0,100)=%v, want ≥ 0.89 (formula cap ≈ 0.9 with float drift)", got)
	}
}

// -----------------------------------------------------------------------------
// Additional coverage: With* immutability, BlastRadius.Zero, Validate failure
// paths, MarshalJSON, helper errors.
// -----------------------------------------------------------------------------

// TestPlan_KindAssignment guards that direct field assignment on a Plan
// copy is value-safe (Plan is a value type, so the receiver copy is independent).
func TestPlan_KindAssignment(t *testing.T) {
	p1 := validPlan(t)
	p2 := *p1
	p2.Kind = ExplorationPlan
	if p1.Kind == ExplorationPlan {
		t.Error("direct Kind assignment leaked through Plan value copy")
	}
	if p2.Kind != ExplorationPlan {
		t.Errorf("copy.Kind = %v, want %v", p2.Kind, ExplorationPlan)
	}
}

// TestPlan_StrengthAssignment guards the same value-copy contract for Strength.
// Validate also enforces the [0,1] range so out-of-range values must fail.
func TestPlan_StrengthAssignment(t *testing.T) {
	p1 := validPlan(t)
	p2 := *p1
	p2.Strength = 0.42
	if p1.Strength == 0.42 {
		t.Error("direct Strength assignment leaked through Plan value copy")
	}
	if p2.Strength != 0.42 {
		t.Errorf("copy.Strength = %v, want %v", p2.Strength, 0.42)
	}
}

// TestPlan_Strength_OutOfRange_Fails guards the Validate() range check.
func TestPlan_Strength_OutOfRange_Fails(t *testing.T) {
	for _, s := range []float64{-0.01, 1.01} {
		p := *validPlan(t)
		p.Strength = s
		err := p.Validate()
		if err == nil {
			t.Errorf("expected range error for Strength=%v", s)
			continue
		}
		if !errors.Is(err, ErrPlanStrengthOutOfRange) {
			t.Errorf("Strength=%v: expected ErrPlanStrengthOutOfRange, got %v", s, err)
		}
	}
}

// TestBlastRadius_Zero covers the trivial no-side-effect predicate.
func TestBlastRadius_Zero(t *testing.T) {
	if !(BlastRadius{}).Zero() {
		t.Error("zero-value BlastRadius must report Zero()=true")
	}
	if (BlastRadius{FileCount: 1}).Zero() {
		t.Error("non-zero BlastRadius must report Zero()=false")
	}
}

// TestPersistScope_Valid covers the 3-value enum guard.
func TestPersistScope_Valid(t *testing.T) {
	for _, s := range []PersistScope{PersistTransient, PersistSession, PersistPermanent} {
		if !s.Valid() {
			t.Errorf("PersistScope=%q should be valid", s)
		}
	}
	for _, s := range []PersistScope{"", "garbage", "session_permanent"} {
		if s.Valid() {
			t.Errorf("PersistScope=%q should be invalid", s)
		}
	}
}

// TestPlan_Validate_PersistScopeInvalid covers the Plan.Validate guard.
func TestPlan_Validate_PersistScopeInvalid(t *testing.T) {
	p := validPlan(t)
	p.BlastRadius.PersistScope = "permanent_session"
	if err := p.Validate(); err == nil {
		t.Fatal("expected PersistScope validation error")
	} else if !errors.Is(err, ErrPlanPersistScopeInvalid) {
		t.Errorf("expected ErrPlanPersistScopeInvalid, got %v", err)
	}
}

// TestPlan_Validate_FailureCriterionInvalidOp covers the PP-2 op whitelist.
func TestPlan_Validate_FailureCriterionInvalidOp(t *testing.T) {
	p := validPlan(t).WithFailureCriteria([]FailureCriterion{
		{Field: "exit_code", Op: "matches", Value: "0"},
	})
	err := p.Validate()
	if err == nil {
		t.Fatal("expected op whitelist error")
	}
	if !errors.Is(err, ErrPlanFailureCriteriaInvalidOp) {
		t.Errorf("expected ErrPlanFailureCriteriaInvalidOp, got %v", err)
	}
	if code := sharederrors.ErrorCode(err); code != "PLAN_PP2_OP_8021" {
		t.Errorf("expected error code PLAN_PP2_OP_8021, got %q", code)
	}
}

// TestPlan_Validate_FailureCriterionInvalidField covers the PP-2 observability
// whitelist — only fields extractable from ExecutionEvidence are accepted.
func TestPlan_Validate_FailureCriterionInvalidField(t *testing.T) {
	p := validPlan(t).WithFailureCriteria([]FailureCriterion{
		{Field: "deep.nested.user_secret", Op: "eq", Value: "x"},
	})
	err := p.Validate()
	if err == nil {
		t.Fatal("expected field observability error")
	}
	if !errors.Is(err, ErrPlanFailureCriteriaInvalidField) {
		t.Errorf("expected ErrPlanFailureCriteriaInvalidField, got %v", err)
	}
}

// TestPlan_Validate_BlastRadiusExceeded covers PP-3 with explicit thresholds
// via ValidateWithOpts (default 50/20/100000).
func TestPlan_Validate_BlastRadiusExceeded(t *testing.T) {
	for _, tc := range []struct {
		axis string
		br   BlastRadius
		code string
	}{
		{"FileCount", BlastRadius{FileCount: 51, APICallCount: 1, TokenCost: 1, PersistScope: PersistSession}, "PLAN_BLAST_8003"},
		{"APICallCount", BlastRadius{FileCount: 1, APICallCount: 21, TokenCost: 1, PersistScope: PersistSession}, "PLAN_BLAST_8003"},
		{"TokenCost", BlastRadius{FileCount: 1, APICallCount: 1, TokenCost: 100_001, PersistScope: PersistSession}, "PLAN_BLAST_8003"},
	} {
		p := validPlan(t).WithBlastRadius(tc.br)
		err := p.Validate()
		if err == nil {
			t.Errorf("%s: expected PP-3 violation, got nil", tc.axis)
			continue
		}
		if !errors.Is(err, ErrPlanBlastRadiusExceeded) {
			t.Errorf("%s: expected ErrPlanBlastRadiusExceeded, got %v", tc.axis, err)
		}
		if code := sharederrors.ErrorCode(err); code != tc.code {
			t.Errorf("%s: expected error code %q, got %q", tc.axis, tc.code, code)
		}
	}
}

// TestPlan_Validate_KindUnset covers the structural Kind check.
func TestPlan_Validate_KindUnset(t *testing.T) {
	p := *validPlan(t)
	p.Kind = KindUnset
	err := p.Validate()
	if err == nil {
		t.Fatal("expected KindUnset validation error")
	}
	if !errors.Is(err, ErrPlanKindUnset) {
		t.Errorf("expected ErrPlanKindUnset, got %v", err)
	}
	if code := sharederrors.ErrorCode(err); code != "PLAN_KIND_8001" {
		t.Errorf("expected error code PLAN_KIND_8001, got %q", code)
	}
}

// TestPlan_Validate_StepsEmpty covers the Steps non-empty structural check.
func TestPlan_Validate_StepsEmpty(t *testing.T) {
	p := validPlan(t)
	p.Steps = nil
	err := p.Validate()
	if err == nil {
		t.Fatal("expected empty-Steps validation error")
	}
	if !errors.Is(err, ErrPlanStepsEmpty) {
		t.Errorf("expected ErrPlanStepsEmpty, got %v", err)
	}
}

// TestPlan_MarshalJSON_RoundTrip guards wire-format stability for the full
// Plan struct (omitempty on SessionID/AnomaliesCount must match downstream
// D5 dashboards).
func TestPlan_MarshalJSON_RoundTrip(t *testing.T) {
	p := validPlan(t)
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(data), `"commitment_plan"`) {
		t.Errorf("Marshal missing kind wire-format: %s", data)
	}
	if !strings.Contains(string(data), `"source_observation_ids"`) {
		t.Errorf("Marshal missing source_observation_ids field: %s", data)
	}
}

// TestNewPlanKindUnsetError covers the helper's wire-format (CODE 8001).
func TestNewPlanKindUnsetError(t *testing.T) {
	e := NewPlanKindUnsetError()
	if code := sharederrors.ErrorCode(e); code != "PLAN_KIND_8001" {
		t.Errorf("code=%q, want PLAN_KIND_8001", code)
	}
	if !errors.Is(e, ErrPlanKindUnset) {
		t.Errorf("must wrap ErrPlanKindUnset, got %v", e)
	}
}

// TestNewPlanBlastRadiusExceededError covers the PP-3 helper wire-format and
// the axis/observed/limit detail propagation.
func TestNewPlanBlastRadiusExceededError(t *testing.T) {
	e := NewPlanBlastRadiusExceededError("FileCount", 99, 50)
	if code := sharederrors.ErrorCode(e); code != "PLAN_BLAST_8003" {
		t.Errorf("code=%q, want PLAN_BLAST_8003", code)
	}
	if !errors.Is(e, ErrPlanBlastRadiusExceeded) {
		t.Errorf("must wrap ErrPlanBlastRadiusExceeded, got %v", e)
	}
	if !strings.Contains(e.Error(), "FileCount") || !strings.Contains(e.Error(), "99") || !strings.Contains(e.Error(), "50") {
		t.Errorf("error message missing axis/observed/limit: %v", e)
	}
}

// TestValidateOpts_CustomThresholds verifies ValidateWithOpts respects caller
// thresholds (test-only escape hatch).
func TestValidateOpts_CustomThresholds(t *testing.T) {
	p := validPlan(t).WithBlastRadius(BlastRadius{
		FileCount: 10, APICallCount: 1, TokenCost: 1, PersistScope: PersistSession,
	})
	// Default: 10 < 50 → OK
	if err := p.Validate(); err != nil {
		t.Fatalf("default opts should accept FileCount=10, got %v", err)
	}
	// Custom: fileLimit=5 → reject
	if err := p.ValidateWithOpts(ValidateOpts{MaxBlastRadiusFileCount: 5}); err == nil {
		t.Error("expected PP-3 violation with fileLimit=5")
	}
}
