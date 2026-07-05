package prompttags

// SemanticCondition is a locale-neutral machine code for when/when-not guidance.
// Glossary text lives in D2 i18n; registry SoT is prompttags.
type SemanticCondition string

const (
	CondApplies SemanticCondition = "applies"

	// Observe obs_* kinds
	CondScopeUnclear       SemanticCondition = "scope_unclear"
	CondStrongFactExists   SemanticCondition = "strong_fact_exists"
	CondSignalBacked       SemanticCondition = "signal_backed"
	CondNoSpeculation      SemanticCondition = "no_speculation"
	CondStructuredSignal   SemanticCondition = "structured_signal"
	CondPreferUncertainty  SemanticCondition = "prefer_uncertainty"
	CondMetricDelta        SemanticCondition = "metric_delta"
	CondNoBaseline         SemanticCondition = "no_baseline"

	// Observe fields / rules
	CondStrengthAlignedKind      SemanticCondition = "strength_aligned_kind"
	CondRequiredForObsUncertainty SemanticCondition = "required_for_obs_uncertainty"
	CondEvidenceExistingIDsOnly  SemanticCondition = "evidence_existing_ids_only"
	CondMaxProposalsThree        SemanticCondition = "max_proposals_three"

	// Plan output
	CondExecutionModeDecisionTree SemanticCondition = "execution_mode_decision_tree"
	CondDeliverableContractExample SemanticCondition = "deliverable_contract_example"
	CondChildSpecsDecomposeMax2  SemanticCondition = "child_specs_decompose_max2"

	// Execute output
	CondRequiredWhenContract    SemanticCondition = "required_when_contract"
	CondRequiredWhenFindingsJSON SemanticCondition = "required_when_findings_json"
	CondOptionalResidualQuestions SemanticCondition = "optional_residual_questions"
	CondOptionalScopeUpdate     SemanticCondition = "optional_scope_update"
	CondOptionalConclusionProse SemanticCondition = "optional_conclusion_prose"

	// Lineframe input (shared codes)
	CondTaskDirective            SemanticCondition = "task_directive"
	CondPriorParseRejectFeedback SemanticCondition = "prior_parse_reject_feedback"
	CondStructuredSignals        SemanticCondition = "structured_signals"
	CondPriorMeanControl         SemanticCondition = "prior_mean_control"
	CondScopeOpenQuestions       SemanticCondition = "scope_open_questions"
	CondIncrementalRound         SemanticCondition = "incremental_round"
	CondScopeGoal                SemanticCondition = "scope_goal"
	CondWorkItemIdentifier       SemanticCondition = "work_item_identifier"
	CondPriorObservationIDs      SemanticCondition = "prior_observation_ids"
	CondObservationSummary       SemanticCondition = "observation_summary"
	CondUncertaintyMeanControl   SemanticCondition = "uncertainty_mean_control"

	// DM-20260705-010 (devrix-d7-mups-frame-delta-closure) Phase 2 T9:
	// Observe→Plan frame delta inject input semantics.
	CondPriorArtifactSummary     SemanticCondition = "prior_artifact_summary"
	CondKnownGaps                SemanticCondition = "known_gaps"
	CondRemainingChildrenBudget  SemanticCondition = "remaining_children_budget"
	CondObservationIDs           SemanticCondition = "observation_ids"
	CondDepthControl             SemanticCondition = "depth_control"
	CondMaxDepthControl          SemanticCondition = "max_depth_control"
	CondExistingChildrenControl  SemanticCondition = "existing_children_control"
	CondMaxChildrenControl       SemanticCondition = "max_children_control"
	CondDecomposeUsedToday       SemanticCondition = "decompose_used_today"
	CondRemainingDaily           SemanticCondition = "remaining_daily"
	CondMaxDailyControl          SemanticCondition = "max_daily_control"
	CondMaxItersControl          SemanticCondition = "max_iters_control"
	CondParentScopeInControl     SemanticCondition = "parent_scope_in_control"

	// ResolutionContract (DM-20260704-006) — Plan user frame fields.
	// CondResolutionStrategies: when ObsUncertainty exists, emit one
	// resolution_strategy per ObsID with planned_tool + success_criterion;
	// attach sub_worktree when a sibling WorkItem is required.
	CondResolutionStrategies SemanticCondition = "resolution_strategies"
	// CondResolutionClaims: cross-round feedback; Execute-emitted
	// claims from the previous round so the LLM can refine strategies.
	CondResolutionClaims SemanticCondition = "resolution_claims"
)
