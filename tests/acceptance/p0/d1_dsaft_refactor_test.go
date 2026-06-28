//go:build acceptance && d1

package p0

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/provision"
	"github.com/devrix/devrix/internal/layers/multiagent/run"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/devrix/devrix/tests/testutil"
)

type countingEntry struct {
	mu        sync.Mutex
	processes int
	events    []*contracts.EngineEvent
}

func (c *countingEntry) ProcessMessage(_ context.Context, sessionID, _ string) (<-chan *contracts.EngineEvent, error) {
	c.mu.Lock()
	c.processes++
	events := c.events
	c.mu.Unlock()
	ch := make(chan *contracts.EngineEvent, len(events)+1)
	for _, ev := range events {
		copy := *ev
		copy.SessionID = sessionID
		ch <- &copy
	}
	if len(events) == 0 {
		ch <- &contracts.EngineEvent{Type: "complete", SessionID: sessionID}
	}
	close(ch)
	return ch, nil
}

func (c *countingEntry) Cancel(context.Context, string) error { return nil }

func (c *countingEntry) processCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.processes
}

// T: D1-RF-T02 — L5: multi-agent wired but ingress stays on D7; leader provisioned Created-only.
func TestL5_D1_Refactor_D7IngressWithSessionAgents(t *testing.T) {
	dir := t.TempDir()
	store, err := capture.NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	handler := testutil.NewMockEventHandler()
	gw := capture.NewCommunicationGateway(store, handler, nil, config.DefaultConfig(), nil)

	entry := &countingEntry{}
	gw.SetOrchestrationEntry(entry)

	factory := provision.NewAgentFactory(multiagent.AgentDeps{
		Engine: &run.StubEngine{Events: []*contracts.EngineEvent{{Type: "complete"}}},
	}, config.DefaultMultiAgentConfig())
	mgr := testutil.WireGatewaySessionAgents(gw, factory)

	session, err := gw.CreateSession("cli", dir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if err := gw.RouteInbound(context.Background(), &types.InboundMessage{
		SessionID: session.SessionID,
		ChatID:    "cli",
		MessageID: "rf-turn-1",
		Content:   "hello refactor",
	}); err != nil {
		t.Fatalf("RouteInbound: %v", err)
	}
	gw.WaitForProcesses()

	if entry.processCount() != 1 {
		t.Fatalf("ProcessMessage calls=%d want 1", entry.processCount())
	}
	ag := mgr.SessionAgent(session.SessionID)
	if ag == nil {
		t.Fatal("expected session leader")
	}
	if ag.State() != multiagent.AgentStateCreated {
		t.Fatalf("leader state=%v want Created", ag.State())
	}
	if !handler.WaitForMessages(1, 2*time.Second) {
		t.Fatal("expected complete outbound")
	}
}
