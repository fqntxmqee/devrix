// Package interfaces defines the D7 TaskContract dual contract — the unified
// down-link (TaskSpec) and up-link (TaskReport) types that propagate requests
// down the orchestration tree and feedback back up.
//
// # Design intent (devrix-d7-taskcontract-unification-pr-a, DM-20260629-007)
//
// The interfaces package is a Pure types package: it MUST NOT import any other
// D7 sub-package (workmodel, sessionorchestrator, mups/*, decisionplanning,
// escape, hardening, executionflow, d7-bootstrap). This invariant is what
// allows TaskSpec/TaskReport to flow across sub-package boundaries without
// creating an import cycle. PR-C will add a layout-guard enforcement check.
//
// # Type contracts
//
//   - TaskSpec (down-link): the 4+2 field contract that every Plan / Channel /
//     WorkItem creation point must produce. Pure value type, immutable via
//     With* builders (returns shallow copies).
//
//   - TaskReport (up-link): the 5+2 field contract that every Channel.Execute
//     exit point and every Learn-node entry point must produce / accept.
//     Pure value type, immutable, supports AppendDissent for top-N minority
//     capture (default top-3 truncation).
//
// # Relationship to v6.0.x
//
// During the v6.0.x → v7.0.0 transition the existing ChannelRequest and
// LearnRequest types embed TaskSpec / TaskReport as additive fields. The
// legacy fields stay intact for one minor version so existing call sites
// keep compiling. PR-B will fully migrate all call sites, PR-C removes the
// legacy fields.
//
// # Thread-safety
//
// TaskSpec and TaskReport are immutable. The With* builders always return a
// shallow copy of the receiver with the requested field updated, so callers
// can safely share a base value across goroutines while building their own
// derived values. Mutation is impossible by construction.
//
// # Errors
//
// This package defines ORCH_TASK_SPEC_GOAL_EMPTY,
// ORCH_TASK_SPEC_TRACE_ID_EMPTY, ORCH_TASK_REPORT_TRACE_ID_EMPTY,
// ORCH_DISSENT_REJECTION and ORCH_RESOURCE_INVALID. The sentinel errors live
// in errors.go; the wrap helpers in orchtypes/errors.go re-export them with
// the canonical 7xxx code range.
package interfaces