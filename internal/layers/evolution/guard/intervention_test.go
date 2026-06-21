package guard

import (
	"context"
	"errors"
	"testing"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/shared/types"
)

// newTestInterventionExecutor returns an executor pre-wired with a single
// mock agent and metrics. The mockAgent field is injected into mockAgentCtrl.
func newTestInterventionExecutor(t *testing.T, ag multiagent.Agent, tasks TaskController) (*InterventionExecutor, *orchMetrics) {
	t.Helper()
	m := &orchMetrics{}
	ctrl := &mockAgentCtrl{agents: map[string]multiagent.Agent{"test-session": ag}}
	exec := NewInterventionExecutor(ctrl, tasks, &mockAgentFactory{}).WithMetrics(m)
	return exec, m
}

func TestInterventionExecutor_WaitFailure_RecordsMetric(t *testing.T) {
	waitErr := errors.New("agent wait timeout")
	ag := &mockAgent{waitErr: waitErr}
	exec, metrics := newTestInterventionExecutor(t, ag, &mockTaskCtrl{})

	iv := Intervention{Action: "reroute", Reason: "test reroute"}
	err := exec.terminateAndReroute(context.Background(), session(), iv)

	if err == nil {
		t.Fatal("expected non-nil error from Wait failure")
	}
	if !errors.Is(err, waitErr) {
		t.Errorf("expected errors.Is(err, waitErr) true, got %v", err)
	}
	if got := metrics.SnapshotWaitFailed(); got != 1 {
		t.Errorf("WaitFailed counter = %d, want 1", got)
	}
	if got := metrics.SnapshotTaskFailFailed(); got != 0 {
		t.Errorf("TaskFailFailed counter = %d, want 0", got)
	}
}

func TestInterventionExecutor_TaskFailFailure_RecordsMetric(t *testing.T) {
	failErr := errors.New("task fail denied")
	ag := &mockAgent{} // waitErr == nil
	tasks := &mockTaskCtrl{failErr: failErr}
	exec, metrics := newTestInterventionExecutor(t, ag, tasks)

	iv := Intervention{Action: "reroute", Reason: "milestone fail", MilestoneFail: true, FailReason: "task-1"}
	err := exec.terminateAndReroute(context.Background(), session(), iv)

	if err == nil {
		t.Fatal("expected non-nil error from tasks.Fail failure")
	}
	if !errors.Is(err, failErr) {
		t.Errorf("expected errors.Is(err, failErr) true, got %v", err)
	}
	if got := metrics.SnapshotTaskFailFailed(); got != 1 {
		t.Errorf("TaskFailFailed counter = %d, want 1", got)
	}
	if got := metrics.SnapshotWaitFailed(); got != 0 {
		t.Errorf("WaitFailed counter = %d, want 0", got)
	}
}

func TestInterventionExecutor_TerminateFailure_ReturnsPartialErr(t *testing.T) {
	termErr := errors.New("terminate denied")
	ag := &mockAgent{terminateErr: termErr} // waitErr == nil
	exec, metrics := newTestInterventionExecutor(t, ag, &mockTaskCtrl{})

	iv := Intervention{Action: "reroute", Reason: "test reroute"}
	err := exec.terminateAndReroute(context.Background(), session(), iv)

	if err == nil {
		t.Fatal("expected non-nil error from Terminate failure")
	}
	if !errors.Is(err, termErr) {
		t.Errorf("expected errors.Is(err, termErr) true, got %v", err)
	}
	// Terminate failure must NOT bump wait counter.
	if got := metrics.SnapshotWaitFailed(); got != 0 {
		t.Errorf("WaitFailed counter = %d, want 0 (Terminate-only failure)", got)
	}
}

func TestInterventionExecutor_AllSuccess_ReturnsNil(t *testing.T) {
	ag := &mockAgent{}
	exec, metrics := newTestInterventionExecutor(t, ag, &mockTaskCtrl{})

	iv := Intervention{Action: "reroute", Reason: "test reroute"}
	err := exec.terminateAndReroute(context.Background(), session(), iv)

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if got := metrics.SnapshotWaitFailed(); got != 0 {
		t.Errorf("WaitFailed counter = %d, want 0", got)
	}
	if got := metrics.SnapshotTaskFailFailed(); got != 0 {
		t.Errorf("TaskFailFailed counter = %d, want 0", got)
	}
}

func TestInterventionExecutor_NilMetrics_NilSafe(t *testing.T) {
	waitErr := errors.New("wait timeout")
	ag := &mockAgent{waitErr: waitErr}
	// No WithMetrics call → metrics == nil
	ctrl := &mockAgentCtrl{agents: map[string]multiagent.Agent{"test-session": ag}}
	exec := NewInterventionExecutor(ctrl, &mockTaskCtrl{}, &mockAgentFactory{})

	iv := Intervention{Action: "reroute", Reason: "test reroute"}
	err := exec.terminateAndReroute(context.Background(), session(), iv)

	// Wait failure must still surface as error even with nil metrics.
	if err == nil {
		t.Fatal("expected non-nil error (nil metrics must not swallow)")
	}
	if !errors.Is(err, waitErr) {
		t.Errorf("expected errors.Is(err, waitErr) true, got %v", err)
	}
}

func TestInterventionExecutor_BothWaitAndTaskFailFail_AggregateBoth(t *testing.T) {
	waitErr := errors.New("wait timeout")
	failErr := errors.New("task fail denied")
	ag := &mockAgent{waitErr: waitErr}
	tasks := &mockTaskCtrl{failErr: failErr}
	exec, metrics := newTestInterventionExecutor(t, ag, tasks)

	iv := Intervention{
		Action: "reroute", Reason: "test reroute",
		MilestoneFail: true, FailReason: "task-1",
	}
	err := exec.terminateAndReroute(context.Background(), session(), iv)

	if err == nil {
		t.Fatal("expected non-nil aggregated error")
	}
	if !errors.Is(err, waitErr) {
		t.Errorf("expected errors.Is(err, waitErr) true, got %v", err)
	}
	if !errors.Is(err, failErr) {
		t.Errorf("expected errors.Is(err, failErr) true, got %v", err)
	}
	if got := metrics.SnapshotWaitFailed(); got != 1 {
		t.Errorf("WaitFailed counter = %d, want 1", got)
	}
	if got := metrics.SnapshotTaskFailFailed(); got != 1 {
		t.Errorf("TaskFailFailed counter = %d, want 1", got)
	}
}

func TestInterventionExecutor_WithMetrics_Chainable(t *testing.T) {
	exec := NewInterventionExecutor(&mockAgentCtrl{}, &mockTaskCtrl{}, &mockAgentFactory{})
	m := &orchMetrics{}
	got := exec.WithMetrics(m)
	if got != exec {
		t.Errorf("WithMetrics should return same pointer for chaining, got %p want %p", got, exec)
	}
	if exec.metrics != m {
		t.Errorf("metrics not set, got %v want %v", exec.metrics, m)
	}
}

// Compile-time guard: ensure mockAgent implements multiagent.Agent
// (multiagent.Agent is an interface; mockAgent satisfies it via duck-typing).
var _ multiagent.Agent = (*mockAgent)(nil)

// Sanity: session() helper from validator_test.go is still usable.
var _ = func() *types.Session { return session() }