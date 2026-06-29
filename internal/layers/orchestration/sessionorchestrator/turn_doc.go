// Package sessionorchestrator hosts the D7 v6.0 6-Scenario orchestration surface
// after the DM-20260626-001 14→6 simplification. The package boundary bundles
// what used to be a separate turn/ subpackage (merged by DM-20260626-004) and
// remains the single source of truth for turn execution.
//
// v6.0 6S + 1 cross-cutting layout (D7-S1..S6 + D7-S0 meta):
//
//	S1 (Intent)        — orchestrator intent classification + sub-intent dispatch
//	S2 (Session)       — per-session state, multi-turn context, resume/force-exit
//	S3 (Decide)        — decide-next-action with WorkTree downward propagation
//	S4 (Verify)        — promote to executionflow/verify, drives ExitReason
//	S5 (Plan / Wave)   — wave / task planning + scheduling
//	S6 (Tool)          — D4 delegate canonical tools + D2 fallback runner
//	S0 (Meta, cross)   — DSAFT governance, registry sync, value-flow rename,
//	                      span coverage, archive
//
// Current state (v2.6.0, DM-20260629-001):
//   - TurnOrchestrator is the only turn-loop owner (replaces legacy D2-S16).
//   - D7 calls D3 directly for LLM inference; D2 is a Context Follower via
//     ContextPreparer, ToolRoundExecutor, SessionPersister primitives.
//   - ingress routes via ProcessMessage with loop_first only (rule_orchestrate
//     retired in v6.0.0, FastPath retired in PR #239).
//   - WorkTree governance: typed RollupReport struct, deterministic
//     sessionRootGoal, 5 ReevaluateParentAfterChild call sites are all
//     migrated (see design.md §2.4).
package sessionorchestrator
