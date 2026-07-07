// Package wavescheduler — DAGExecutor error sentinels (DM-20260707-001 PR-B).
//
// These sentinels live on the 72xx audit-code series that plan/dag_validator.go
// established (7200-7205). They use the internal/shared/errors.SentinelError
// type so the audit-log and metrics pipelines can grep ORCH_DAG_EXECUTOR_*
// without parse-fishing on inner error strings — consistent with the existing
// AGENTS.md mandate to use sharederrors.SentinelError for cross-package
// error reporting.
//
// Surgical scope: PR-B does NOT refactor the existing waveError style used by
// wavescheduler/errors.go. The package keeps its self-contained errWave for
// internal validation and adds this file only for the four executor sentinels
// that must cross the package boundary (audit + IM error contract).
package wavescheduler

import (
	"errors"
	"fmt"

	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// Sentinel errors (DM-20260707-001 PR-B Q9 — codex + cursor ACCEPT-WITH-CHANGE).
//
// Code allocation (see reviews/pr-b-consensus-packet.md §8 Q9 and the
// codex/cursor consensus files):
//
//	ORCH_DAG_EXECUTOR_NIL_DAG_7210
//	ORCH_DAG_EXECUTOR_NIL_SEGSET_7211
//	ORCH_DAG_EXECUTOR_MISSING_SEGMENT_7212
//	ORCH_DAG_EXECUTOR_EXECUTION_FAILED_7213
//
// 7214 was proposed by codex as a "workitem hint unsupported" sentinel, but
// the consensus (Q3) routes workitem hints to WorkerSubAgent + a metadata
// stamp instead of rejecting — so no 7214 is needed. See dag_executor.go
// convertWorkerHint for the routing table.
var (
	ErrDAGExecutorNilDAG           = errors.New("wavescheduler: DAGExecutor.RunPlanDAG called with nil PlanDAG")
	ErrDAGExecutorNilSegmentSet    = errors.New("wavescheduler: DAGExecutor.RunPlanDAG called with nil IntentSegmentSet")
	ErrDAGExecutorMissingSegment   = errors.New("wavescheduler: DAGExecutor conversion found PlanNode.SegmentID not in IntentSegmentSet")
	ErrDAGExecutionFailed          = errors.New("wavescheduler: DAGExecutor wave terminated with at least one child failure")
)

// NewDAGExecutorNilDAGError wraps ORCH_DAG_EXECUTOR_NIL_DAG_7210.
func NewDAGExecutorNilDAGError() *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_DAG_EXECUTOR_NIL_DAG_7210",
		"DAGExecutor.RunPlanDAG called with nil PlanDAG",
		ErrDAGExecutorNilDAG,
	)
}

// NewDAGExecutorNilSegmentSetError wraps ORCH_DAG_EXECUTOR_NIL_SEGSET_7211.
func NewDAGExecutorNilSegmentSetError() *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_DAG_EXECUTOR_NIL_SEGSET_7211",
		"DAGExecutor.RunPlanDAG called with nil IntentSegmentSet",
		ErrDAGExecutorNilSegmentSet,
	)
}

// NewDAGExecutorMissingSegmentError wraps ORCH_DAG_EXECUTOR_MISSING_SEGMENT_7212.
// The PlanNode references a SegmentID that is not present in the parent
// IntentSegmentSet — a cross-reference integrity failure surfaced at
// conversion time before any worker is dispatched.
func NewDAGExecutorMissingSegmentError(planNodeID, missingSegmentID string) *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_DAG_EXECUTOR_MISSING_SEGMENT_7212",
		fmt.Sprintf("DAGExecutor conversion: PlanNode %q references missing SegmentID %q (not in IntentSegmentSet)", planNodeID, missingSegmentID),
		fmt.Errorf("%w: planNodeID=%q segmentID=%q", ErrDAGExecutorMissingSegment, planNodeID, missingSegmentID),
	)
}

// NewDAGExecutionFailedError wraps ORCH_DAG_EXECUTOR_EXECUTION_FAILED_7213.
// Surfaced after a strict cancel-all abort: at least one child node reached
// StateFailed, the wave context was cancelled, and remaining pending nodes
// were explicitly marked StateCancelled before the channel was closed.
//
// failedTaskIDs lists the node IDs that reached StateFailed (may be empty
// when ctx cancel / reentry cancel triggered the abort with no failures).
func NewDAGExecutionFailedError(failedTaskIDs []string) *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_DAG_EXECUTOR_EXECUTION_FAILED_7213",
		fmt.Sprintf("DAGExecutor wave aborted (failed task IDs: %v)", failedTaskIDs),
		fmt.Errorf("%w: failedTaskIDs=%v", ErrDAGExecutionFailed, failedTaskIDs),
	)
}

// IsDAGExecutionFailedError reports whether err is (or wraps) the 7213
// execution-failed sentinel. Used by callers that want to distinguish
// abort-by-failure from ctx.Canceled.
func IsDAGExecutionFailedError(err error) bool {
	return err != nil && errors.Is(err, ErrDAGExecutionFailed)
}
