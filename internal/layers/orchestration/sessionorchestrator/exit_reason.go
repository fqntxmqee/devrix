package sessionorchestrator

// ExitReason captures *why* the turn loop stopped. Surfaced on the final
// `complete` EngineEvent's Metadata["exit_reason"] and on the persisted
// turn record so SDK consumers, dashboards, and tests can distinguish a
// healthy LLM finish from a forced exit.
//
// The taxonomy mirrors claude-code's query-loop terminal reasons
// (clawcode/src/query.ts + 16-reason catalogue in
// docs/agent/clawcode/01-tools-queryloop §4). Only the subset relevant
// to devrix's D7 orchestrator is enumerated; missing reasons (e.g.
// prompt_too_long, image_error) are still surfaced via D2 enforce
// before reaching D7, so D7 does not need its own enum values for them.
type ExitReason string

const (
	// ExitReasonNatural: LLM emitted no tool calls (end_turn / stop).
	ExitReasonNatural ExitReason = "natural"
	// ExitReasonMaxTurns: MaxTurns safety net triggered. Only set when
	// MaxTurns > 0; an unbounded turn (MaxTurns ≤ 0) cannot hit this.
	ExitReasonMaxTurns ExitReason = "max_turns"
	// ExitReasonAbortedUser: ctx cancelled (user interrupt / deadline).
	ExitReasonAbortedUser ExitReason = "aborted_user"
	// ExitReasonAbortedLLM: invokeStream returned a fatal error.
	ExitReasonAbortedLLM ExitReason = "aborted_llm"
	// ExitReasonAbortedTool: ExecuteRound returned a fatal error.
	ExitReasonAbortedTool ExitReason = "aborted_tool"
	// ExitReasonRepeatedTool: same (tool_name|input) signature appeared
	// ≥ repeatedToolThreshold times in the last repeatedToolLookback
	// turns. Indicates the LLM is stuck retrying the same action.
	ExitReasonRepeatedTool ExitReason = "repeated_tool"
	// ExitReasonToolFailure: ≥ consecutiveToolErrorThreshold consecutive
	// tool errors with the same error fingerprint. Indicates the LLM
	// cannot recover from a tool failure pattern.
	ExitReasonToolFailure ExitReason = "tool_failure"
	// ExitReasonTokenDiminishing: cumulative token usage crossed the
	// 90% budget threshold AND the last two per-turn deltas were both
	// below the diminishing delta floor. Mirrors clawcode's
	// checkTokenBudget "marginal utility" stop condition.
	ExitReasonTokenDiminishing ExitReason = "token_diminishing"

	// Phase 4 PR-D2 additions (DM-20260623-002). The 8 reasons above are
	// the deterministic-stops set; the 6 below extend the taxonomy with
	// Verify-derived stop conditions so downstream D5 dashboards and
	// Phase 5 Learn's ReputationEvidence can distinguish "verifier said
	// pass" from "verifier said partial" from "verifier abstained" etc.
	// See doc 45 §三 + doc 17/18 L1+L2 verifier.

	// ExitReasonPartialVerified: VerdictKind=Partial (some criteria met,
	// some missed — needs human review before downstream action).
	ExitReasonPartialVerified ExitReason = "partial_verified"
	// ExitReasonVerifierAbstain: VerdictKind=Indeterminate (verifier
	// could not reach a conclusion — parse failure / no consensus /
	// VerifyWithRetry exhausted 3 attempts). Requires human review.
	ExitReasonVerifierAbstain ExitReason = "verifier_abstain"
	// ExitReasonVerifierFail: VerdictKind=Fail (criteria violated — the
	// plan / artifact / evidence is rejected by the verifier).
	ExitReasonVerifierFail ExitReason = "verifier_fail"
	// ExitReasonSystemAnomaly: SystemAnomaly=true override (CatSystem
	// anomalies exceeded threshold). Forced UncertaintyCoord.Value=0.95
	// per Phase 4 PR-D4.
	ExitReasonSystemAnomaly ExitReason = "system_anomaly"
	// ExitReasonUnresolved: failure is recoverable on retry — surfaced
	// when Phase 3 Channel returns SideEffectInflight or Unknown and
	// Phase 4 Verifier cannot conclude. Distinct from VerifierFail
	// (definitive rejection) and VerifierAbstain (no signal).
	ExitReasonUnresolved ExitReason = "unresolved"
	// ExitReasonAbstain: explicit abstain (verifier decided NOT to
	// render judgement, e.g. out-of-scope plan). Distinct from
	// VerifierAbstain which is parser-level failure.
	ExitReasonAbstain ExitReason = "abstain"
)
