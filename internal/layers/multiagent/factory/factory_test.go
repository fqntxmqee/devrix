package factory

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/agent"
	"github.com/devrix/devrix/internal/layers/multiagent/observer"
	sharedconfig "github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// Covers: L5-4-1-01
func TestAgentFactory_should_create_agent_in_created_state(t *testing.T) {
	f := NewAgentFactory(multiagent.AgentDeps{
		Engine: &agent.StubEngine{
			Events: []*gateway.EngineEvent{{Type: "complete", Content: "ok"}},
		},
		AgentObserver: observer.NoOpAgentObserver{},
	}, sharedconfig.DefaultMultiAgentConfig())

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
	f := NewAgentFactory(multiagent.AgentDeps{
		Engine: &agent.StubEngine{},
	}, nil)
	_, err := f.Create(context.Background(), multiagent.AgentConfig{
		WorkDir: "/tmp",
	}, types.NewSession("sess_x", "cli", "/tmp"))
	if err == nil {
		t.Fatal("expected error for missing session_id in config")
	}
}
