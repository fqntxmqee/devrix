package delegate

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/worktree"
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

type stubLeader struct {
	cfg multiagent.AgentConfig
}

func (s *stubLeader) ID() string                                       { return "leader-1" }
func (s *stubLeader) State() multiagent.AgentState                     { return multiagent.AgentStateRunning }
func (s *stubLeader) Config() multiagent.AgentConfig                   { return s.cfg }
func (s *stubLeader) Run(context.Context) (*multiagent.AgentResult, error) {
	return nil, nil
}
func (s *stubLeader) Fork(_ context.Context, cfg multiagent.AgentConfig) (multiagent.Agent, error) {
	return &stubWorker{cfg: cfg}, nil
}
func (s *stubLeader) Join(context.Context, multiagent.Agent) error { return nil }
func (s *stubLeader) Terminate(context.Context) error              { return nil }
func (s *stubLeader) Wait(context.Context) (*multiagent.AgentResult, error) {
	return nil, nil
}
func (s *stubLeader) ResolvePermission(string, bool)                 {}
func (s *stubLeader) GetMessages() []types.Message                   { return nil }
func (s *stubLeader) SetAgentObserver(multiagent.AgentObserver)      {}
func (s *stubLeader) SetEngineEventSink(func(*contracts.EngineEvent)) {}

type stubWorker struct {
	cfg multiagent.AgentConfig
}

func (w *stubWorker) ID() string { return "worker-wt" }
func (w *stubWorker) State() multiagent.AgentState {
	return multiagent.AgentStateCreated
}
func (w *stubWorker) Config() multiagent.AgentConfig { return w.cfg }
func (w *stubWorker) Run(context.Context) (*multiagent.AgentResult, error) {
	if w.cfg.WorkDir != "" {
		_ = os.WriteFile(filepath.Join(w.cfg.WorkDir, "isolated.txt"), []byte("ok"), 0o644)
	}
	return &multiagent.AgentResult{ExitCode: 0}, nil
}
func (w *stubWorker) Fork(context.Context, multiagent.AgentConfig) (multiagent.Agent, error) {
	return nil, nil
}
func (w *stubWorker) Join(context.Context, multiagent.Agent) error { return nil }
func (w *stubWorker) Terminate(context.Context) error              { return nil }
func (w *stubWorker) Wait(context.Context) (*multiagent.AgentResult, error) {
	return nil, nil
}
func (w *stubWorker) ResolvePermission(string, bool)                 {}
func (w *stubWorker) GetMessages() []types.Message                   { return nil }
func (w *stubWorker) SetAgentObserver(multiagent.AgentObserver)      {}
func (w *stubWorker) SetEngineEventSink(func(*contracts.EngineEvent)) {}

// Covers: L5-4-12-01 (delegate path uses worktree WorkDir)
func TestDelegateSync_should_run_worker_in_worktree_sandbox(t *testing.T) {
	mainDir := t.TempDir()
	baseDir := filepath.Join(t.TempDir(), "wt")
	wt := worktree.NewManager(config.WorktreeConfig{Enabled: true, BaseDir: baseDir})
	svc := NewService(config.DelegateConfig{Enabled: true}, nil, wt, nil)
	leader := &stubLeader{cfg: multiagent.AgentConfig{SessionID: "sess_d", WorkDir: mainDir}}

	_, err := svc.DelegateSync(context.Background(), leader, WorkerSpec{
		Role:         WorkerRoleImplement,
		Directive:    "write file",
		WorktreeSlug: "impl-1",
	})
	if err != nil {
		t.Fatalf("DelegateSync: %v", err)
	}
	if _, err := os.Stat(filepath.Join(mainDir, "isolated.txt")); !os.IsNotExist(err) {
		t.Fatal("main workdir must not receive worker writes")
	}
}
