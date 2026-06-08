//go:build smoke && d4

package e2e

import (
	"context"
	"sync"
	"testing"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/agent"
	multiagentfactory "github.com/devrix/devrix/internal/layers/multiagent/factory"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// Covers: L5-4-0-04
func TestE2E_AgentForkParallelJoin(t *testing.T) {
	factory := multiagentfactory.NewAgentFactory(multiagent.AgentDeps{
		Engine: &agent.StubEngine{
			Events: []*contracts.EngineEvent{{Type: "complete", Content: "done"}},
		},
	}, config.DefaultMultiAgentConfig())

	session := types.NewSession("sess_e2e_fork", "cli", t.TempDir())
	parent, err := factory.Create(context.Background(), multiagent.AgentConfig{
		SessionID:   session.SessionID,
		WorkDir:     session.WorkDir,
		MaxChildren: 2,
	}, session)
	if err != nil {
		t.Fatalf("create parent: %v", err)
	}

	child1, err := parent.Fork(context.Background(), multiagent.AgentConfig{})
	if err != nil {
		t.Fatalf("fork child1: %v", err)
	}
	child2, err := parent.Fork(context.Background(), multiagent.AgentConfig{})
	if err != nil {
		t.Fatalf("fork child2: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	run := func(child multiagent.Agent) {
		defer wg.Done()
		if _, err := child.Run(context.Background()); err != nil {
			t.Errorf("child run: %v", err)
		}
	}
	go run(child1)
	go run(child2)
	wg.Wait()

	if err := parent.Join(context.Background(), child1); err != nil {
		t.Fatalf("join child1: %v", err)
	}
	if err := parent.Join(context.Background(), child2); err != nil {
		t.Fatalf("join child2: %v", err)
	}
	if len(parent.GetMessages()) == 0 {
		t.Fatal("expected merged child messages on parent")
	}
}
