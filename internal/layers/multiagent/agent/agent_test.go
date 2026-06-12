package agent_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/agent"
	"github.com/devrix/devrix/internal/layers/multiagent/factory"
	"github.com/devrix/devrix/internal/layers/multiagent/observer"
	sharedconfig "github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

func newTestFactory(engine contracts.IEngine) *factory.AgentFactory {
	return factory.NewAgentFactory(multiagent.AgentDeps{
		Engine:        engine,
		AgentObserver: observer.NoOpAgentObserver{},
	}, sharedconfig.DefaultMultiAgentConfig())
}

// Covers: L5-4-2-01
func TestAgent_should_transition_created_to_terminated_on_run(t *testing.T) {
	f := newTestFactory(&agent.StubEngine{
		Events: []*contracts.EngineEvent{
			{Type: "text", Content: "hello", Metadata: map[string]string{"is_complete": "true"}},
			{Type: "complete", Content: "done"},
		},
	})
	session := types.NewSession("sess_lifecycle", "cli", "/tmp")
	ag, err := f.Create(context.Background(), multiagent.AgentConfig{
		SessionID:    session.SessionID,
		WorkDir:      session.WorkDir,
		InitialInput: "hi",
	}, session)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	result, err := ag.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if ag.State() != multiagent.AgentStateTerminated {
		t.Fatalf("State() = %s, want TERMINATED", ag.State())
	}
	if result.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", result.ExitCode)
	}
	if len(ag.GetMessages()) == 0 {
		t.Fatal("expected isolated message buffer to capture assistant output")
	}
}

// Covers: subagent streaming text (QueryLoop emits is_complete=false deltas)
func TestAgent_should_accumulate_streaming_text_deltas(t *testing.T) {
	f := newTestFactory(&agent.StubEngine{
		Events: []*contracts.EngineEvent{
			{Type: "text", Content: "分析", Metadata: map[string]string{"is_complete": "false"}},
			{Type: "text", Content: "结论", Metadata: map[string]string{"is_complete": "false"}},
			{Type: "complete"},
		},
	})
	session := types.NewSession("sess_stream", "cli", "/tmp")
	ag, err := f.Create(context.Background(), multiagent.AgentConfig{
		SessionID:    session.SessionID,
		WorkDir:      session.WorkDir,
		InitialInput: "analyze",
	}, session)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	result, err := ag.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	msgs := ag.GetMessages()
	if len(msgs) == 0 {
		t.Fatal("expected assistant message from streamed deltas")
	}
	last := msgs[len(msgs)-1]
	if last.Role != types.MessageRoleAssistant || last.Content != "分析结论" {
		t.Fatalf("assistant content = %+v, want 分析结论", last)
	}
	if result == nil || len(result.Messages) == 0 {
		t.Fatal("expected result messages")
	}
}

func TestAgent_should_reject_run_when_already_terminated(t *testing.T) {
	f := newTestFactory(&agent.StubEngine{Events: []*contracts.EngineEvent{{Type: "complete"}}})
	session := types.NewSession("sess_twice", "cli", "/tmp")
	ag, _ := f.Create(context.Background(), multiagent.AgentConfig{
		SessionID: session.SessionID,
		WorkDir:   session.WorkDir,
	}, session)
	if _, err := ag.Run(context.Background()); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if _, err := ag.Run(context.Background()); err == nil {
		t.Fatal("expected error on second Run")
	}
}

// Covers: L5-4-3-01, L5-4-0-01, L5-4-0-02
func TestAgent_should_fork_join_with_isolated_buffers(t *testing.T) {
	f := newTestFactory(&agent.StubEngine{
		Events: []*contracts.EngineEvent{{Type: "complete", Content: "child-done"}},
	})
	session := types.NewSession("sess_fork", "cli", "/tmp")
	parent, err := f.Create(context.Background(), multiagent.AgentConfig{
		SessionID:   session.SessionID,
		WorkDir:     session.WorkDir,
		MaxChildren: 2,
	}, session)
	if err != nil {
		t.Fatalf("Create parent: %v", err)
	}

	child1, err := parent.Fork(context.Background(), multiagent.AgentConfig{})
	if err != nil {
		t.Fatalf("Fork child1: %v", err)
	}
	child2, err := parent.Fork(context.Background(), multiagent.AgentConfig{})
	if err != nil {
		t.Fatalf("Fork child2: %v", err)
	}
	if _, err := parent.Fork(context.Background(), multiagent.AgentConfig{}); err == nil {
		t.Fatal("expected max children error")
	}

	var wg sync.WaitGroup
	wg.Add(2)
	runChild := func(child multiagent.Agent) {
		defer wg.Done()
		if _, err := child.Run(context.Background()); err != nil {
			t.Errorf("child Run: %v", err)
		}
	}
	go runChild(child1)
	go runChild(child2)
	wg.Wait()

	if err := parent.Join(context.Background(), child1); err != nil {
		t.Fatalf("Join child1: %v", err)
	}
	if err := parent.Join(context.Background(), child2); err != nil {
		t.Fatalf("Join child2: %v", err)
	}
	if len(parent.GetMessages()) == 0 {
		t.Fatal("parent should have merged child messages")
	}
}

func TestAgent_should_reject_join_before_child_completes(t *testing.T) {
	f := newTestFactory(&agent.StubEngine{Events: []*contracts.EngineEvent{{Type: "complete"}}})
	session := types.NewSession("sess_join_early", "cli", "/tmp")
	parent, _ := f.Create(context.Background(), multiagent.AgentConfig{
		SessionID: session.SessionID,
		WorkDir:   session.WorkDir,
	}, session)
	child, _ := parent.Fork(context.Background(), multiagent.AgentConfig{})
	err := parent.Join(context.Background(), child)
	if err == nil {
		t.Fatal("expected join-not-completed error")
	}
	var se *sharederrors.SentinelError
	if !errors.As(err, &se) {
		t.Fatalf("expected SentinelError, got %v", err)
	}
}

// Covers: L5-4-3-03
func TestAgent_should_timeout_when_exceeded(t *testing.T) {
	block := make(chan struct{})
	f := newTestFactory(&blockingEngine{block: block})
	session := types.NewSession("sess_timeout", "cli", "/tmp")
	ag, err := f.Create(context.Background(), multiagent.AgentConfig{
		SessionID:    session.SessionID,
		WorkDir:      session.WorkDir,
		InitialInput: "hi",
		Timeout:      50 * time.Millisecond,
	}, session)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = ag.Run(context.Background())
	if err == nil {
		close(block)
		t.Fatal("expected timeout error")
	}
	var se *sharederrors.SentinelError
	if !errors.As(err, &se) || se.Code != sharederrors.CodeAgentTimeout {
		t.Fatalf("expected AGT_LIFECYCLE_5005, got %v", err)
	}
	if ag.State() != multiagent.AgentStateTerminated {
		t.Fatalf("State() = %s, want TERMINATED", ag.State())
	}
}

// Covers: L5-4-3-04
func TestAgent_should_terminate_children_on_parent_terminate(t *testing.T) {
	block := make(chan struct{})
	f := newTestFactory(&blockingEngine{block: block})
	session := types.NewSession("sess_child_cancel", "cli", "/tmp")
	parent, err := f.Create(context.Background(), multiagent.AgentConfig{
		SessionID: session.SessionID,
		WorkDir:   session.WorkDir,
	}, session)
	if err != nil {
		t.Fatalf("Create parent: %v", err)
	}
	child, err := parent.Fork(context.Background(), multiagent.AgentConfig{})
	if err != nil {
		t.Fatalf("Fork: %v", err)
	}

	childDone := make(chan struct{})
	go func() {
		defer close(childDone)
		_, _ = child.Run(context.Background())
	}()
	time.Sleep(20 * time.Millisecond)

	if err := parent.Terminate(context.Background()); err != nil {
		t.Fatalf("Terminate parent: %v", err)
	}
	select {
	case <-childDone:
	case <-time.After(2 * time.Second):
		close(block)
		t.Fatal("child did not stop after parent Terminate")
	}
	if child.State() != multiagent.AgentStateTerminated {
		t.Fatalf("child State() = %s, want TERMINATED", child.State())
	}
}

func TestAgent_should_cancel_on_terminate(t *testing.T) {
	block := make(chan struct{})
	engine := &blockingEngine{block: block}
	f := newTestFactory(engine)
	session := types.NewSession("sess_term", "cli", "/tmp")
	ag, _ := f.Create(context.Background(), multiagent.AgentConfig{
		SessionID: session.SessionID,
		WorkDir:   session.WorkDir,
	}, session)

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = ag.Run(context.Background())
	}()

	time.Sleep(20 * time.Millisecond)
	if err := ag.Terminate(context.Background()); err != nil {
		t.Fatalf("Terminate: %v", err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		close(block)
		t.Fatal("Run did not finish after Terminate")
	}
	if ag.State() != multiagent.AgentStateTerminated {
		t.Fatalf("State() = %s, want TERMINATED", ag.State())
	}
}

type blockingEngine struct {
	block chan struct{}
}

func (b *blockingEngine) Process(ctx context.Context, session *types.Session, message string) <-chan *contracts.EngineEvent {
	ch := make(chan *contracts.EngineEvent)
	go func() {
		defer close(ch)
		select {
		case <-ctx.Done():
		case <-b.block:
		}
	}()
	return ch
}
