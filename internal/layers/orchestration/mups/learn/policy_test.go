// Package learn: LearnPolicy + classifyScenario tests (DM-20260707-001 PR-E, T65).
//
// 22-scenario table-driven coverage for ClassifyScenario +
// ClassifyScenarioWithReputation + LearnPolicy + AllScenarios.
//
// Test taxonomy:
//   - TestClassifyScenario_VerdictOutcomes: P1-P3 + F1-F3 + I1-I3 (9 cases)
//   - TestClassifyScenario_UpstreamErrors:  E1-E6 (6 cases)
//   - TestClassifyScenario_UserActions:     U1-U3 (3 cases)
//   - TestClassifyScenario_RollupOutcomes:  R1-R2 (2 cases)
//   - TestClassifyScenario_ForcePlanFP1:    FP1 with various β/(α+β) ratios
//   - TestClassifyScenario_All22Scenarios:  enumerates every AllScenarios() entry
//   - TestLearnPolicy_StringRoundTrip:      PolicyDecision.String() format
//   - TestForcePlanThreshold_Constant:     sanity-check the 0.7 threshold value
package learn

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// --- 22-scenario test matrix -------------------------------------------------

// scenarioCase is one row in the 22-scenario table test. Each case carries a
// PolicyInput, the expected ScenarioID + BayesianAction, and a human-readable
// reason (printed when the test fails for easy debugging).
type scenarioCase struct {
	name     string
	in       PolicyInput
	wantScen ScenarioID
	wantAct  BayesianAction
	// wantTagsAny: at least ONE of these tag substrings must be present in
	// the resulting decision's Tags (loose match — many tags carry dynamic
	// confidence values).
	wantTagsAny []string
	// skipReasonNonEmpty: when set, the resulting SkipReason must be non-empty.
	skipReasonNonEmpty bool
}

// makeInput builds a PolicyInput with the given Verdict + flags. Used by all
// table cases to keep test setup readable.
func makeInput(v workmodel.Verdict, isRollup bool, childCount int, hasLastGood bool, userAction string) PolicyInput {
	return PolicyInput{
		Verdict:           v,
		IsRollup:          isRollup,
		RollupChildCount:  childCount,
		HasLastGood:       hasLastGood,
		UserAction:        userAction,
		IsAsyncWrapper:    false,
	}
}

// TestClassifyScenario_VerdictOutcomes covers P1-P3 / F1-F3 / I1-I3 (9 cases).
func TestClassifyScenario_VerdictOutcomes(t *testing.T) {
	t.Parallel()

	cases := []scenarioCase{
		{
			name:     "P1_high_conf_pass",
			in:       makeInput(workmodel.Verdict{Kind: types.VerdictPass, Confidence: 0.85, Reason: "deliverable_match"}, false, 0, false, ""),
			wantScen: ScenarioP1HighConfPass,
			wantAct:  ActionAlphaBump,
			wantTagsAny: []string{"conf=high", "confidence=0.85"},
		},
		{
			name:     "P2_low_conf_pass",
			in:       makeInput(workmodel.Verdict{Kind: types.VerdictPass, Confidence: 0.45, Reason: "deliverable_match"}, false, 0, false, ""),
			wantScen: ScenarioP2LowConfPass,
			wantAct:  ActionAlphaBump,
			wantTagsAny: []string{"conf=low", "confidence=0.45"},
		},
		{
			name:     "P3_partial",
			in:       makeInput(workmodel.Verdict{Kind: types.VerdictPartial, Confidence: 0.55, Reason: "partial_evidence"}, false, 0, false, ""),
			wantScen: ScenarioP3Partial,
			wantAct:  ActionAlphaBump,
			wantTagsAny: []string{"partial", "confidence=0.55"},
		},
		{
			name:     "F1_system_error",
			in:       makeInput(workmodel.Verdict{Kind: types.VerdictFail, Confidence: 0.2, Reason: "system_outage_detected"}, false, 0, false, ""),
			wantScen: ScenarioF1SystemError,
			wantAct:  ActionBetaBump,
			wantTagsAny: []string{"system_error"},
		},
		{
			name:     "F2_user_error",
			in:       makeInput(workmodel.Verdict{Kind: types.VerdictFail, Confidence: 0.7, Reason: "missing_required_field"}, false, 0, false, ""),
			wantScen: ScenarioF2UserError,
			wantAct:  ActionBetaBump,
			wantTagsAny: []string{"user_error"},
		},
		{
			name:     "F3_ambiguous_empty_reason",
			in:       makeInput(workmodel.Verdict{Kind: types.VerdictFail, Confidence: 0.5, Reason: ""}, false, 0, false, ""),
			wantScen: ScenarioF3Ambiguous,
			wantAct:  ActionBetaBump,
			wantTagsAny: []string{"ambiguous"},
		},
		{
			name:     "I1_verifier_parse_failure",
			in:       makeInput(workmodel.Verdict{Kind: types.VerdictIndeterminate, Confidence: 0.5, IndeterminateReason: "verifier_parse_failure"}, false, 0, false, ""),
			wantScen: ScenarioI1VerifierFail,
			wantAct:  ActionNoChange,
			wantTagsAny: []string{"verifier_parse_failure"},
		},
		{
			name:     "I2_env_limited",
			in:       makeInput(workmodel.Verdict{Kind: types.VerdictIndeterminate, Confidence: 0.3, Reason: "env_network_timeout"}, false, 0, false, ""),
			wantScen: ScenarioI2EnvLimited,
			wantAct:  ActionNoChange,
			wantTagsAny: []string{"env_limited"},
		},
		{
			name:     "I3_other_indeterminate",
			in:       makeInput(workmodel.Verdict{Kind: types.VerdictIndeterminate, Confidence: 0.4, Reason: "verifier_consensus_split"}, false, 0, false, ""),
			wantScen: ScenarioI3OtherIndet,
			wantAct:  ActionNoChange,
			wantTagsAny: []string{"other_indeterminate"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ClassifyScenario(tc.in)
			if got.Scenario != tc.wantScen {
				t.Errorf("scenario mismatch: got %s, want %s", got.Scenario, tc.wantScen)
			}
			if got.Action != tc.wantAct {
				t.Errorf("action mismatch: got %s, want %s", got.Action, tc.wantAct)
			}
			if !hasAnyTag(got.Tags, tc.wantTagsAny) {
				t.Errorf("missing expected tag (any of %v) in %v", tc.wantTagsAny, got.Tags)
			}
		})
	}
}

// TestClassifyScenario_UpstreamErrors covers E1-E6 (6 cases).
func TestClassifyScenario_UpstreamErrors(t *testing.T) {
	t.Parallel()

	cases := []scenarioCase{
		{
			name:     "E1_plan_timeout",
			in:       makeInput(workmodel.Verdict{Kind: types.VerdictFail, Confidence: 0.0, Reason: "plan_llm_timeout"}, false, 0, false, ""),
			wantScen: ScenarioE1PlanTimeout,
			wantAct:  ActionSkip,
			wantTagsAny: []string{"upstream=plan", "no_learn"},
			skipReasonNonEmpty: true,
		},
		{
			name:     "E1_plan_timeout_with_detail",
			in:       makeInput(workmodel.Verdict{Kind: types.VerdictFail, Confidence: 0.0, Reason: "plan_llm_timeout:30s"}, false, 0, false, ""),
			wantScen: ScenarioE1PlanTimeout,
			wantAct:  ActionSkip,
			wantTagsAny: []string{"upstream=plan"},
			skipReasonNonEmpty: true,
		},
		{
			name:     "E2_plan_5xx",
			in:       makeInput(workmodel.Verdict{Kind: types.VerdictFail, Confidence: 0.0, Reason: "plan_llm_call_error:503"}, false, 0, false, ""),
			wantScen: ScenarioE2PlanCall5xx,
			wantAct:  ActionSkip,
			wantTagsAny: []string{"upstream=plan", "no_learn"},
			skipReasonNonEmpty: true,
		},
		{
			name:     "E3_plan_partial_response",
			in:       makeInput(workmodel.Verdict{Kind: types.VerdictFail, Confidence: 0.0, Reason: "plan_llm_partial_response"}, false, 0, false, ""),
			wantScen: ScenarioE3PlanPartial,
			wantAct:  ActionSkip,
			wantTagsAny: []string{"upstream=plan"},
			skipReasonNonEmpty: true,
		},
		{
			name:     "E4_post_exec_with_last_good",
			in:       makeInput(workmodel.Verdict{Kind: types.VerdictFail, Confidence: 0.3, Reason: "execute_error:tool_timeout"}, false, 0, true, ""),
			wantScen: ScenarioE4PostExec,
			wantAct:  ActionAlphaBump,
			wantTagsAny: []string{"execute_error", "last_good_round"},
		},
		{
			name:     "E5_verify_error",
			in:       makeInput(workmodel.Verdict{Kind: types.VerdictFail, Confidence: 0.2, Reason: "verify_llm_error"}, false, 0, false, ""),
			wantScen: ScenarioE5VerifyError,
			wantAct:  ActionBetaBump,
			wantTagsAny: []string{"verify_error", "verifier_infra"},
		},
		{
			name:     "E6_system_anomaly",
			in:       makeInput(workmodel.Verdict{Kind: types.VerdictFail, Confidence: 0.1, Reason: "system_anomaly:circuit_open"}, false, 0, false, ""),
			wantScen: ScenarioE6SystemAnomaly,
			wantAct:  ActionBetaBump,
			wantTagsAny: []string{"system_anomaly", "l3_pessimistic_eligible"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ClassifyScenario(tc.in)
			if got.Scenario != tc.wantScen {
				t.Errorf("scenario mismatch: got %s, want %s", got.Scenario, tc.wantScen)
			}
			if got.Action != tc.wantAct {
				t.Errorf("action mismatch: got %s, want %s", got.Action, tc.wantAct)
			}
			if !hasAnyTag(got.Tags, tc.wantTagsAny) {
				t.Errorf("missing expected tag (any of %v) in %v", tc.wantTagsAny, got.Tags)
			}
			if tc.skipReasonNonEmpty && got.SkipReason == "" {
				t.Errorf("expected non-empty SkipReason for ActionSkip, got empty")
			}
		})
	}
}

// TestClassifyScenario_UserActions covers U1-U3 (3 cases).
func TestClassifyScenario_UserActions(t *testing.T) {
	t.Parallel()

	cases := []scenarioCase{
		{
			name:     "U1_user_cancel",
			in:       makeInput(workmodel.Verdict{Kind: types.VerdictPass, Confidence: 0.9, Reason: "deliverable_match"}, false, 0, false, "cancel"),
			wantScen: ScenarioU1UserCancel,
			wantAct:  ActionBetaBump,
			wantTagsAny: []string{"user_action=cancel"},
		},
		{
			name:     "U2_user_accept",
			in:       makeInput(workmodel.Verdict{Kind: types.VerdictPass, Confidence: 0.8, Reason: "deliverable_match"}, false, 0, false, "accept"),
			wantScen: ScenarioU2UserAccept,
			wantAct:  ActionAlphaBump,
			wantTagsAny: []string{"user_action=accept"},
		},
		{
			name:     "U3_user_modify",
			in:       makeInput(workmodel.Verdict{Kind: types.VerdictPartial, Confidence: 0.5, Reason: "partial_evidence"}, false, 0, false, "modify"),
			wantScen: ScenarioU3UserModify,
			wantAct:  ActionNoChange,
			wantTagsAny: []string{"user_action=modify"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ClassifyScenario(tc.in)
			if got.Scenario != tc.wantScen {
				t.Errorf("scenario mismatch: got %s, want %s", got.Scenario, tc.wantScen)
			}
			if got.Action != tc.wantAct {
				t.Errorf("action mismatch: got %s, want %s", got.Action, tc.wantAct)
			}
			if !hasAnyTag(got.Tags, tc.wantTagsAny) {
				t.Errorf("missing expected tag (any of %v) in %v", tc.wantTagsAny, got.Tags)
			}
		})
	}
}

// TestClassifyScenario_RollupOutcomes covers R1-R2 (2 cases).
func TestClassifyScenario_RollupOutcomes(t *testing.T) {
	t.Parallel()

	cases := []scenarioCase{
		{
			name:     "R1_rollup_pass",
			in:       makeInput(workmodel.Verdict{Kind: types.VerdictPass, Confidence: 0.8, Reason: "rollup_synthesis"}, true, 3, false, ""),
			wantScen: ScenarioR1RollupPass,
			wantAct:  ActionAlphaBump,
			wantTagsAny: []string{"rollup", "rollup_count=3"},
		},
		{
			name:     "R2_rollup_fail",
			in:       makeInput(workmodel.Verdict{Kind: types.VerdictFail, Confidence: 0.6, Reason: "rollup_synthesis"}, true, 2, false, ""),
			wantScen: ScenarioR2RollupFail,
			wantAct:  ActionBetaBump,
			wantTagsAny: []string{"rollup", "rollup_count=2"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ClassifyScenario(tc.in)
			if got.Scenario != tc.wantScen {
				t.Errorf("scenario mismatch: got %s, want %s", got.Scenario, tc.wantScen)
			}
			if got.Action != tc.wantAct {
				t.Errorf("action mismatch: got %s, want %s", got.Action, tc.wantAct)
			}
			if !hasAnyTag(got.Tags, tc.wantTagsAny) {
				t.Errorf("missing expected tag (any of %v) in %v", tc.wantTagsAny, got.Tags)
			}
		})
	}
}

// TestClassifyScenario_ForcePlanFP1 covers FP1 (post-BayesianUpdate
// force_plan) for various β/(α+β) ratios.
func TestClassifyScenario_ForcePlanFP1(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		alpha      int
		beta       int
		wantAction BayesianAction
	}{
		{name: "FP1_cold_start_zero_zero", alpha: 0, beta: 0, wantAction: ActionNoChange},
		{name: "FP1_balanced_5_5_below_threshold", alpha: 5, beta: 5, wantAction: ActionNoChange},
		{name: "FP1_2_of_3_below_0_7", alpha: 1, beta: 2, wantAction: ActionNoChange},
		{name: "FP1_3_of_3_at_1_0_triggers", alpha: 0, beta: 3, wantAction: ActionForcePlan},
		{name: "FP1_8_of_10_at_0_8_triggers", alpha: 2, beta: 8, wantAction: ActionForcePlan},
		{name: "FP1_7_of_10_at_0_7_just_below_threshold", alpha: 3, beta: 7, wantAction: ActionNoChange},
		{name: "FP1_71_of_100_above_threshold_triggers", alpha: 29, beta: 71, wantAction: ActionForcePlan},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := makeInput(workmodel.Verdict{Kind: types.VerdictPass, Confidence: 0.85}, false, 0, false, "")
			got := ClassifyScenarioWithReputation(in, tc.alpha, tc.beta)
			if got.Action != tc.wantAction {
				t.Errorf("FP1 action mismatch for α=%d β=%d: got %s, want %s", tc.alpha, tc.beta, got.Action, tc.wantAction)
			}
			if got.Scenario != ScenarioF1ForcePlanTrigger {
				t.Errorf("FP1 scenario mismatch: got %s, want FP1", got.Scenario)
			}
		})
	}
}

// TestClassifyScenario_All22Scenarios enumerates AllScenarios() and confirms
// the count matches the spec (22). Catches accidental deletions.
func TestClassifyScenario_All22Scenarios(t *testing.T) {
	t.Parallel()
	got := AllScenarios()
	if len(got) != 22 {
		t.Errorf("AllScenarios() returned %d entries, expected 22", len(got))
	}
	seen := make(map[ScenarioID]bool)
	for _, s := range got {
		if seen[s] {
			t.Errorf("duplicate ScenarioID in AllScenarios(): %s", s)
		}
		seen[s] = true
	}
	// Spot-check the prefix coverage.
	requiredPrefixes := []string{"P1", "P2", "P3", "F1", "F2", "F3", "I1", "I2", "I3",
		"E1", "E2", "E3", "E4", "E5", "E6",
		"U1", "U2", "U3",
		"R1", "R2",
		"FP1", "FP2"}
	for _, want := range requiredPrefixes {
		if !seen[ScenarioID(want)] {
			t.Errorf("AllScenarios() missing required ScenarioID %s", want)
		}
	}
}

// TestLearnPolicy_StringRoundTrip verifies PolicyDecision.String() includes
// scenario + action + tags (skip_reason only when set).
func TestLearnPolicy_StringRoundTrip(t *testing.T) {
	t.Parallel()

	// Case 1: P1 — should NOT include skip_reason.
	dec1 := ClassifyScenario(makeInput(workmodel.Verdict{Kind: types.VerdictPass, Confidence: 0.85}, false, 0, false, ""))
	s1 := dec1.String()
	if !strings.Contains(s1, "scenario=P1") {
		t.Errorf("String() missing scenario=P1: %s", s1)
	}
	if !strings.Contains(s1, "action=alpha_bump") {
		t.Errorf("String() missing action=alpha_bump: %s", s1)
	}
	if strings.Contains(s1, "skip_reason=") {
		t.Errorf("String() unexpectedly contains skip_reason for non-skip action: %s", s1)
	}

	// Case 2: E1 (skip) — should include skip_reason.
	dec2 := ClassifyScenario(makeInput(workmodel.Verdict{Kind: types.VerdictFail, Reason: "plan_llm_timeout"}, false, 0, false, ""))
	s2 := dec2.String()
	if !strings.Contains(s2, "scenario=E1") {
		t.Errorf("String() missing scenario=E1: %s", s2)
	}
	if !strings.Contains(s2, "action=skip") {
		t.Errorf("String() missing action=skip: %s", s2)
	}
	if !strings.Contains(s2, "skip_reason=plan_llm_timeout") {
		t.Errorf("String() missing skip_reason for skip action: %s", s2)
	}
}

// TestForcePlanThreshold_Constant sanity-checks the constant value (0.7).
// A change to this value is a meaningful policy shift and should require a
// deliberate PR review, so the test guards against accidental drift.
func TestForcePlanThreshold_Constant(t *testing.T) {
	t.Parallel()
	if forcePlanThreshold != 0.7 {
		t.Errorf("forcePlanThreshold drifted: got %v, want 0.7", forcePlanThreshold)
	}
}

// TestLearnPolicy_EvaluateAndEvaluateWithReputation verifies the struct
// methods delegate to the package-level functions correctly.
func TestLearnPolicy_EvaluateAndEvaluateWithReputation(t *testing.T) {
	t.Parallel()
	p := NewLearnPolicy()

	in := PolicyInput{
		Verdict: workmodel.Verdict{Kind: types.VerdictPass, Confidence: 0.85},
	}
	got := p.Evaluate(in)
	if got.Scenario != ScenarioP1HighConfPass {
		t.Errorf("Evaluate() = %s, want P1", got.Scenario)
	}

	// FP1 above threshold
	got2 := p.EvaluateWithReputation(in, 1, 9)
	if got2.Action != ActionForcePlan {
		t.Errorf("EvaluateWithReputation(1,9) = %s, want force_plan", got2.Action)
	}
	// FP1 below threshold
	got3 := p.EvaluateWithReputation(in, 7, 3)
	if got3.Action != ActionNoChange {
		t.Errorf("EvaluateWithReputation(7,3) = %s, want no_change", got3.Action)
	}
}

// --- helpers -----------------------------------------------------------------

// hasAnyTag returns true if any substring in needles appears in haystack.
func hasAnyTag(haystack []string, needles []string) bool {
	for _, n := range needles {
		for _, h := range haystack {
			if strings.Contains(h, n) {
				return true
			}
		}
	}
	return false
}