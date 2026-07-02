package execute

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/shared/types"
)

// -----------------------------------------------------------------------------
// Test fixtures
// -----------------------------------------------------------------------------

// fakeRunner is a deterministic ToolRunner for tests. It dispatches to
// per-tool handlers (registered via OnInvoke) and records every call so
// rollback / parallelism / priority tests can assert behavior.
type fakeRunner struct {
	mu       sync.Mutex
	handlers map[string]func(req ToolRequest) (ToolResult, error)
	calls    []ToolRequest
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{handlers: make(map[string]func(req ToolRequest) (ToolResult, error))}
}

func (f *fakeRunner) OnInvoke(tool string, h func(req ToolRequest) (ToolResult, error)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.handlers[tool] = h
}

func (f *fakeRunner) Invoke(ctx context.Context, req ToolRequest) (ToolResult, error) {
	f.mu.Lock()
	f.calls = append(f.calls, req)
	h, ok := f.handlers[req.ToolName]
	f.mu.Unlock()
	if !ok {
		return ToolResult{ToolName: req.ToolName, ExitCode: 1, Output: "no handler"},
			fmt.Errorf("fake_runner: no handler for %s", req.ToolName)
	}
	now := time.Now()
	res, err := h(req)
	if res.StartedAt.IsZero() {
		res.StartedAt = now
	}
	if res.CompletedAt.IsZero() {
		res.CompletedAt = now.Add(10 * time.Millisecond)
	}
	return res, err
}

func (f *fakeRunner) CallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// validPlan constructs a Plan with the given Kind + Steps. It does NOT
// call Validate (caller's responsibility if needed).
func validPlan(kind plan.PlanKind, sessionID string, steps ...plan.Step) *plan.Plan {
	if steps == nil {
		steps = []plan.Step{}
	}
	planVal := plan.NewPlan("plan_test_"+kind.String(), sessionID, kind,
		[]string{"obs_1"}, steps, 0.85).
		WithBlastRadius(plan.BlastRadius{
			FileCount: 1, APICallCount: 1, TokenCost: 100,
			PersistScope: plan.PersistSession,
		}).
		WithAnomaliesCount(0)
	return &planVal
}

func validStep(id, tool, idemp string) plan.Step {
	return plan.Step{
		ID:             id,
		Directive:      "do " + tool,
		ToolName:       tool,
		ToolArgs:       map[string]any{"path": "/tmp/" + id},
		IdempotencyKey: idemp,
		EstimatedTokens: 100,
	}
}

// okResult is a canned successful tool result.
func okResult(tool string) ToolResult {
	return ToolResult{ToolName: tool, ExitCode: 0, Output: tool + " done"}
}

// -----------------------------------------------------------------------------
// D7-S9-A26-T01: PlanChannel interface + ChannelRegistry
// -----------------------------------------------------------------------------

// TestChannelRegistry_Register_4Kinds covers the canonical 1:1 binding
// between PlanKind and Channel. Each of the 4 production Channels must
// register without conflict.
func TestChannelRegistry_Register_4Kinds(t *testing.T) {
	runner := newFakeRunner()
	reg := NewChannelRegistry()
	for _, c := range []Channel{
		mustCommit(t, runner),
		mustProtocol(t, runner),
		mustScenario(t, runner),
		mustExploration(t, runner),
	} {
		if err := reg.Register(c); err != nil {
			t.Fatalf("Register(%s): %v", c.Name(), err)
		}
	}
	if reg.Len() != 4 {
		t.Errorf("registry Len()=%d, want 4", reg.Len())
	}
	for _, k := range []plan.PlanKind{
		plan.CommitmentPlan, plan.ProtocolPlan, plan.ScenarioPlan, plan.ExplorationPlan,
	} {
		c, err := reg.Get(k)
		if err != nil {
			t.Errorf("Get(%s): %v", k, err)
			continue
		}
		if !c.Supports(k) {
			t.Errorf("registered channel %s does not support %s", c.Name(), k)
		}
	}
}

// TestChannelRegistry_Get_NotFound covers the not-found error path with
// the wired-up sentinel (EXEC_CHANNEL_9001).
func TestChannelRegistry_Get_NotFound(t *testing.T) {
	reg := NewChannelRegistry()
	_, err := reg.Get(plan.CommitmentPlan)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrChannelNotFound) {
		t.Errorf("expected ErrChannelNotFound, got %v", err)
	}
}

// TestChannelRegistry_Register_DuplicateConflict ensures two Channels
// claiming the same PlanKind trigger a wiring-conflict error.
func TestChannelRegistry_Register_DuplicateConflict(t *testing.T) {
	runner := newFakeRunner()
	reg := NewChannelRegistry()
	// Force a duplicate by hand-crafting a second Channel for CommitmentPlan.
	dup := &nameChannel{name: "dup", supports: func(k plan.PlanKind) bool { return k == plan.CommitmentPlan }}
	if err := reg.Register(mustCommit(t, runner)); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	err := reg.Register(dup)
	if err == nil {
		t.Fatal("expected duplicate-registration error")
	}
	if !errors.Is(err, ErrChannelUnsupported) {
		t.Errorf("expected ErrChannelUnsupported, got %v", err)
	}
}

// nameChannel is a hand-rolled Channel for the duplicate-registration test
// (avoiding the constructor's nil-runner check).
type nameChannel struct {
	name     string
	supports func(plan.PlanKind) bool
}

func (n *nameChannel) Name() string { return n.name }
func (n *nameChannel) Supports(k plan.PlanKind) bool {
	return n.supports != nil && n.supports(k)
}
func (n *nameChannel) Execute(ctx context.Context, p *plan.Plan, req ChannelRequest) (*wavescheduler.Artifact, error) {
	return &wavescheduler.Artifact{Kind: types.ArtifactStateChangeCert}, nil
}

// TestChannelRouter_Route_4Kinds verifies the router dispatches the right
// Channel based on Plan.Kind and returns the expected ArtifactKind.
func TestChannelRouter_Route_4Kinds(t *testing.T) {
	runner := newFakeRunner()
	registry := registerAll(t, runner)
	router := NewChannelRouter(registry)

	cases := []struct {
		name string
		kind plan.PlanKind
		step plan.Step
		want types.ArtifactKind
	}{
		{"commitment", plan.CommitmentPlan, validStep("s1", "deploy", "idem-c"), types.ArtifactStateChangeCert},
		{"protocol", plan.ProtocolPlan, validStep("s1", "login", "idem-p1"), types.ArtifactResponseRecord},
		{"scenario", plan.ScenarioPlan, validStep("s1", "probe", "idem-s"), types.ArtifactProbeReport},
		{"exploration", plan.ExplorationPlan, validStep("s1", "explore", "idem-e"), types.ArtifactExperimentData},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner.OnInvoke(tc.step.ToolName, func(req ToolRequest) (ToolResult, error) {
				return okResult(req.ToolName), nil
			})
			p := validPlan(tc.kind, "sess_1", tc.step)
			art, err := router.Route(context.Background(), p, ChannelRequest{SessionID: "sess_1"})
			if err != nil {
				t.Fatalf("Route: %v", err)
			}
			if art.Kind != tc.want {
				t.Errorf("Kind=%v, want %v", art.Kind, tc.want)
			}
			if art.SourcePlanID != p.ID {
				t.Errorf("SourcePlanID=%q, want %q", art.SourcePlanID, p.ID)
			}
			if art.SessionID != "sess_1" {
				t.Errorf("SessionID=%q, want sess_1", art.SessionID)
			}
		})
	}
}

// TestChannelRouter_Route_NilPlan covers the defensive nil-Plan path.
func TestChannelRouter_Route_NilPlan(t *testing.T) {
	router := NewChannelRouter(NewChannelRegistry())
	_, err := router.Route(context.Background(), nil, ChannelRequest{})
	if !errors.Is(err, ErrChannelPlanNil) {
		t.Errorf("expected ErrChannelPlanNil, got %v", err)
	}
}

// TestChannelRouter_Route_UnknownPlanKind covers routing an unknown kind
// (e.g. KindUnset or a value outside the 4-class enum).
func TestChannelRouter_Route_UnknownPlanKind(t *testing.T) {
	router := NewChannelRouter(NewChannelRegistry())
	p := validPlan(plan.CommitmentPlan, "sess_1", validStep("s1", "deploy", "idem-c"))
	p.Kind = plan.KindUnset
	_, err := router.Route(context.Background(), p, ChannelRequest{})
	if err == nil {
		t.Fatal("expected error for KindUnset, got nil")
	}
	if !errors.Is(err, ErrChannelUnsupported) {
		t.Errorf("expected ErrChannelUnsupported, got %v", err)
	}
}

// -----------------------------------------------------------------------------
// D7-S9-A26-T02: CommitChannel
// -----------------------------------------------------------------------------

// TestCommitChannel_CommitmentPlan_OK is the golden-path: single Step
// succeeds → ArtifactStateChangeCert with SideEffectCommitted.
func TestCommitChannel_CommitmentPlan_OK(t *testing.T) {
	runner := newFakeRunner()
	runner.OnInvoke("deploy", func(req ToolRequest) (ToolResult, error) {
		return okResult("deploy"), nil
	})
	ch, err := NewCommitChannel(runner, CommitChannelConfig{})
	if err != nil {
		t.Fatalf("NewCommitChannel: %v", err)
	}
	p := validPlan(plan.CommitmentPlan, "sess_1", validStep("s1", "deploy", "idem-c"))
	art, err := ch.Execute(context.Background(), p, ChannelRequest{SessionID: "sess_1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if art.Kind != types.ArtifactStateChangeCert {
		t.Errorf("Kind=%v, want ArtifactStateChangeCert", art.Kind)
	}
	if art.SideEffectStatus != types.SideEffectCommitted {
		t.Errorf("SideEffectStatus=%v, want SideEffectCommitted", art.SideEffectStatus)
	}
	if art.ExitCode != 0 {
		t.Errorf("ExitCode=%d, want 0", art.ExitCode)
	}
	if art.SourcePlanID != p.ID {
		t.Errorf("SourcePlanID=%q, want %q", art.SourcePlanID, p.ID)
	}
	if art.SideEffectDetail == nil || art.SideEffectDetail.IdempotencyKey != "idem-c" {
		t.Errorf("SideEffectDetail.IdempotencyKey=%v, want idem-c", art.SideEffectDetail)
	}
}

// TestCommitChannel_OtherPlan_NotSupported checks the Supports() guard.
func TestCommitChannel_OtherPlan_NotSupported(t *testing.T) {
	ch, _ := NewCommitChannel(newFakeRunner(), CommitChannelConfig{})
	for _, k := range []plan.PlanKind{plan.ProtocolPlan, plan.ScenarioPlan, plan.ExplorationPlan} {
		if ch.Supports(k) {
			t.Errorf("CommitChannel should NOT support %s", k)
		}
	}
}

// TestCommitChannel_SingleStep_ProducesStateChangeCert covers the step-count
// invariant and IdempotencyKey requirement.
func TestCommitChannel_SingleStep_ProducesStateChangeCert(t *testing.T) {
	runner := newFakeRunner()
	runner.OnInvoke("noop", func(req ToolRequest) (ToolResult, error) {
		return okResult("noop"), nil
	})
	ch, _ := NewCommitChannel(runner, CommitChannelConfig{})

	// Multi-step plan must fail fast.
	p2 := validPlan(plan.CommitmentPlan, "sess_1",
		validStep("s1", "noop", "i1"),
		validStep("s2", "noop", "i2"),
	)
	if _, err := ch.Execute(context.Background(), p2, ChannelRequest{SessionID: "sess_1"}); err == nil {
		t.Error("expected error for 2-step commitment plan")
	} else if !errors.Is(err, ErrChannelStepCountMismatch) {
		t.Errorf("expected ErrChannelStepCountMismatch, got %v", err)
	}

	// Step without IdempotencyKey must fail fast.
	pNoIdem := validPlan(plan.CommitmentPlan, "sess_1", plan.Step{
		ID: "s1", Directive: "x", ToolName: "noop",
	})
	if _, err := ch.Execute(context.Background(), pNoIdem, ChannelRequest{SessionID: "sess_1"}); err == nil {
		t.Error("expected error for missing IdempotencyKey")
	}
}

// TestCommitChannel_Timeout_InflightSideEffect covers the timeout path.
func TestCommitChannel_Timeout_InflightSideEffect(t *testing.T) {
	runner := newFakeRunner()
	runner.OnInvoke("slow", func(req ToolRequest) (ToolResult, error) {
		return ToolResult{ToolName: "slow"}, context.DeadlineExceeded
	})
	ch, _ := NewCommitChannel(runner, CommitChannelConfig{Timeout: 50 * time.Millisecond})
	p := validPlan(plan.CommitmentPlan, "sess_1", validStep("s1", "slow", "i1"))
	art, err := ch.Execute(context.Background(), p, ChannelRequest{SessionID: "sess_1"})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if art.SideEffectStatus != types.SideEffectInflight {
		t.Errorf("SideEffectStatus=%v, want SideEffectInflight", art.SideEffectStatus)
	}
}

// TestCommitChannel_NilRunner covers the constructor's defensive check.
func TestCommitChannel_NilRunner(t *testing.T) {
	_, err := NewCommitChannel(nil, CommitChannelConfig{})
	if err == nil {
		t.Fatal("expected error for nil runner")
	}
	if !errors.Is(err, ErrChannelToolRunnerNil) {
		t.Errorf("expected ErrChannelToolRunnerNil, got %v", err)
	}
}

// -----------------------------------------------------------------------------
// D7-S9-A26-T03: ProtocolChannel
// -----------------------------------------------------------------------------

// TestProtocolChannel_AllStepsSuccess_ResponseRecord is the golden-path:
// all Steps succeed → ArtifactResponseRecord with SideEffectCommitted.
func TestProtocolChannel_AllStepsSuccess_ResponseRecord(t *testing.T) {
	runner := newFakeRunner()
	runner.OnInvoke("login", func(req ToolRequest) (ToolResult, error) { return okResult("login"), nil })
	runner.OnInvoke("fetch", func(req ToolRequest) (ToolResult, error) { return okResult("fetch"), nil })
	runner.OnInvoke("parse", func(req ToolRequest) (ToolResult, error) { return okResult("parse"), nil })

	ch, _ := NewProtocolChannel(runner, ProtocolChannelConfig{})
	p := validPlan(plan.ProtocolPlan, "sess_1",
		validStep("s1", "login", "i1"),
		validStep("s2", "fetch", "i2"),
		validStep("s3", "parse", "i3"),
	)
	art, err := ch.Execute(context.Background(), p, ChannelRequest{SessionID: "sess_1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if art.Kind != types.ArtifactResponseRecord {
		t.Errorf("Kind=%v, want ArtifactResponseRecord", art.Kind)
	}
	if art.SideEffectStatus != types.SideEffectCommitted {
		t.Errorf("SideEffectStatus=%v, want SideEffectCommitted", art.SideEffectStatus)
	}
	if !strings.Contains(art.Summary, "step_0") || !strings.Contains(art.Summary, "step_1") || !strings.Contains(art.Summary, "step_2") {
		t.Errorf("Summary missing step entries: %q", art.Summary)
	}
}

// TestProtocolChannel_Step2_Failed_RollbackStep1 covers the rollback path:
// step 2 fails → step 1 is rolled back, Artifact reports RolledBack.
func TestProtocolChannel_Step2_Failed_RollbackStep1(t *testing.T) {
	runner := newFakeRunner()
	runner.OnInvoke("login", func(req ToolRequest) (ToolResult, error) {
		// Verify the rollback hint is present.
		if req.Args["__rollback"] == true {
			return okResult("login:rollback"), nil
		}
		return okResult("login"), nil
	})
	runner.OnInvoke("fetch", func(req ToolRequest) (ToolResult, error) {
		return ToolResult{}, fmt.Errorf("network error")
	})

	ch, _ := NewProtocolChannel(runner, ProtocolChannelConfig{})
	p := validPlan(plan.ProtocolPlan, "sess_1",
		validStep("s1", "login", "i1"),
		validStep("s2", "fetch", "i2"),
	)
	art, err := ch.Execute(context.Background(), p, ChannelRequest{SessionID: "sess_1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if art.SideEffectStatus != types.SideEffectRolledBack {
		t.Errorf("SideEffectStatus=%v, want SideEffectRolledBack", art.SideEffectStatus)
	}
	// Both login (forward) + login (rollback) + fetch (failed) = 3 calls
	if runner.CallCount() != 3 {
		t.Errorf("expected 3 runner calls (login + fetch + login-rollback), got %d", runner.CallCount())
	}
}

// TestProtocolChannel_OtherPlan_NotSupported checks Supports guard.
func TestProtocolChannel_OtherPlan_NotSupported(t *testing.T) {
	ch, _ := NewProtocolChannel(newFakeRunner(), ProtocolChannelConfig{})
	for _, k := range []plan.PlanKind{plan.CommitmentPlan, plan.ScenarioPlan, plan.ExplorationPlan} {
		if ch.Supports(k) {
			t.Errorf("ProtocolChannel should NOT support %s", k)
		}
	}
}

// TestProtocolChannel_EmptySteps covers the empty-Steps rejection.
func TestProtocolChannel_EmptySteps(t *testing.T) {
	ch, _ := NewProtocolChannel(newFakeRunner(), ProtocolChannelConfig{})
	p := validPlan(plan.ProtocolPlan, "sess_1")
	if _, err := ch.Execute(context.Background(), p, ChannelRequest{}); err == nil {
		t.Fatal("expected error for 0-step protocol")
	} else if !errors.Is(err, ErrChannelStepCountMismatch) {
		t.Errorf("expected ErrChannelStepCountMismatch, got %v", err)
	}
}

// -----------------------------------------------------------------------------
// D7-S9-A26-T04: ScenarioChannel
// -----------------------------------------------------------------------------

// TestScenarioChannel_5ParallelProbes verifies that 5 probes run in
// parallel (bounded by MaxParallel=5) and produce a ProbeReport.
func TestScenarioChannel_5ParallelProbes(t *testing.T) {
	runner := newFakeRunner()
	var concurrent atomicCounter
	var maxConcurrent atomicCounter
	for i := 0; i < 5; i++ {
		tool := fmt.Sprintf("probe_%d", i)
		runner.OnInvoke(tool, func(req ToolRequest) (ToolResult, error) {
			concurrent.Inc()
			defer concurrent.Dec()
			// Track peak concurrency.
			if c := concurrent.Load(); c > maxConcurrent.Load() {
				maxConcurrent.Store(c)
			}
			time.Sleep(20 * time.Millisecond) // hold slot briefly
			return okResult(tool), nil
		})
	}

	ch, _ := NewScenarioChannel(runner, ScenarioChannelConfig{MaxParallel: 5})
	steps := []plan.Step{
		validStep("s1", "probe_0", "i0"),
		validStep("s2", "probe_1", "i1"),
		validStep("s3", "probe_2", "i2"),
		validStep("s4", "probe_3", "i3"),
		validStep("s5", "probe_4", "i4"),
	}
	p := validPlan(plan.ScenarioPlan, "sess_1", steps...)
	art, err := ch.Execute(context.Background(), p, ChannelRequest{SessionID: "sess_1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if art.Kind != types.ArtifactProbeReport {
		t.Errorf("Kind=%v, want ArtifactProbeReport", art.Kind)
	}
	if art.SideEffectStatus != types.SideEffectNone {
		t.Errorf("SideEffectStatus=%v, want SideEffectNone (read-only)", art.SideEffectStatus)
	}
	if maxConcurrent.Load() < 2 {
		t.Errorf("probes did not run in parallel (maxConcurrent=%d)", maxConcurrent.Load())
	}
}

// TestScenarioChannel_MajorityVote_ProbeReport covers the majority-vote
// policy: 3 successes out of 5 → pass.
func TestScenarioChannel_MajorityVote_ProbeReport(t *testing.T) {
	runner := newFakeRunner()
	for i := 0; i < 3; i++ {
		tool := fmt.Sprintf("ok_%d", i)
		runner.OnInvoke(tool, func(req ToolRequest) (ToolResult, error) {
			return okResult(tool), nil
		})
	}
	for i := 0; i < 2; i++ {
		tool := fmt.Sprintf("fail_%d", i)
		runner.OnInvoke(tool, func(req ToolRequest) (ToolResult, error) {
			return ToolResult{}, fmt.Errorf("nope")
		})
	}
	ch, _ := NewScenarioChannel(runner, ScenarioChannelConfig{})
	steps := []plan.Step{
		validStep("s1", "ok_0", "i0"),
		validStep("s2", "ok_1", "i1"),
		validStep("s3", "ok_2", "i2"),
		validStep("s4", "fail_0", "i3"),
		validStep("s5", "fail_1", "i4"),
	}
	p := validPlan(plan.ScenarioPlan, "sess_1", steps...)
	art, err := ch.Execute(context.Background(), p, ChannelRequest{SessionID: "sess_1"})
	if err != nil {
		t.Fatalf("Execute: %v (majority of 3/5 should pass)", err)
	}
	if art.ExitCode != 0 {
		t.Errorf("ExitCode=%d, want 0", art.ExitCode)
	}
	if !strings.Contains(art.Summary, "3/5 probes succeeded") {
		t.Errorf("Summary missing vote count: %q", art.Summary)
	}
}

// TestScenarioChannel_MixedResults_TakesMajority covers the failure path
// where the majority rejected (2/5 pass = fail since threshold is 2).
func TestScenarioChannel_MixedResults_TakesMajority(t *testing.T) {
	runner := newFakeRunner()
	runner.OnInvoke("ok_0", func(req ToolRequest) (ToolResult, error) { return okResult("ok_0"), nil })
	for i := 1; i < 5; i++ {
		tool := fmt.Sprintf("fail_%d", i)
		runner.OnInvoke(tool, func(req ToolRequest) (ToolResult, error) {
			return ToolResult{}, fmt.Errorf("nope")
		})
	}
	ch, _ := NewScenarioChannel(runner, ScenarioChannelConfig{})
	steps := []plan.Step{
		validStep("s1", "ok_0", "i0"),
		validStep("s2", "fail_1", "i1"),
		validStep("s3", "fail_2", "i2"),
		validStep("s4", "fail_3", "i3"),
		validStep("s5", "fail_4", "i4"),
	}
	p := validPlan(plan.ScenarioPlan, "sess_1", steps...)
	_, err := ch.Execute(context.Background(), p, ChannelRequest{SessionID: "sess_1"})
	if err == nil {
		t.Fatal("expected majority-failure error")
	}
}

// TestScenarioChannel_CtxCancel_SurfacesCtxError is the RH-D7-09
// regression test (DM-20260630-013 T-P1-A3-8.2). When the outer ctx is
// cancelled mid-run, the channel must return a NewChannelCtxCancelledError
// — NOT the misleading ErrChannelStepCountMismatch that the majority-vote
// failure path would otherwise produce. Without the early check, callers
// downstream (StrategyDecider) couldn't distinguish a turn-abort cancel
// from a real probe majority failure.
func TestScenarioChannel_CtxCancel_SurfacesCtxError(t *testing.T) {
	runner := newFakeRunner()
	// Probe handlers return ok quickly (fakeRunner doesn't propagate ctx
	// to handlers); the channel observes ctx.Err() after wg.Wait().
	for i := 0; i < 5; i++ {
		tool := fmt.Sprintf("ok_%d", i)
		runner.OnInvoke(tool, func(req ToolRequest) (ToolResult, error) {
			return okResult(tool), nil
		})
	}
	ch, _ := NewScenarioChannel(runner, ScenarioChannelConfig{Timeout: 5 * time.Second})
	steps := make([]plan.Step, 5)
	for i := range steps {
		steps[i] = validStep(fmt.Sprintf("s%d", i+1), fmt.Sprintf("ok_%d", i), fmt.Sprintf("i%d", i))
	}
	p := validPlan(plan.ScenarioPlan, "sess_cancel", steps...)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE Execute — wg.Wait returns immediately, ctx.Err() != nil
	art, err := ch.Execute(ctx, p, ChannelRequest{SessionID: "sess_cancel"})
	if err == nil {
		t.Fatal("expected ctx-cancelled error, got nil")
	}
	if !errors.Is(err, ErrChannelCtxCancelled) {
		t.Errorf("err = %v, want errors.Is(ErrChannelCtxCancelled)", err)
	}
	if art == nil {
		t.Fatal("art nil on ctx cancel")
	}
	if art.SideEffectStatus != types.SideEffectUnknown {
		t.Errorf("SideEffectStatus = %v, want SideEffectUnknown on ctx cancel", art.SideEffectStatus)
	}
}

// -----------------------------------------------------------------------------
// D7-S9-A26-T05: ExplorationChannel
// -----------------------------------------------------------------------------

// TestExplorationChannel_MultiAgent_Parallel covers the parallel
// exploration + ExperimentData artifact.
func TestExplorationChannel_MultiAgent_Parallel(t *testing.T) {
	runner := newFakeRunner()
	runner.OnInvoke("exp_a", func(req ToolRequest) (ToolResult, error) {
		time.Sleep(15 * time.Millisecond)
		return okResult("exp_a"), nil
	})
	runner.OnInvoke("exp_b", func(req ToolRequest) (ToolResult, error) {
		time.Sleep(15 * time.Millisecond)
		return okResult("exp_b"), nil
	})
	runner.OnInvoke("exp_c", func(req ToolRequest) (ToolResult, error) {
		time.Sleep(15 * time.Millisecond)
		return okResult("exp_c"), nil
	})
	ch, _ := NewExplorationChannel(runner, ExplorationChannelConfig{MaxParallel: 3})
	p := validPlan(plan.ExplorationPlan, "sess_1",
		validStep("s1", "exp_a", "i1"),
		validStep("s2", "exp_b", "i2"),
		validStep("s3", "exp_c", "i3"),
	)
	art, err := ch.Execute(context.Background(), p, ChannelRequest{SessionID: "sess_1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if art.Kind != types.ArtifactExperimentData {
		t.Errorf("Kind=%v, want ArtifactExperimentData", art.Kind)
	}
	if art.SideEffectStatus != types.SideEffectCommitted {
		t.Errorf("SideEffectStatus=%v, want SideEffectCommitted (PersistSession)",
			art.SideEffectStatus)
	}
	if !strings.Contains(art.Summary, "3/3 succeeded") {
		t.Errorf("Summary missing success count: %q", art.Summary)
	}
}

// TestExplorationChannel_FreeFork_Optional covers the free-fork pattern:
// ExplorationChannel tolerates partial failures (unlike ScenarioChannel).
func TestExplorationChannel_FreeFork_Optional(t *testing.T) {
	runner := newFakeRunner()
	runner.OnInvoke("win", func(req ToolRequest) (ToolResult, error) { return okResult("win"), nil })
	runner.OnInvoke("lose", func(req ToolRequest) (ToolResult, error) {
		return ToolResult{}, fmt.Errorf("trial and error is the point")
	})
	ch, _ := NewExplorationChannel(runner, ExplorationChannelConfig{})
	p := validPlan(plan.ExplorationPlan, "sess_1",
		validStep("s1", "win", "i1"),
		validStep("s2", "lose", "i2"),
	)
	art, err := ch.Execute(context.Background(), p, ChannelRequest{SessionID: "sess_1"})
	if err != nil {
		t.Fatalf("Execute: %v (exploration must tolerate partial failure)", err)
	}
	// Top result is the winning "win" tool, so ExitCode=0.
	if art.ExitCode != 0 {
		t.Errorf("ExitCode=%d, want 0 (winning top result)", art.ExitCode)
	}
	if !strings.Contains(art.Summary, "1/2 succeeded") {
		t.Errorf("Summary missing 1/2 count: %q", art.Summary)
	}
}

// TestExplorationChannel_PriorityOrder_ExperimentData verifies that the
// top-ranked result becomes the Artifact.Summary.
func TestExplorationChannel_PriorityOrder_ExperimentData(t *testing.T) {
	runner := newFakeRunner()
	runner.OnInvoke("slow_loser", func(req ToolRequest) (ToolResult, error) {
		// Slow + losing: should be ranked last.
		time.Sleep(50 * time.Millisecond)
		return ToolResult{}, fmt.Errorf("slow fail")
	})
	runner.OnInvoke("fast_winner", func(req ToolRequest) (ToolResult, error) {
		// Fast + winning: should be ranked first.
		time.Sleep(5 * time.Millisecond)
		return okResult("fast_winner"), nil
	})
	runner.OnInvoke("medium_loser", func(req ToolRequest) (ToolResult, error) {
		time.Sleep(25 * time.Millisecond)
		return ToolResult{}, fmt.Errorf("medium fail")
	})
	ch, _ := NewExplorationChannel(runner, ExplorationChannelConfig{})
	p := validPlan(plan.ExplorationPlan, "sess_1",
		validStep("s1", "slow_loser", "i1"),
		validStep("s2", "fast_winner", "i2"),
		validStep("s3", "medium_loser", "i3"),
	)
	art, err := ch.Execute(context.Background(), p, ChannelRequest{SessionID: "sess_1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(art.Summary, "fast_winner") {
		t.Errorf("top result should be fast_winner, got Summary=%q", art.Summary)
	}
}

// TestExplorationChannel_PersistScope_Mapping verifies SideEffectStatus
// follows the Plan's PersistScope.
func TestExplorationChannel_PersistScope_Mapping(t *testing.T) {
	cases := []struct {
		scope plan.PersistScope
		want  types.SideEffectStatus
	}{
		{plan.PersistTransient, types.SideEffectNone},
		{plan.PersistSession, types.SideEffectCommitted},
		{plan.PersistPermanent, types.SideEffectCommitted},
		{"unknown", types.SideEffectUnknown},
	}
	for _, c := range cases {
		got := sideEffectForScope(c.scope)
		if got != c.want {
			t.Errorf("sideEffectForScope(%q)=%v, want %v", c.scope, got, c.want)
		}
	}
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

func mustCommit(t *testing.T, r ToolRunner) *CommitChannel {
	t.Helper()
	c, err := NewCommitChannel(r, CommitChannelConfig{})
	if err != nil {
		t.Fatalf("NewCommitChannel: %v", err)
	}
	return c
}

func mustProtocol(t *testing.T, r ToolRunner) *ProtocolChannel {
	t.Helper()
	c, err := NewProtocolChannel(r, ProtocolChannelConfig{})
	if err != nil {
		t.Fatalf("NewProtocolChannel: %v", err)
	}
	return c
}

func mustScenario(t *testing.T, r ToolRunner) *ScenarioChannel {
	t.Helper()
	c, err := NewScenarioChannel(r, ScenarioChannelConfig{})
	if err != nil {
		t.Fatalf("NewScenarioChannel: %v", err)
	}
	return c
}

func mustExploration(t *testing.T, r ToolRunner) *ExplorationChannel {
	t.Helper()
	c, err := NewExplorationChannel(r, ExplorationChannelConfig{})
	if err != nil {
		t.Fatalf("NewExplorationChannel: %v", err)
	}
	return c
}

func registerAll(t *testing.T, r ToolRunner) *ChannelRegistry {
	t.Helper()
	reg := NewChannelRegistry()
	if err := reg.Register(mustCommit(t, r)); err != nil {
		t.Fatalf("Register commit: %v", err)
	}
	if err := reg.Register(mustProtocol(t, r)); err != nil {
		t.Fatalf("Register protocol: %v", err)
	}
	if err := reg.Register(mustScenario(t, r)); err != nil {
		t.Fatalf("Register scenario: %v", err)
	}
	if err := reg.Register(mustExploration(t, r)); err != nil {
		t.Fatalf("Register exploration: %v", err)
	}
	return reg
}

// atomicCounter is a tiny atomic int for the parallelism assertion.
type atomicCounter struct {
	mu sync.Mutex
	v  int
}

func (a *atomicCounter) Inc() {
	a.mu.Lock()
	a.v++
	a.mu.Unlock()
}

func (a *atomicCounter) Dec() {
	a.mu.Lock()
	a.v--
	a.mu.Unlock()
}

func (a *atomicCounter) Load() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.v
}

func (a *atomicCounter) Store(v int) {
	a.mu.Lock()
	a.v = v
	a.mu.Unlock()
}
