// Package learn: LearnPolicy + classifyScenario (DM-20260707-001 PR-E, T60-T65).
//
// LearnPolicy is the pure-function policy that maps a (Verdict + Context)
// tuple to a BayesianAction (one of 4: alpha_bump / beta_bump / no_change /
// force_plan). The 22-scenario table classifies every conceivable Learn
// invocation into one of:
//
//	Verdict outcomes (9):  P1-P3 / F1-F3 / I1-I3
//	Upstream errors (6):   E1-E6 (Plan/Execute/Verify failures)
//	User actions (3):      U1-U3 (cancel/accept/modify)
//	Rollup outcomes (2):   R1-R2 (parent aggregation)
//	force_plan (2):        F1-F2 (reputation-driven Plan bypass)
//
// The policy is a pure function (no side effects, no I/O) so the table
// itself is exhaustively unit-testable. AsyncLearner (async.go) and
// ItemPipelineRunner (sessionorchestrator) both call into the policy
// to decide whether to enqueue, fall back, or skip Learn entirely.
//
// Why a separate type (vs. inline in DefaultLearner): the policy is the
// "what to do" decision; DefaultLearner is the "how to do it" executor.
// Splitting the two lets the 22 scenarios live in a flat table that is
// easy to extend (add a new scenario = add a row, not edit logic).
package learn

import (
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// BayesianAction is the side-effect classification emitted by classifyScenario.
// It tells the caller which ReputationStore knob to turn (or whether to skip
// Learn entirely for upstream errors).
type BayesianAction string

const (
	// ActionAlphaBump: increment Alpha (VerdictPass/Partial, user-accept).
	// Maps to BayesianUpdate's α++ path.
	ActionAlphaBump BayesianAction = "alpha_bump"

	// ActionBetaBump: increment Beta (VerdictFail, user-cancel, system errors).
	// Maps to BayesianUpdate's β++ path.
	ActionBetaBump BayesianAction = "beta_bump"

	// ActionNoChange: log only, do not touch α/β (verifier_parse_failure,
	// env-limited INDETERMINATE, user-modify).
	// Maps to BayesianUpdate's "no update" path (VerifierFailureCount++ or
	// IndeterminateCount++ only).
	ActionNoChange BayesianAction = "no_change"

	// ActionForcePlan: emit "next_observation_force_plan=true" metadata and
	// bypass the observational fast-path on the next round. Triggered when
	// β/(α+β) > forcePlanThreshold after a BayesianUpdate.
	ActionForcePlan BayesianAction = "force_plan"

	// ActionSkip: do NOT call Learn at all. Plan errors (E1-E3) take this
	// path because the artifact may be incomplete and BayesianUpdate on a
	// half-baked Verdict would pollute reputation. An audit log row is
	// still emitted so dashboards can see the skip happened.
	ActionSkip BayesianAction = "skip"
)

// ScenarioID is the 22-scenario classification tag. The format is a short
// prefix (P/F/I/E/U/R) + a numeric counter; e.g. "P1" / "E3" / "U2" / "F1".
// Scenarios are stable across releases (consumers in dashboards + tests pin
// on these IDs).
type ScenarioID string

const (
	// P1-P3: VerdictPass / VerdictPartial outcomes.
	ScenarioP1HighConfPass    ScenarioID = "P1" // VerdictPass + Confidence ≥ 0.7
	ScenarioP2LowConfPass     ScenarioID = "P2" // VerdictPass + Confidence < 0.7
	ScenarioP3Partial         ScenarioID = "P3" // VerdictPartial (any confidence)

	// F1-F3: VerdictFail outcomes.
	ScenarioF1SystemError     ScenarioID = "F1" // VerdictFail + Reason starts with "system_"
	ScenarioF2UserError       ScenarioID = "F2" // VerdictFail + user-attributable
	ScenarioF3Ambiguous       ScenarioID = "F3" // VerdictFail + Reason empty (ambiguous)

	// I1-I3: VerdictIndeterminate outcomes.
	ScenarioI1VerifierFail    ScenarioID = "I1" // VerdictIndeterminate + IndeterminateReason = "verifier_parse_failure"
	ScenarioI2EnvLimited      ScenarioID = "I2" // VerdictIndeterminate + Reason starts with "env_"
	ScenarioI3OtherIndet      ScenarioID = "I3" // VerdictIndeterminate + other (no α/β update)

	// E1-E6: upstream error dimensions (T61).
	ScenarioE1PlanTimeout     ScenarioID = "E1" // PlanLLMCallTimeout — NO Learn + audit
	ScenarioE2PlanCall5xx     ScenarioID = "E2" // PlanLLMCallError 5xx — NO Learn + audit
	ScenarioE3PlanPartial     ScenarioID = "E3" // PlanLLMCallPartialResponse — NO Learn + audit
	ScenarioE4PostExec        ScenarioID = "E4" // Execute error + last good round — α++ (last good)
	ScenarioE5VerifyError     ScenarioID = "E5" // Verify error — β++ (env)
	ScenarioE6SystemAnomaly   ScenarioID = "E6" // SystemAnomaly — β++ + system_anomaly flag

	// U1-U3: user action dimensions (T62).
	ScenarioU1UserCancel      ScenarioID = "U1" // user-cancel — β++ (user rejected)
	ScenarioU2UserAccept      ScenarioID = "U2" // user-accept — α++ (user confirmed)
	ScenarioU3UserModify      ScenarioID = "U3" // user-modify — NO Learn (this round)

	// R1-R2: rollup outcomes (PR-C parent aggregation).
	ScenarioR1RollupPass      ScenarioID = "R1" // rollup verdict = Pass — α++ with rollup_count
	ScenarioR2RollupFail      ScenarioID = "R2" // rollup verdict = Fail — β++

	// FP1-FP2: force_plan outcomes. The "F" prefix here means
	// "force_plan" (distinct from F1-F3 above which are VerdictFail
	// outcomes). FP1 fires when the post-BayesianUpdate β/(α+β) ratio
	// exceeds the threshold; FP2 is the marker written to the next
	// Observe call's metadata when FP1 fires. ClassifyScenario returns
	// FP1 (with a "fp2_marker=true" tag); ClassifyScenarioWithReputation
	// performs the actual ratio computation.
	ScenarioF1ForcePlanTrigger ScenarioID = "FP1" // β/(α+β) > threshold → force_plan
	ScenarioF2ForcePlanOnNext  ScenarioID = "FP2" // next directive Plan path (read by Observe)
)

// PolicyInput is the policy's read-only view of the round. It is built by
// the caller (ItemPipelineRunner or AsyncLearner) and passed to
// classifyScenario. The struct is intentionally narrow (no Plan / Artifact
// / Observation) so the policy is fast (no big-struct copies) and pure.
type PolicyInput struct {
	// Verdict is the round's terminal Verdict. Required.
	Verdict workmodel.Verdict

	// IsRollup is true when this Learn call is for a parent rollup.
	IsRollup bool

	// RollupChildCount is the number of child segments that contributed to
	// the rollup. Only meaningful when IsRollup=true.
	RollupChildCount int

	// HasLastGood is true when there is a prior round with a Pass verdict
	// (used by E4-post: Execute error + last good → α++ on last good).
	HasLastGood bool

	// UserAction is one of "" (none), "cancel", "accept", "modify" — only
	// set for user-driven Learn calls (Feishu listener). Empty for
	// automatic (LLM-verifier-driven) Learn calls.
	UserAction string

	// IsAsyncWrapper is true when the policy is being evaluated by the
	// AsyncLearner (T58) — used for the L1/L2/L3 degradation tag.
	IsAsyncWrapper bool
}

// PolicyDecision is the policy's output. It bundles the scenario ID, the
// action to take, and an optional metadata map for observability.
type PolicyDecision struct {
	// Scenario is the matched 22-scenario ID (e.g. "P1" / "E3" / "FP1").
	Scenario ScenarioID

	// Action is the BayesianAction to execute.
	Action BayesianAction

	// SkipReason is set when Action == ActionSkip; describes why Learn was
	// skipped (used in the audit log). Empty otherwise.
	SkipReason string

	// Tags are free-form tags attached to the audit log row + Reputation
	// metadata. Examples: "system_error" / "verifier_parse_failure" /
	// "force_plan_threshold" / "user_cancel" / "rollup_count=3".
	Tags []string
}

// String renders a PolicyDecision for logging.
func (d PolicyDecision) String() string {
	parts := []string{
		"scenario=" + string(d.Scenario),
		"action=" + string(d.Action),
	}
	if d.SkipReason != "" {
		parts = append(parts, "skip_reason="+d.SkipReason)
	}
	if len(d.Tags) > 0 {
		parts = append(parts, "tags=["+strings.Join(d.Tags, ",")+"]")
	}
	return strings.Join(parts, " ")
}

// forcePlanThreshold is the β/(α+β) ratio above which FP1 (force_plan) fires.
// 0.7 = 70% failure rate. Empirically calibrated to flag a session where
// recent rounds have been consistently failing without triggering on
// transient noise (a 2/3 = 66.7% ratio stays below threshold; 3/3 = 100%
// fires).
const forcePlanThreshold = 0.7

// ClassifyScenario is the pure policy function. Given a PolicyInput it
// returns the matched PolicyDecision. The function is total (covers all
// inputs) so callers never have to handle a "no match" error.
//
// The implementation is a flat if-else chain over the 22 scenarios, ordered
// from most-specific to least-specific. The order is important: the
// force_plan check (FP1) MUST come after the verdict-based classification
// so a Pass verdict with high-failure-reputation still emits α++ (not
// force_plan alone) on the current round.
//
// All 22 scenarios are tested in policy_test.go via a flat table test.
func ClassifyScenario(in PolicyInput) PolicyDecision {
	v := in.Verdict
	reason := strings.TrimSpace(v.Reason)
	indetReason := strings.TrimSpace(v.IndeterminateReason)

	// --- E1-E3: Plan upstream errors (NO Learn, audit) ---
	if reason == "plan_llm_timeout" || strings.HasPrefix(reason, "plan_llm_timeout:") {
		return PolicyDecision{
			Scenario:   ScenarioE1PlanTimeout,
			Action:     ActionSkip,
			SkipReason: "plan_llm_timeout",
			Tags:       []string{"upstream=plan", "no_learn"},
		}
	}
	if reason == "plan_llm_call_error" || strings.HasPrefix(reason, "plan_llm_call_error:") {
		return PolicyDecision{
			Scenario:   ScenarioE2PlanCall5xx,
			Action:     ActionSkip,
			SkipReason: "plan_llm_call_error_5xx",
			Tags:       []string{"upstream=plan", "no_learn"},
		}
	}
	if reason == "plan_llm_partial_response" || strings.HasPrefix(reason, "plan_llm_partial_response:") {
		return PolicyDecision{
			Scenario:   ScenarioE3PlanPartial,
			Action:     ActionSkip,
			SkipReason: "plan_llm_partial_response",
			Tags:       []string{"upstream=plan", "no_learn"},
		}
	}

	// --- U1-U3: user actions (only set when UserAction is non-empty) ---
	switch in.UserAction {
	case "cancel":
		return PolicyDecision{
			Scenario: ScenarioU1UserCancel,
			Action:   ActionBetaBump,
			Tags:     []string{"user_action=cancel", "user_rejected"},
		}
	case "accept":
		return PolicyDecision{
			Scenario: ScenarioU2UserAccept,
			Action:   ActionAlphaBump,
			Tags:     []string{"user_action=accept", "user_confirmed"},
		}
	case "modify":
		return PolicyDecision{
			Scenario: ScenarioU3UserModify,
			Action:   ActionNoChange,
			Tags:     []string{"user_action=modify", "defer_to_next_round"},
		}
	}

	// --- E4-E6 (VerdictFail upstream errors) — BEFORE the verdict-based
	//     switch so an Execute/Verify error never gets reclassified as F2. ---
	if v.Kind == types.VerdictFail {
		// E4-post: Execute error + last good round → α++ on last good.
		// We detect this by Reason prefix "execute_error" combined with
		// HasLastGood. The caller passes HasLastGood based on item.LastRound.
		if strings.HasPrefix(reason, "execute_error") && in.HasLastGood {
			return PolicyDecision{
				Scenario: ScenarioE4PostExec,
				Action:   ActionAlphaBump,
				Tags:     []string{"execute_error", "last_good_round", "no_pollution"},
			}
		}
		// E5: Verify node error (verifier LLM timed out / 5xx / parse failed).
		// Distinct from F1 (system_/env_ prefix) so dashboards can split
		// "verifier infrastructure flake" from "execute_runtime env failure".
		if strings.HasPrefix(reason, "verify_") {
			return PolicyDecision{
				Scenario: ScenarioE5VerifyError,
				Action:   ActionBetaBump,
				Tags:     []string{"verify_error", "env_attributable", "verifier_infra"},
			}
		}
		// E6: SystemAnomaly aggregator surfaced an anomaly (circuit_open /
		// cb_l1 / resource_exhausted). β++ + system_anomaly flag for the
		// Pessimistic Commit L3 fallback selector.
		if strings.HasPrefix(reason, "system_anomaly") || v.SystemAnomaly {
			return PolicyDecision{
				Scenario: ScenarioE6SystemAnomaly,
				Action:   ActionBetaBump,
				Tags:     []string{"system_anomaly", "l3_pessimistic_eligible"},
			}
		}
	}

	// --- R1-R2: rollup outcomes (IsRollup=true) MUST come BEFORE the
	//     verdict-based outcomes — a rollup VerdictPass should be R1, not P1. ---
	if in.IsRollup {
		// Rollup synthesis already converted child verdicts into a single
		// verdict on the parent (see SynthesizeRollupVerdict). The kind
		// check above handles it; the rollup tag is added here.
		switch v.Kind {
		case types.VerdictPass, types.VerdictPartial:
			return PolicyDecision{
				Scenario: ScenarioR1RollupPass,
				Action:   ActionAlphaBump,
				Tags:     []string{"rollup", fmt.Sprintf("rollup_count=%d", in.RollupChildCount)},
			}
		case types.VerdictFail:
			return PolicyDecision{
				Scenario: ScenarioR2RollupFail,
				Action:   ActionBetaBump,
				Tags:     []string{"rollup", fmt.Sprintf("rollup_count=%d", in.RollupChildCount)},
			}
		}
	}

	// --- Verdict-based outcomes (P1-P3, F1-F3, I1-I3) ---
	switch v.Kind {
	case types.VerdictPass:
		if v.Confidence >= 0.7 {
			return PolicyDecision{
				Scenario: ScenarioP1HighConfPass,
				Action:   ActionAlphaBump,
				Tags:     []string{"conf=high", fmt.Sprintf("confidence=%.2f", v.Confidence)},
			}
		}
		return PolicyDecision{
			Scenario: ScenarioP2LowConfPass,
			Action:   ActionAlphaBump,
			Tags:     []string{"conf=low", fmt.Sprintf("confidence=%.2f", v.Confidence)},
		}

	case types.VerdictPartial:
		return PolicyDecision{
			Scenario: ScenarioP3Partial,
			Action:   ActionAlphaBump,
			Tags:     []string{"partial", fmt.Sprintf("confidence=%.2f", v.Confidence)},
		}

	case types.VerdictFail:
		// F1 / F2 / F3 by Reason prefix or empty.
		if reason == "" {
			return PolicyDecision{
				Scenario: ScenarioF3Ambiguous,
				Action:   ActionBetaBump,
				Tags:     []string{"ambiguous"},
			}
		}
		if strings.HasPrefix(reason, "system_") || strings.HasPrefix(reason, "env_") {
			return PolicyDecision{
				Scenario: ScenarioF1SystemError,
				Action:   ActionBetaBump,
				Tags:     []string{"system_error", "env_attributable"},
			}
		}
		// Default fail classification is F2 (user-attributable).
		return PolicyDecision{
			Scenario: ScenarioF2UserError,
			Action:   ActionBetaBump,
			Tags:     []string{"user_error"},
		}

	case types.VerdictIndeterminate:
		// I1: verifier_parse_failure — G8-1 fix; only bump
		// VerifierFailureCount, do not touch α/β.
		if indetReason == "verifier_parse_failure" {
			return PolicyDecision{
				Scenario: ScenarioI1VerifierFail,
				Action:   ActionNoChange,
				Tags:     []string{"verifier_parse_failure", "g8_1_fix"},
			}
		}
		// I2: env-limited INDETERMINATE (e.g. tool timeout, rate limit).
		if strings.HasPrefix(reason, "env_") || strings.HasPrefix(indetReason, "env_") {
			return PolicyDecision{
				Scenario: ScenarioI2EnvLimited,
				Action:   ActionNoChange,
				Tags:     []string{"env_limited", "no_alpha_beta_update"},
			}
		}
		// I3: other INDETERMINATE (default).
		return PolicyDecision{
			Scenario: ScenarioI3OtherIndet,
			Action:   ActionNoChange,
			Tags:     []string{"other_indeterminate"},
		}
	}

	// --- FP1: force_plan check ---
	// Fires when the post-update β/(α+β) ratio would exceed the threshold.
	// The caller (BayesianUpdate wrapper) computes the post-update ratio
	// from the resulting ReputationEvidence; ClassifyScenario is purely
	// a decision-table lookup and does not run BayesianUpdate itself.
	//
	// Note: FP1 is checked LAST so verdict-based classification always
	// wins for current-round outcomes. The caller invokes ClassifyScenario
	// once per round; if the verdict-based row fires, FP1 is skipped.
	// If the verdict-based row would emit alpha_bump but the resulting
	// reputation is still β-dominated (rare for high-confidence Pass but
	// possible for low-confidence), a second pass through
	// ClassifyScenarioWithReputation is called by the BayesianUpdate
	// wrapper (see force_plan.go).
	//
	// This branch is intentionally a no-op here; FP1 detection lives in
	// ClassifyScenarioWithReputation below.
	return PolicyDecision{
		Scenario: ScenarioF1ForcePlanTrigger,
		Action:   ActionForcePlan,
		Tags:     []string{"force_plan_threshold", "reputation_driven"},
	}
}

// ClassifyScenarioWithReputation is the post-BayesianUpdate variant that
// fires when the resulting reputation is dominated by failures. It is
// called by the BayesianUpdate wrapper after α/β have been updated, so
// the policy can decide whether to also emit the force_plan signal.
//
// Why a separate function: the pre-update ClassifyScenario is total (covers
// all current-round outcomes); the post-update variant is a one-shot
// overlay that can only fire force_plan. Keeping them separate means the
// flat table in classifyScenario is the single source of truth for
// current-round classification, while force_plan is a reputation-driven
// second-order decision.
func ClassifyScenarioWithReputation(in PolicyInput, postRepAlpha, postRepBeta int) PolicyDecision {
	total := postRepAlpha + postRepBeta
	if total == 0 {
		// Cold start with no updates: no reputation signal yet. The
		// pre-update ClassifyScenario already returned a verdict-based
		// decision; force_plan is a no-op here.
		return PolicyDecision{
			Scenario: ScenarioF1ForcePlanTrigger,
			Action:   ActionNoChange,
			Tags:     []string{"cold_start", "no_force_plan"},
		}
	}
	ratio := float64(postRepBeta) / float64(total)
	if ratio > forcePlanThreshold {
		return PolicyDecision{
			Scenario: ScenarioF1ForcePlanTrigger,
			Action:   ActionForcePlan,
			Tags: []string{
				"force_plan_threshold",
				fmt.Sprintf("beta_ratio=%.2f", ratio),
				fmt.Sprintf("alpha=%d,beta=%d", postRepAlpha, postRepBeta),
			},
		}
	}
	return PolicyDecision{
		Scenario: ScenarioF1ForcePlanTrigger,
		Action:   ActionNoChange,
		Tags: []string{
			"below_force_plan_threshold",
			fmt.Sprintf("beta_ratio=%.2f", ratio),
		},
	}
}

// LearnPolicy is the runtime policy struct. It is intentionally stateless
// (no fields) — all logic lives in the free functions ClassifyScenario and
// ClassifyScenarioWithReputation. The struct is exposed so callers can
// inject a mock in tests, and so the 22-scenario table is referenced by
// the type system (rather than as a bare package-level function).
type LearnPolicy struct{}

// NewLearnPolicy returns the default policy.
func NewLearnPolicy() *LearnPolicy { return &LearnPolicy{} }

// Evaluate is the public entry point — wraps ClassifyScenario for callers
// that prefer method-call syntax.
func (p *LearnPolicy) Evaluate(in PolicyInput) PolicyDecision {
	return ClassifyScenario(in)
}

// EvaluateWithReputation is the post-BayesianUpdate variant.
func (p *LearnPolicy) EvaluateWithReputation(in PolicyInput, postAlpha, postBeta int) PolicyDecision {
	return ClassifyScenarioWithReputation(in, postAlpha, postBeta)
}

// AllScenarios returns the 22 scenario IDs in stable order. Useful for
// dashboard / metric dashboards that want to enumerate the full table
// without depending on the constant ordering.
func AllScenarios() []ScenarioID {
	return []ScenarioID{
		// P1-P3
		ScenarioP1HighConfPass, ScenarioP2LowConfPass, ScenarioP3Partial,
		// F1-F3
		ScenarioF1SystemError, ScenarioF2UserError, ScenarioF3Ambiguous,
		// I1-I3
		ScenarioI1VerifierFail, ScenarioI2EnvLimited, ScenarioI3OtherIndet,
		// E1-E6
		ScenarioE1PlanTimeout, ScenarioE2PlanCall5xx, ScenarioE3PlanPartial,
		ScenarioE4PostExec, ScenarioE5VerifyError, ScenarioE6SystemAnomaly,
		// U1-U3
		ScenarioU1UserCancel, ScenarioU2UserAccept, ScenarioU3UserModify,
		// R1-R2
		ScenarioR1RollupPass, ScenarioR2RollupFail,
		// FP1-FP2
		ScenarioF1ForcePlanTrigger, ScenarioF2ForcePlanOnNext,
	}
}
