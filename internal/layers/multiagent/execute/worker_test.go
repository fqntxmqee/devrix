package execute_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/execute"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// stubAgent implements multiagent.Agent for testing.
type stubAgent struct {
	id       string
	state    multiagent.AgentState
	cfg      multiagent.AgentConfig
	messages []types.Message
	children []multiagent.Agent
	joined   []string
	observer multiagent.AgentObserver
	sink     func(*contracts.EngineEvent)
	runErr   error
	runMsgs  []types.Message
}

func newStubAgent(id, sessionID, workDir string) *stubAgent {
	return &stubAgent{
		id:    id,
		state: multiagent.AgentStateCreated,
		cfg: multiagent.AgentConfig{
			SessionID: sessionID,
			WorkDir:   workDir,
		},
		messages: make([]types.Message, 0),
	}
}

func (s *stubAgent) ID() string                              { return s.id }
func (s *stubAgent) State() multiagent.AgentState             { return s.state }
func (s *stubAgent) Config() multiagent.AgentConfig           { return s.cfg }
func (s *stubAgent) GetMessages() []types.Message             { return s.messages }
func (s *stubAgent) ResolvePermission(string, bool)           {}
func (s *stubAgent) Terminate(context.Context) error          { return nil }
func (s *stubAgent) SetAgentObserver(o multiagent.AgentObserver) { s.observer = o }
func (s *stubAgent) SetEngineEventSink(f func(*contracts.EngineEvent)) { s.sink = f }

func (s *stubAgent) Run(ctx context.Context) (*multiagent.AgentResult, error) {
	if s.runErr != nil {
		return nil, s.runErr
	}
	s.state = multiagent.AgentStateTerminated
	s.messages = s.runMsgs
	return &multiagent.AgentResult{Messages: s.runMsgs, ExitCode: 0}, nil
}

func (s *stubAgent) Wait(ctx context.Context) (*multiagent.AgentResult, error) {
	return &multiagent.AgentResult{Messages: s.messages, ExitCode: 0}, nil
}

func (s *stubAgent) Fork(ctx context.Context, cfg multiagent.AgentConfig) (multiagent.Agent, error) {
	child := newStubAgent("child-"+s.id+"-"+cfg.WorkerRole, cfg.SessionID, cfg.WorkDir)
	child.cfg = cfg
	s.children = append(s.children, child)
	return child, nil
}

func (s *stubAgent) Join(ctx context.Context, child multiagent.Agent) error {
	s.joined = append(s.joined, child.ID())
	s.messages = append(s.messages, child.GetMessages()...)
	return nil
}

// --- Tests ---

func TestNewExecutor(t *testing.T) {
	e := execute.NewExecutor(config.DelegateConfig{Enabled: true}, nil, nil)
	if e == nil {
		t.Fatal("NewExecutor returned nil")
	}
}

func TestExecuteSync_nilLeader(t *testing.T) {
	e := execute.NewExecutor(config.DelegateConfig{}, nil, nil)
	_, err := e.ExecuteSync(context.Background(), nil, execute.WorkerRunSpec{})
	if err == nil {
		t.Fatal("expected error for nil leader")
	}
	if !strings.Contains(err.Error(), "leader is nil") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteSync_success(t *testing.T) {
	leader := newStubAgent("leader-1", "sess-1", "/tmp")
	childMsgs := []types.Message{
		{Role: types.MessageRoleAssistant, Content: "task done"},
	}
	// Pre-configure the child that will be forked. The stub creates children
	// dynamically in Fork, so we set expectations on the leader.
	e := execute.NewExecutor(config.DelegateConfig{Enabled: true}, nil, nil)

	result, err := e.ExecuteSync(context.Background(), leader, execute.WorkerRunSpec{
		Role:      "plan",
		Directive: "plan this task",
		TaskID:    "task-1",
		MaxTurns:  3,
	})
	if err != nil {
		t.Fatalf("ExecuteSync: %v", err)
	}
	if result.WorkerID == "" {
		t.Fatal("WorkerID is empty")
	}
	if len(leader.joined) == 0 {
		t.Fatal("leader should have joined child")
	}
	_ = childMsgs
}

func TestExecuteSync_observerCalled(t *testing.T) {
	leader := newStubAgent("leader-obs", "sess-obs", "/tmp")
	obs := &recordingObserver{}
	e := execute.NewExecutor(config.DelegateConfig{Enabled: true}, nil, obs)

	_, err := e.ExecuteSync(context.Background(), leader, execute.WorkerRunSpec{
		Role:      "explore",
		Directive: "search",
		TaskID:    "task-obs",
	})
	if err != nil {
		t.Fatalf("ExecuteSync: %v", err)
	}
	if obs.forkCalls == 0 {
		t.Fatal("OnWorkerForked was not called")
	}
	if obs.completeCalls == 0 {
		t.Fatal("OnWorkerCompleted was not called")
	}
}

func TestExecuteSync_perCallObserver(t *testing.T) {
	leader := newStubAgent("leader-pco", "sess-pco", "/tmp")
	defaultObs := &recordingObserver{}
	e := execute.NewExecutor(config.DelegateConfig{Enabled: true}, nil, defaultObs)

	perCallObs := &recordingObserver{}
	_, err := e.ExecuteSync(context.Background(), leader, execute.WorkerRunSpec{
		Role:     "implement",
		TaskID:   "task-pco",
		Observer: perCallObs,
	})
	if err != nil {
		t.Fatalf("ExecuteSync: %v", err)
	}
	if defaultObs.forkCalls != 0 {
		t.Fatal("default observer should NOT be called when per-call observer is set")
	}
	if perCallObs.forkCalls == 0 {
		t.Fatal("per-call observer should be called")
	}
}

func TestExecuteAsync_disabled(t *testing.T) {
	leader := newStubAgent("leader-async", "sess-async", "/tmp")
	e := execute.NewExecutor(config.DelegateConfig{AllowAsync: false}, nil, nil)

	_, err := e.ExecuteAsync(context.Background(), leader, execute.WorkerRunSpec{
		Role: "plan",
	})
	if err == nil {
		t.Fatal("expected error when async is disabled")
	}
	if !strings.Contains(err.Error(), "async not enabled") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteAsync_enabled(t *testing.T) {
	leader := newStubAgent("leader-ae", "sess-ae", "/tmp")
	e := execute.NewExecutor(config.DelegateConfig{AllowAsync: true}, nil, nil)

	workerID, err := e.ExecuteAsync(context.Background(), leader, execute.WorkerRunSpec{
		Role:      "plan",
		Directive: "async plan",
		TaskID:    "task-ae",
	})
	if err != nil {
		t.Fatalf("ExecuteAsync: %v", err)
	}
	if workerID == "" {
		t.Fatal("workerID is empty")
	}
}

func TestExecuteSync_forkError(t *testing.T) {
	// Agent that fails on Fork
	badLeader := &forkErrorAgent{id: "bad", cfg: multiagent.AgentConfig{SessionID: "s1", WorkDir: "/tmp"}}
	e := execute.NewExecutor(config.DelegateConfig{Enabled: true}, nil, nil)

	_, err := e.ExecuteSync(context.Background(), badLeader, execute.WorkerRunSpec{Role: "plan", TaskID: "t1"})
	if err == nil {
		t.Fatal("expected fork error")
	}
}

func TestExecuteSync_runError(t *testing.T) {
	e := execute.NewExecutor(config.DelegateConfig{Enabled: true}, nil, nil)
	leader := &runErrorLeader{id: "leader-rerr", sessionID: "sess-rerr", workDir: "/tmp"}
	_, err := e.ExecuteSync(context.Background(), leader, execute.WorkerRunSpec{
		Role:     "implement",
		TaskID:   "task-runerr",
		MaxTurns: 5,
	})
	if err == nil {
		t.Fatal("expected run error")
	}
	if !strings.Contains(err.Error(), "run exploded") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Helpers ---

type recordingObserver struct {
	forkCalls     int
	completeCalls int
}

func (r *recordingObserver) OnWorkerForked(string, string, multiagent.Agent) { r.forkCalls++ }
func (r *recordingObserver) OnWorkerCompleted(string, string, string, error) { r.completeCalls++ }

type forkErrorAgent struct {
	id  string
	cfg multiagent.AgentConfig
}

func (f *forkErrorAgent) ID() string                                      { return f.id }
func (f *forkErrorAgent) State() multiagent.AgentState                    { return multiagent.AgentStateCreated }
func (f *forkErrorAgent) Config() multiagent.AgentConfig                  { return f.cfg }
func (f *forkErrorAgent) GetMessages() []types.Message                    { return nil }
func (f *forkErrorAgent) ResolvePermission(string, bool)                  {}
func (f *forkErrorAgent) Terminate(context.Context) error                 { return nil }
func (f *forkErrorAgent) SetAgentObserver(multiagent.AgentObserver)       {}
func (f *forkErrorAgent) SetEngineEventSink(func(*contracts.EngineEvent)) {}
func (f *forkErrorAgent) Run(ctx context.Context) (*multiagent.AgentResult, error) {
	return &multiagent.AgentResult{ExitCode: 0}, nil
}
func (f *forkErrorAgent) Wait(ctx context.Context) (*multiagent.AgentResult, error) {
	return &multiagent.AgentResult{}, nil
}
func (f *forkErrorAgent) Fork(ctx context.Context, cfg multiagent.AgentConfig) (multiagent.Agent, error) {
	return nil, errors.New("fork failed")
}
func (f *forkErrorAgent) Join(ctx context.Context, child multiagent.Agent) error { return nil }

// runErrorLeader is an agent whose Fork returns a child that errors on Run.
type runErrorLeader struct {
	id        string
	sessionID string
	workDir   string
	cfg       multiagent.AgentConfig
}

func (r *runErrorLeader) ID() string            { return r.id }
func (r *runErrorLeader) State() multiagent.AgentState { return multiagent.AgentStateCreated }
func (r *runErrorLeader) Config() multiagent.AgentConfig {
	return multiagent.AgentConfig{SessionID: r.sessionID, WorkDir: r.workDir}
}
func (r *runErrorLeader) GetMessages() []types.Message                    { return nil }
func (r *runErrorLeader) ResolvePermission(string, bool)                  {}
func (r *runErrorLeader) Terminate(context.Context) error                 { return nil }
func (r *runErrorLeader) SetAgentObserver(multiagent.AgentObserver)       {}
func (r *runErrorLeader) SetEngineEventSink(func(*contracts.EngineEvent)) {}
func (r *runErrorLeader) Run(ctx context.Context) (*multiagent.AgentResult, error) {
	return &multiagent.AgentResult{ExitCode: 0}, nil
}
func (r *runErrorLeader) Wait(ctx context.Context) (*multiagent.AgentResult, error) {
	return nil, nil
}
func (r *runErrorLeader) Fork(ctx context.Context, cfg multiagent.AgentConfig) (multiagent.Agent, error) {
	child := newStubAgent("child-err", r.sessionID, r.workDir)
	child.runErr = errors.New("run exploded")
	return child, nil
}
func (r *runErrorLeader) Join(ctx context.Context, child multiagent.Agent) error { return nil }
