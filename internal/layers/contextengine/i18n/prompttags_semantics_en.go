package i18n

import "github.com/devrix/devrix/internal/shared/prompttags"

// semanticGlossaryEN maps SemanticCondition machine codes to en-US labels.
var semanticGlossaryEN = map[prompttags.SemanticCondition]string{
	prompttags.CondScopeUnclear:       "Use when scope/goal unclear or open questions remain",
	prompttags.CondStrongFactExists:   "Do not use when strong scope facts exist",
	prompttags.CondSignalBacked:       "Use for signal-backed statements with evidence",
	prompttags.CondNoSpeculation:      "Do not speculate without signal support (strength cap 0.85)",
	prompttags.CondStructuredSignal:   "Structured signal summary (below fact level)",
	prompttags.CondPreferUncertainty:  "Prefer uncertainty when classification is unclear",
	prompttags.CondMetricDelta:        "Expected vs observed delta (metric)",
	prompttags.CondNoBaseline:         "Do not use without a baseline",
	prompttags.CondStrengthAlignedKind: "0–1 aligned with kind; higher uncertainty → higher strength",
	prompttags.CondRequiredForObsUncertainty: "Required for obs_uncertainty; optional otherwise",
	prompttags.CondEvidenceExistingIDsOnly:   "Existing IDs only (wi_id, signal source); do not invent paths",
	prompttags.CondMaxProposalsThree:         "Maximum 3 proposals (Go enforced)",

	prompttags.CondExecutionModeDecisionTree: "IF uncertainty_mean≥0.45 OR remaining_children>1 needed → decompose; ELIF scope clear → single; ELIF parallel probes → parallel_probe (single blocked when U≥0.45, Go enforced)",
	prompttags.CondDeliverableContractExample: `Example: {"citation":"file_line","severity":"p0_p1","reject":["planning_meta"],"structure":"findings_json"}`,
	prompttags.CondChildSpecsDecomposeMax2:    "For decompose: title/directive_suffix/expected_return each; max 2 (Go enforced)",

	prompttags.CondRequiredWhenContract:       "control · Required when contract applicable · VerifyDeliverableContract",
	prompttags.CondRequiredWhenFindingsJSON:   "data · Required when structure=findings_json · findings_json_* verify",
	prompttags.CondOptionalResidualQuestions:  "data · Optional · residual uncertainty",
	prompttags.CondOptionalScopeUpdate:        "control · Optional update · scope monotonic",
	prompttags.CondOptionalConclusionProse:    "human prose · Optional · machine blocks before <conclusion>",

	prompttags.CondTaskDirective:            "Task directive (data)",
	prompttags.CondPriorParseRejectFeedback: "Prior-round parse/budget reject (control); fix output per code/field",
	prompttags.CondStructuredSignals:        "Structured signal lines (data)",
	prompttags.CondPriorMeanControl:         "Prior mean (control, not business content)",
	prompttags.CondScopeOpenQuestions:       "Scope open questions (data)",
	prompttags.CondIncrementalRound:         "Incremental round marker (control)",
	prompttags.CondScopeGoal:                "Scope goal statement (data)",
	prompttags.CondWorkItemIdentifier:       "WorkItem ID (control, identifier only)",
	prompttags.CondPriorObservationIDs:      "Prior-round observation IDs (control, incremental context)",
	prompttags.CondObservationSummary:       "Observation summary (data)",
	prompttags.CondUncertaintyMeanControl:   "Uncertainty mean (control); single forbidden when ≥0.45",
	prompttags.CondRemainingChildrenBudget:  "Remaining child budget (control)",
	prompttags.CondObservationIDs:           "Prior-round observation IDs (data, incremental context)",
	prompttags.CondDepthControl:             "Current work item depth (control)",
	prompttags.CondMaxDepthControl:          "Maximum depth allowed (control)",
	prompttags.CondExistingChildrenControl:  "Existing non-ephemeral child count (control)",
	prompttags.CondMaxChildrenControl:       "Maximum children allowed (control)",
	prompttags.CondDecomposeUsedToday:       "Decompose operations used in 24h (control)",
	prompttags.CondRemainingDaily:           "Remaining decompose headroom today (control)",
	prompttags.CondMaxDailyControl:          "Daily decompose limit (control)",
	prompttags.CondMaxItersControl:          "Per-WorkItem ReAct iteration cap (control)",
	prompttags.CondParentScopeInControl:     "Parent scope in-paths (control, subset constraint)",

	// ResolutionContract (DM-20260704-006, RC-1) — Plan user frame fields.
	prompttags.CondResolutionStrategies: "resolution_strategies (data): one per ObsUncertainty, binding obs_id + planned_tool + success_criterion; attach sub_worktree when sibling investigation is required",
	prompttags.CondResolutionClaims:     "resolution_claims (data, cross-round feedback): previous round's Execute-emitted per-obs_id answers + confidence + evidence",
}

// semanticNodeRoleEN maps node-role keys to one-line en-US descriptions.
var semanticNodeRoleEN = map[string]string{
	"observe.node_role":              "Six-node pipeline step 1 Observe: closed-set classifier; input=directive+signals, output=Obs* array; no tool execution and no task assessment.",
	"plan.node_role":                 "Six-node pipeline step 2 Plan: propose execution_mode and deliverable_contract.",
	"execute.node_role":              "Six-node pipeline step 3 Execute: ReAct + deliverable; Obs taxonomy done in Observe.",
	"execute.output.section.react":   "ReAct phase: tool calls + short text",
	"execute.output.section.final":   "Final reply: machine blocks (contract/findings) first, then <conclusion> summary",
}

const semanticPlaneGuideEN = "In the user frame below, [control] fields are orchestration budget/constraints — not business content to analyze; [data] holds task payload and signals."
