package agent_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/agent"
	"github.com/devrix/devrix/internal/layers/multiagent/factory"
	"github.com/devrix/devrix/internal/layers/multiagent/observer"
	"github.com/devrix/devrix/internal/layers/multiagent/sessionview"
	sharedconfig "github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// Covers: L5-4-3-02 (Fork metadata write does not pollute parent)
func TestFork_metadata_writes_should_not_pollute_parent_session(t *testing.T) {
	f := newTestFactory(&agent.StubEngine{Events: []*contracts.EngineEvent{{Type: "complete", Content: "ok"}}})
	session := types.NewSession("sess_meta_parent", "cli", "/tmp")
	parent, err := f.Create(context.Background(), multiagent.AgentConfig{
		SessionID: session.SessionID,
		WorkDir:   session.WorkDir,
	}, session)
	if err != nil {
		t.Fatalf("Create parent: %v", err)
	}
	// Parent agent must be a fork-aware agent (SessionView) for the new
	// isolation contract. Wire it explicitly.
	view := sessionview.Fork(session)
	parentImpl, ok := parent.(*agent.Impl)
	if !ok {
		t.Fatalf("parent is not *agent.Impl")
	}
	parentImpl.AttachSessionView(view)

	child, err := parent.Fork(context.Background(), multiagent.AgentConfig{})
	if err != nil {
		t.Fatalf("Fork: %v", err)
	}
	childImpl, ok := child.(*agent.Impl)
	if !ok {
		t.Fatalf("child is not *agent.Impl")
	}
	childView := childImpl.SessionView()
	if childView == nil {
		t.Fatal("child has no SessionView; Fork should attach one")
	}

	// Child writes metadata; parent must remain untouched.
	childView.SetMetadata("task", "child-only")
	childView.SetMetadata("step", 42)
	if _, ok := view.GetMetadata("task"); ok {
		t.Error("parent view was polluted by child writes (parent should not see child metadata)")
	}
	if v, ok := session.Metadata["task"]; ok {
		t.Errorf("parent session metadata polluted: task=%v", v)
	}
	// Child's own view must see its own writes.
	if v, ok := childView.GetMetadata("task"); !ok || v != "child-only" {
		t.Errorf("child view missing task: %v %v", v, ok)
	}
}

// Covers: L5-4-3-03 (concurrent 3 Fork + Join consistency under -race)
func TestFork_concurrent_three_children_should_join_consistently(t *testing.T) {
	f := newTestFactory(&agent.StubEngine{Events: []*contracts.EngineEvent{{Type: "complete", Content: "ok"}}})
	session := types.NewSession("sess_concurrent_3", "cli", "/tmp")
	parent, err := f.Create(context.Background(), multiagent.AgentConfig{
		SessionID:   session.SessionID,
		WorkDir:     session.WorkDir,
		MaxChildren: 3,
	}, session)
	if err != nil {
		t.Fatalf("Create parent: %v", err)
	}

	children := make([]multiagent.Agent, 0, 3)
	for i := 0; i < 3; i++ {
		c, err := parent.Fork(context.Background(), multiagent.AgentConfig{})
		if err != nil {
			t.Fatalf("Fork #%d: %v", i, err)
		}
		children = append(children, c)
	}

	// Each child writes a unique metadata key from its own view.
	// Different sleeps simulate out-of-order completion.
	for i, c := range children {
		impl := c.(*agent.Impl)
		view := impl.SessionView()
		if view == nil {
			t.Fatalf("child #%d has no SessionView", i)
		}
		view.SetMetadata("child_index", i)
	}

	var wg sync.WaitGroup
	for i, c := range children {
		wg.Add(1)
		go func(idx int, child multiagent.Agent) {
			defer wg.Done()
			// Stagger the run start to provoke out-of-order completion.
			time.Sleep(time.Duration(idx) * 10 * time.Millisecond)
			if _, err := child.Run(context.Background()); err != nil {
				t.Errorf("child %d Run: %v", idx, err)
			}
		}(i, c)
	}
	wg.Wait()

	// Join in reverse spawn order to ensure Join tolerates non-FIFO arrivals.
	for i := len(children) - 1; i >= 0; i-- {
		if err := parent.Join(context.Background(), children[i]); err != nil {
			t.Fatalf("Join child %d: %v", i, err)
		}
	}
	if got := len(parent.GetMessages()); got == 0 {
		t.Fatal("parent should have merged child messages after concurrent Join")
	}
}

// Covers: L5-4-3-01 (Join sort + tool_call ID dedup)
func TestJoin_should_dedup_tool_call_ids(t *testing.T) {
	// Two children emit `complete` with a tool_call id "call_shared".
	// The third child emits a different id "call_unique".
	// Join must dedup tool_call ids and keep exactly one entry per id.
	// Note: the key MUST be multiagent.MetaToolCallID ("tool_call_id")
	// to match the production context engine — see S4-Gate review 2026-06-12.
	mkEvents := func(callID, final string) []*contracts.EngineEvent {
		return []*contracts.EngineEvent{
			{Type: "tool_call", ToolName: "bash", Content: "echo",
				Metadata: map[string]string{multiagent.MetaToolCallID: callID}},
			{Type: "complete", Content: final},
		}
	}

	f := factory.NewAgentFactory(multiagent.AgentDeps{
		Engine:        &agent.StubEngine{Events: mkEvents("call_shared", "ok-1")},
		AgentObserver: observer.NoOpAgentObserver{},
	}, sharedconfig.DefaultMultiAgentConfig())

	session := types.NewSession("sess_dedup", "cli", "/tmp")
	parent, err := f.Create(context.Background(), multiagent.AgentConfig{
		SessionID:   session.SessionID,
		WorkDir:     session.WorkDir,
		MaxChildren: 3,
	}, session)
	if err != nil {
		t.Fatalf("Create parent: %v", err)
	}

	c1, err := parent.Fork(context.Background(), multiagent.AgentConfig{})
	if err != nil {
		t.Fatalf("Fork c1: %v", err)
	}
	c2, err := parent.Fork(context.Background(), multiagent.AgentConfig{})
	if err != nil {
		t.Fatalf("Fork c2: %v", err)
	}
	c3, err := parent.Fork(context.Background(), multiagent.AgentConfig{})
	if err != nil {
		t.Fatalf("Fork c3: %v", err)
	}

	// Override each child's engine with its own event stream.
	c1.(*agent.Impl).SetEngine(&agent.StubEngine{Events: mkEvents("call_shared", "ok-1")})
	c2.(*agent.Impl).SetEngine(&agent.StubEngine{Events: mkEvents("call_shared", "ok-2")})
	c3.(*agent.Impl).SetEngine(&agent.StubEngine{Events: mkEvents("call_unique", "ok-3")})

	var wg sync.WaitGroup
	for i, child := range []multiagent.Agent{c1, c2, c3} {
		wg.Add(1)
		go func(idx int, c multiagent.Agent) {
			defer wg.Done()
			if _, err := c.Run(context.Background()); err != nil {
				t.Errorf("child %d Run: %v", idx, err)
			}
		}(i, child)
	}
	wg.Wait()

	for _, c := range []multiagent.Agent{c1, c2, c3} {
		if err := parent.Join(context.Background(), c); err != nil {
			t.Fatalf("Join: %v", err)
		}
	}

	// Verify dedup: only one message per distinct tool_call id.
	seen := map[string]int{}
	for _, m := range parent.GetMessages() {
		if id, ok := m.Metadata[multiagent.MetaToolCallID]; ok && id != "" {
			seen[id]++
		}
	}
	if seen["call_shared"] != 1 {
		t.Errorf("call_shared seen %d times, want 1 (dedup)", seen["call_shared"])
	}
	if seen["call_unique"] != 1 {
		t.Errorf("call_unique seen %d times, want 1", seen["call_unique"])
	}
}
