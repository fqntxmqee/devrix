package provision

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/run"
	"github.com/devrix/devrix/internal/layers/multiagent/observer"
	"github.com/devrix/devrix/internal/layers/multiagent/isolate"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D4-S1-A01-T01
func TestAgentFactory_should_create_agent_in_created_state(t *testing.T) {
	f := NewAgentFactory(multiagent.AgentDeps{
		Engine: &run.StubEngine{
			Events: []*contracts.EngineEvent{{Type: "complete", Content: "ok"}},
		},
		AgentObserver: observer.NoOpAgentObserver{},
	}, config.DefaultMultiAgentConfig())

	session := types.NewSession("sess_factory", "cli", "/tmp/work")
	ag, err := f.Create(context.Background(), multiagent.AgentConfig{
		SessionID: session.SessionID,
		WorkDir:   session.WorkDir,
	}, session)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if ag.State() != multiagent.AgentStateCreated {
		t.Fatalf("State() = %s, want CREATED", ag.State())
	}
	if ag.ID() == "" {
		t.Fatal("ID() is empty")
	}
}

func TestAgentFactory_should_reject_missing_session_id(t *testing.T) {
	f := NewAgentFactory(multiagent.AgentDeps{Engine: &run.StubEngine{}}, nil)
	_, err := f.Create(context.Background(), multiagent.AgentConfig{
		WorkDir: "/tmp",
	}, types.NewSession("sess_x", "cli", "/tmp"))
	if err == nil {
		t.Fatal("expected error for missing session_id in config")
	}
}

// T: D4-S3-A01-T02
type countingEngineBuilder struct {
	builds int
}

func (b *countingEngineBuilder) Build(_ multiagent.PermissionGate) contracts.IEngine {
	b.builds++
	return &run.StubEngine{Events: []*contracts.EngineEvent{{Type: "complete"}}}
}

func TestAgentFactory_should_use_shared_engine_for_root_agents(t *testing.T) {
	builder := &countingEngineBuilder{}
	shared := &run.StubEngine{Events: []*contracts.EngineEvent{{Type: "complete"}}}
	f := NewAgentFactoryWithBuilder(multiagent.AgentDeps{Engine: shared}, builder, config.DefaultMultiAgentConfig())
	session := types.NewSession("sess_shared", "cli", "/tmp")
	base := multiagent.AgentConfig{SessionID: session.SessionID, WorkDir: session.WorkDir}

	if _, err := f.Create(context.Background(), base, session); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := f.Create(context.Background(), base, session); err != nil {
		t.Fatalf("second create: %v", err)
	}
	if builder.builds != 0 {
		t.Fatalf("builder.Build calls = %d, want 0 for root agents with shared engine", builder.builds)
	}
}

func TestAgentFactory_should_build_isolated_engine_for_worker_agents(t *testing.T) {
	builder := &countingEngineBuilder{}
	shared := &run.StubEngine{Events: []*contracts.EngineEvent{{Type: "complete"}}}
	f := NewAgentFactoryWithBuilder(multiagent.AgentDeps{Engine: shared}, builder, config.DefaultMultiAgentConfig())
	session := types.NewSession("sess_worker", "cli", "/tmp")

	if _, err := f.Create(context.Background(), multiagent.AgentConfig{
		SessionID: session.SessionID,
		WorkDir:   session.WorkDir,
		ParentID:  "parent-agent",
	}, session); err != nil {
		t.Fatalf("worker create: %v", err)
	}
	if builder.builds != 1 {
		t.Fatalf("builder.Build calls = %d, want 1 for worker agents", builder.builds)
	}
}

func TestAgentFactory_should_enforce_max_total_agents_per_session(t *testing.T) {
	cfg := config.DefaultMultiAgentConfig()
	cfg.MaxTotalAgents = 2
	f := NewAgentFactory(multiagent.AgentDeps{Engine: &run.StubEngine{}}, cfg)
	session := types.NewSession("sess_cap", "cli", "/tmp")
	base := multiagent.AgentConfig{SessionID: session.SessionID, WorkDir: session.WorkDir}

	if _, err := f.Create(context.Background(), base, session); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := f.Create(context.Background(), base, session); err != nil {
		t.Fatalf("second create: %v", err)
	}
	if _, err := f.Create(context.Background(), base, session); err == nil {
		t.Fatal("expected max total agents error")
	}
}

// T: D4-S3-A01-T04 (ForkSessionView API injection via factory)
func TestAgentFactory_CreateWithView_should_bind_view_to_agent(t *testing.T) {
	f := NewAgentFactory(multiagent.AgentDeps{
		Engine: &run.StubEngine{Events: []*contracts.EngineEvent{{Type: "complete"}}},
	}, config.DefaultMultiAgentConfig())
	session := types.NewSession("sess_create_view", "cli", "/tmp")
	view := isolate.Fork(session)
	view.SetMetadata("pre_bind", "yes")

	ag, err := f.CreateWithView(context.Background(), multiagent.AgentConfig{
		SessionID: session.SessionID,
		WorkDir:   session.WorkDir,
	}, session, view)
	if err != nil {
		t.Fatalf("CreateWithView: %v", err)
	}
	impl, ok := ag.(*run.Impl)
	if !ok {
		t.Fatalf("expected *run.Impl, got %T", ag)
	}
	if impl.SessionView() == nil {
		t.Fatal("agent has no SessionView after CreateWithView")
	}
	if v, _ := impl.SessionView().GetMetadata("pre_bind"); v != "yes" {
		t.Errorf("pre_bind = %v, want yes", v)
	}
}
