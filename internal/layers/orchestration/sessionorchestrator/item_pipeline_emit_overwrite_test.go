package sessionorchestrator

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// TestItemPipelineRunner_DoesNotPersistExecutorEmit_PerRun verifies the
// per-invocation emit fix for DM-20260630-013.
//
// Without the fix, r.Run() only set exec.Emit when it was nil. The first
// turn set exec.Emit to emitFn_1 (capturing out_1); subsequent turns
// silently kept emitFn_1 even though r.Emit had been overwritten to
// emitFn_2 by session_turn_loop.go:53. Once turn 1's `out` was closed via
// defer close(out), any LLM stream chunk on a later turn caused
// `send on closed channel` panic.
//
// The secure fix must NOT write per-run state onto the shared
// DefaultWorkItemExecutor at all; the emit closure travels in
// WorkItemExecContext for the single ExecuteWorkItem invocation.
func TestItemPipelineRunner_DoesNotPersistExecutorEmit_PerRun(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)
	exec := NewWorkItemExecutor(&scriptedLLM{}, nil, nil)
	runner.Executor = exec

	sessionID := "sess-emit-overwrite"
	goal1, err := tm.EnsureGoal(sessionID, "first task")
	if err != nil {
		t.Fatalf("EnsureGoal: %v", err)
	}
	_ = tm.Tree().SetUncertainty(sessionID, goal1.ID, 0.2)
	goal1, _ = tm.GetWorkItem(sessionID, goal1.ID)

	emitFn1Calls := 0
	emitFn1 := func(*contracts.EngineEvent) { emitFn1Calls++ }
	emitFn2Calls := 0
	emitFn2 := func(*contracts.EngineEvent) { emitFn2Calls++ }

	// Turn 1: install emitFn_1 then Run().
	runner.Emit = emitFn1
	if _, err := runner.Run(context.Background(), sessionID, goal1, "", ItemPipelineRunOpts{}); err != nil {
		t.Fatalf("Run turn 1: %v", err)
	}

	if exec.Emit != nil {
		t.Fatal("turn 1: exec.Emit must not retain per-run emit closure")
	}

	// Turn 2: a fresh goal so Run() doesn't bail on a locked historical
	// item; install emitFn_2; Run() must overwrite exec.Emit.
	goal2, err := tm.EnsureGoal(sessionID, "second task")
	if err != nil {
		t.Fatalf("EnsureGoal turn 2: %v", err)
	}
	_ = tm.Tree().SetUncertainty(sessionID, goal2.ID, 0.2)
	goal2, _ = tm.GetWorkItem(sessionID, goal2.ID)

	runner.Emit = emitFn2
	if _, err := runner.Run(context.Background(), sessionID, goal2, "", ItemPipelineRunOpts{}); err != nil {
		t.Fatalf("Run turn 2: %v", err)
	}

	if exec.Emit != nil {
		t.Fatal("turn 2: exec.Emit must not retain per-run emit closure")
	}
	if emitFn1Calls != 0 || emitFn2Calls != 0 {
		t.Fatalf("synthetic emit closures should not be retained on executor: fn1=%d fn2=%d", emitFn1Calls, emitFn2Calls)
	}
}
