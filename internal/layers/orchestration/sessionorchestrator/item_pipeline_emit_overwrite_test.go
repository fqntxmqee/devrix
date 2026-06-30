package sessionorchestrator

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// TestItemPipelineRunner_OverwritesExecutorEmit_PerRun verifies the
// architectural fix for DM-20260628-002 (multi-turn closed-channel panic).
//
// Without the fix, r.Run() only set exec.Emit when it was nil. The first
// turn set exec.Emit to emitFn_1 (capturing out_1); subsequent turns
// silently kept emitFn_1 even though r.Emit had been overwritten to
// emitFn_2 by session_turn_loop.go:53. Once turn 1's `out` was closed via
// defer close(out), any LLM stream chunk on a later turn caused
// `send on closed channel` panic.
//
// The fix unconditionally writes r.Emit to exec.Emit so each Run() picks
// up the freshest emitFn (which captures the current turn's `out`).
//
// The test directly drives two consecutive Run() calls with two distinct
// r.Emit closures (and two distinct WorkItems — the first becomes locked
// after Run completes) and asserts exec.Emit is the SECOND closure after
// the second Run().
func TestItemPipelineRunner_OverwritesExecutorEmit_PerRun(t *testing.T) {
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

	// After turn 1, exec.Emit must point at the emitFn1 closure. Invoke
	// through exec.Emit directly to verify identity (we cannot compare
	// function pointers reliably across closures, so we count calls).
	exec.Emit(&contracts.EngineEvent{Type: "synthetic-1"})
	if emitFn1Calls != 1 {
		t.Fatalf("turn 1: exec.Emit not pointing at emitFn1 (emitFn1Calls=%d)", emitFn1Calls)
	}
	if emitFn2Calls != 0 {
		t.Fatalf("turn 1: exec.Emit unexpectedly invoked emitFn2 (emitFn2Calls=%d)", emitFn2Calls)
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

	// After turn 2, exec.Emit must point at the emitFn2 closure.
	exec.Emit(&contracts.EngineEvent{Type: "synthetic-2"})
	if emitFn2Calls != 1 {
		t.Fatalf("turn 2: exec.Emit NOT overwritten to emitFn2 (emitFn2Calls=%d) — architectural fix missing", emitFn2Calls)
	}
	if emitFn1Calls != 1 {
		t.Fatalf("turn 2: exec.Emit unexpectedly points at emitFn1 (emitFn1Calls=%d, want %d)", emitFn1Calls, 1)
	}
}
