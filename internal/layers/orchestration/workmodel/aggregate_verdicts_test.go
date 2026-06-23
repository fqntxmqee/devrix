package workmodel

import (
	"encoding/json"
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

// --- AggregationStrategy enum tests ---

func TestAggregationStrategy_String_4Strategies(t *testing.T) {
	cases := []struct {
		strat AggregationStrategy
		want  string
	}{
		{WeakConjunction, "weak_conjunction"},
		{StrongConjunction, "strong_conjunction"},
		{Majority, "majority"},
		{ThresholdByPass, "threshold_by_pass"},
	}
	for _, tc := range cases {
		if got := tc.strat.String(); got != tc.want {
			t.Errorf("AggregationStrategy(%d).String() = %q, want %q", uint8(tc.strat), got, tc.want)
		}
	}
}

func TestAggregationStrategy_ParseAggregationStrategy_4Strategies(t *testing.T) {
	cases := []struct {
		in   string
		want AggregationStrategy
	}{
		{"weak_conjunction", WeakConjunction},
		{"strong_conjunction", StrongConjunction},
		{"majority", Majority},
		{"threshold_by_pass", ThresholdByPass},
	}
	for _, tc := range cases {
		got, err := ParseAggregationStrategy(tc.in)
		if err != nil {
			t.Errorf("ParseAggregationStrategy(%q) returned error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseAggregationStrategy(%q) = %d, want %d", tc.in, uint8(got), uint8(tc.want))
		}
	}
}

func TestAggregationStrategy_ParseAggregationStrategy_UnknownFailFast(t *testing.T) {
	_, err := ParseAggregationStrategy("invalid_strategy")
	if err == nil {
		t.Fatal("ParseAggregationStrategy(\"invalid_strategy\") should return error")
	}
}

func TestAggregationStrategy_MarshalJSON_WireFormat(t *testing.T) {
	cases := []struct {
		strat AggregationStrategy
		want  string
	}{
		{WeakConjunction, `"weak_conjunction"`},
		{StrongConjunction, `"strong_conjunction"`},
		{Majority, `"majority"`},
		{ThresholdByPass, `"threshold_by_pass"`},
	}
	for _, tc := range cases {
		got, err := json.Marshal(tc.strat)
		if err != nil {
			t.Errorf("Marshal(%v) returned error: %v", tc.strat, err)
			continue
		}
		if string(got) != tc.want {
			t.Errorf("Marshal(%v) = %s, want %s", tc.strat, got, tc.want)
		}
	}
}

func TestAggregationStrategy_UnmarshalJSON_EmptyString_DefaultsToZeroValue(t *testing.T) {
	var s AggregationStrategy = ThresholdByPass
	if err := json.Unmarshal([]byte(`""`), &s); err != nil {
		t.Fatalf("Unmarshal empty string returned error: %v", err)
	}
	if s != WeakConjunction {
		t.Errorf("Unmarshal empty string: got %d, want %d (WeakConjunction zero value)", uint8(s), uint8(WeakConjunction))
	}
}

// --- AggregateVerdicts boundary tests ---

func TestAggregateVerdicts_EmptySlice_ReturnsIndeterminate(t *testing.T) {
	got := AggregateVerdicts(nil, WeakConjunction)
	if got.Kind != types.VerdictIndeterminate {
		t.Errorf("Empty slice: Kind = %v, want VerdictIndeterminate", got.Kind)
	}
	if got.Reason != "empty_verdict_set" {
		t.Errorf("Empty slice: Reason = %q, want %q", got.Reason, "empty_verdict_set")
	}

	got = AggregateVerdicts([]Verdict{}, StrongConjunction)
	if got.Kind != types.VerdictIndeterminate {
		t.Errorf("Empty slice (literal): Kind = %v, want VerdictIndeterminate", got.Kind)
	}
}

func TestAggregateVerdicts_SingleVerdict_ReturnsDirectly(t *testing.T) {
	v := Verdict{Kind: types.VerdictPass, Confidence: 0.9, Reason: "all_good", SourceID: "verifier_1"}
	got := AggregateVerdicts([]Verdict{v}, Majority)
	if got != v {
		t.Errorf("Single verdict: got %+v, want %+v", got, v)
	}
}

func TestAggregateVerdicts_AllSameKind_ReturnsThatKind(t *testing.T) {
	vs := []Verdict{
		{Kind: types.VerdictPass, Confidence: 0.8, Reason: "good"},
		{Kind: types.VerdictPass, Confidence: 0.9, Reason: "very good — all criteria met"},
		{Kind: types.VerdictPass, Confidence: 0.7, Reason: "ok"},
	}
	got := AggregateVerdicts(vs, StrongConjunction)
	if got.Kind != types.VerdictPass {
		t.Errorf("Homogeneous Pass: Kind = %v, want VerdictPass", got.Kind)
	}
	// Average confidence.
	expectedConf := (0.8 + 0.9 + 0.7) / 3.0
	if got.Confidence != expectedConf {
		// IEEE 754 float drift tolerance: 0.8 + 0.9 + 0.7 = 2.4000000000000004.
		if diff := got.Confidence - expectedConf; diff < 0 || diff > 1e-9 {
			t.Errorf("Homogeneous Pass: Confidence = %f, want %f (drift %g)", got.Confidence, expectedConf, diff)
		}
	}
	// Longest reason wins.
	if got.Reason != "very good — all criteria met" {
		t.Errorf("Homogeneous Pass: Reason = %q, want %q", got.Reason, "very good — all criteria met")
	}
}

// --- Strategy: WeakConjunction (OR semantics) ---

func TestAggregateVerdicts_WeakConjunction_AnyPassWins(t *testing.T) {
	vs := []Verdict{
		{Kind: types.VerdictFail, Confidence: 0.8, Reason: "step1_failed"},
		{Kind: types.VerdictPass, Confidence: 0.9, Reason: "step2_passed"},
		{Kind: types.VerdictIndeterminate, Confidence: 0.5, Reason: "step3_abstain"},
	}
	got := AggregateVerdicts(vs, WeakConjunction)
	if got.Kind != types.VerdictPass {
		t.Errorf("WeakConjunction with Pass+Fail: Kind = %v, want VerdictPass", got.Kind)
	}
}

func TestAggregateVerdicts_WeakConjunction_AnyFailLoses(t *testing.T) {
	vs := []Verdict{
		{Kind: types.VerdictIndeterminate, Confidence: 0.5, Reason: "abstain"},
		{Kind: types.VerdictFail, Confidence: 0.9, Reason: "explicit_fail"},
	}
	got := AggregateVerdicts(vs, WeakConjunction)
	if got.Kind != types.VerdictFail {
		t.Errorf("WeakConjunction with Fail+Indeterminate: Kind = %v, want VerdictFail", got.Kind)
	}
}

func TestAggregateVerdicts_WeakConjunction_NoPassNoFail_Indeterminate(t *testing.T) {
	vs := []Verdict{
		{Kind: types.VerdictIndeterminate, Confidence: 0.5, Reason: "abstain"},
		{Kind: types.VerdictPartial, Confidence: 0.6, Reason: "partial"},
	}
	got := AggregateVerdicts(vs, WeakConjunction)
	if got.Kind != types.VerdictIndeterminate {
		t.Errorf("WeakConjunction with Partial+Indeterminate: Kind = %v, want VerdictIndeterminate", got.Kind)
	}
}

// --- Strategy: StrongConjunction (AND semantics) ---

func TestAggregateVerdicts_StrongConjunction_AllPassRequired(t *testing.T) {
	vs := []Verdict{
		{Kind: types.VerdictPass, Confidence: 0.9, Reason: "good"},
		{Kind: types.VerdictPass, Confidence: 0.85, Reason: "also good"},
		{Kind: types.VerdictPass, Confidence: 0.95, Reason: "perfect"},
	}
	got := AggregateVerdicts(vs, StrongConjunction)
	if got.Kind != types.VerdictPass {
		t.Errorf("StrongConjunction all-Pass: Kind = %v, want VerdictPass", got.Kind)
	}
}

func TestAggregateVerdicts_StrongConjunction_OneFailLoses(t *testing.T) {
	vs := []Verdict{
		{Kind: types.VerdictPass, Confidence: 0.9, Reason: "good"},
		{Kind: types.VerdictFail, Confidence: 0.95, Reason: "explicit_fail"},
		{Kind: types.VerdictPass, Confidence: 0.85, Reason: "also good"},
	}
	got := AggregateVerdicts(vs, StrongConjunction)
	if got.Kind != types.VerdictFail {
		t.Errorf("StrongConjunction with one Fail: Kind = %v, want VerdictFail", got.Kind)
	}
}

func TestAggregateVerdicts_StrongConjunction_PartialAllowed(t *testing.T) {
	vs := []Verdict{
		{Kind: types.VerdictPass, Confidence: 0.9, Reason: "good"},
		{Kind: types.VerdictPartial, Confidence: 0.6, Reason: "partial_ok"},
	}
	got := AggregateVerdicts(vs, StrongConjunction)
	// Partial is not Fail, not Indeterminate, so AND collapses to Pass.
	if got.Kind != types.VerdictPass {
		t.Errorf("StrongConjunction Pass+Partial: Kind = %v, want VerdictPass", got.Kind)
	}
}

// --- Strategy: Majority (plurality with strict > half) ---

func TestAggregateVerdicts_Majority_HalfStrict(t *testing.T) {
	// 3 verdicts: 2 Pass, 1 Fail. PASS > len/2 = 1 → PASS.
	vs := []Verdict{
		{Kind: types.VerdictPass, Confidence: 0.9, Reason: "p1"},
		{Kind: types.VerdictPass, Confidence: 0.85, Reason: "p2"},
		{Kind: types.VerdictFail, Confidence: 0.7, Reason: "f1"},
	}
	got := AggregateVerdicts(vs, Majority)
	if got.Kind != types.VerdictPass {
		t.Errorf("Majority 2P/1F of 3: Kind = %v, want VerdictPass", got.Kind)
	}
}

func TestAggregateVerdicts_Majority_TieGoesIndeterminate(t *testing.T) {
	// 4 verdicts: 2 Pass, 2 Fail. PASS > 2 = false, FAIL > 2 = false → Indeterminate.
	vs := []Verdict{
		{Kind: types.VerdictPass, Confidence: 0.9, Reason: "p1"},
		{Kind: types.VerdictPass, Confidence: 0.85, Reason: "p2"},
		{Kind: types.VerdictFail, Confidence: 0.7, Reason: "f1"},
		{Kind: types.VerdictFail, Confidence: 0.95, Reason: "f2"},
	}
	got := AggregateVerdicts(vs, Majority)
	if got.Kind != types.VerdictIndeterminate {
		t.Errorf("Majority 2P/2F of 4: Kind = %v, want VerdictIndeterminate", got.Kind)
	}
}

func TestAggregateVerdicts_Majority_FailPlurality(t *testing.T) {
	// 5 verdicts: 1 Pass, 3 Fail, 1 Indeterminate. FAIL > 2 → FAIL.
	vs := []Verdict{
		{Kind: types.VerdictPass, Confidence: 0.9, Reason: "p1"},
		{Kind: types.VerdictFail, Confidence: 0.7, Reason: "f1"},
		{Kind: types.VerdictFail, Confidence: 0.95, Reason: "f2"},
		{Kind: types.VerdictFail, Confidence: 0.8, Reason: "f3"},
		{Kind: types.VerdictIndeterminate, Confidence: 0.5, Reason: "i1"},
	}
	got := AggregateVerdicts(vs, Majority)
	if got.Kind != types.VerdictFail {
		t.Errorf("Majority 1P/3F/1I of 5: Kind = %v, want VerdictFail", got.Kind)
	}
}

// --- Strategy: ThresholdByPass (configurable PASS count) ---

func TestAggregateVerdicts_ThresholdByPass_DefaultOne(t *testing.T) {
	// Default threshold = 1, so any PASS wins.
	vs := []Verdict{
		{Kind: types.VerdictFail, Confidence: 0.7, Reason: "f1"},
		{Kind: types.VerdictPass, Confidence: 0.9, Reason: "p1"},
		{Kind: types.VerdictIndeterminate, Confidence: 0.5, Reason: "i1"},
	}
	got := AggregateVerdicts(vs, ThresholdByPass)
	if got.Kind != types.VerdictPass {
		t.Errorf("ThresholdByPass default=1 with 1 Pass: Kind = %v, want VerdictPass", got.Kind)
	}
}

func TestAggregateVerdicts_ThresholdByPass_NoPass_Indeterminate(t *testing.T) {
	vs := []Verdict{
		{Kind: types.VerdictFail, Confidence: 0.7, Reason: "f1"},
		{Kind: types.VerdictIndeterminate, Confidence: 0.5, Reason: "i1"},
	}
	got := AggregateVerdicts(vs, ThresholdByPass)
	if got.Kind != types.VerdictIndeterminate {
		t.Errorf("ThresholdByPass with no Pass: Kind = %v, want VerdictIndeterminate", got.Kind)
	}
}

// --- Confidence & Reason aggregation ---

func TestAggregateVerdicts_ConfidenceAverageAndLongestReason(t *testing.T) {
	vs := []Verdict{
		{Kind: types.VerdictPass, Confidence: 0.6, Reason: "ok"},
		{Kind: types.VerdictPass, Confidence: 0.9, Reason: "very detailed reason"},
		{Kind: types.VerdictPass, Confidence: 0.3, Reason: "weak pass"},
	}
	got := AggregateVerdicts(vs, StrongConjunction)
	expectedConf := (0.6 + 0.9 + 0.3) / 3.0
	if diff := got.Confidence - expectedConf; diff < 0 || diff > 1e-9 {
		t.Errorf("Confidence: got %f, want %f (drift %g)", got.Confidence, expectedConf, diff)
	}
	if got.Reason != "very detailed reason" {
		t.Errorf("Reason: got %q, want %q", got.Reason, "very detailed reason")
	}
}

// --- Verdict immutability (With* contract) ---

func TestVerdict_WithKind_ReturnsNewVerdict(t *testing.T) {
	v := Verdict{Kind: types.VerdictPass, Confidence: 0.9}
	v2 := v.WithKind(types.VerdictFail)
	if v.Kind != types.VerdictPass {
		t.Errorf("Original Verdict.Kind mutated: got %v, want VerdictPass", v.Kind)
	}
	if v2.Kind != types.VerdictFail {
		t.Errorf("New Verdict.Kind: got %v, want VerdictFail", v2.Kind)
	}
}

func TestVerdict_WithConfidence_ClampedToRange(t *testing.T) {
	v := Verdict{Kind: types.VerdictPass}
	v1 := v.WithConfidence(1.5) // > 1
	if v1.Confidence != 0.5 {
		t.Errorf("Confidence > 1 not clamped to fallback 0.5: got %f", v1.Confidence)
	}
	v2 := v.WithConfidence(-0.3) // < 0
	if v2.Confidence != 0.5 {
		t.Errorf("Confidence < 0 not clamped to fallback 0.5: got %f", v2.Confidence)
	}
	v3 := v.WithConfidence(0.7)
	if v3.Confidence != 0.7 {
		t.Errorf("Valid Confidence 0.7: got %f, want 0.7", v3.Confidence)
	}
}

func TestVerdict_WithReasonAndSourceID(t *testing.T) {
	v := Verdict{Kind: types.VerdictPass, Reason: "old", SourceID: "src_old"}
	v2 := v.WithReason("new").WithSourceID("src_new")
	if v.Reason != "old" || v.SourceID != "src_old" {
		t.Errorf("Original Verdict mutated: got Reason=%q SourceID=%q", v.Reason, v.SourceID)
	}
	if v2.Reason != "new" || v2.SourceID != "src_new" {
		t.Errorf("New Verdict: got Reason=%q SourceID=%q", v2.Reason, v2.SourceID)
	}
}