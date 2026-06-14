package run_test

import (
	"context"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/run"
	
	"github.com/devrix/devrix/internal/layers/multiagent/observer"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D4-S2-A02-T02, D4-S2-A02-T03
func TestAgentPermissionGate_should_auto_approve_non_critical(t *testing.T) {
	a := run.New(multiagent.AgentConfig{
		SessionID:         "sess_perm",
		WorkDir:           "/tmp",
		PermissionTimeout: time.Second,
	}, types.NewSession("sess_perm", "cli", "/tmp"), multiagent.AgentDeps{
		Engine:        &run.StubEngine{},
		AgentObserver: observer.NoOpAgentObserver{},
	}, nil)

	gate := a.PermissionGate()
	granted := gate.Request(context.Background(), "sess_perm", "read_file", "{}", types.RiskLevelLow)
	if !granted {
		t.Fatal("expected non-CRITICAL tool to be auto-approved")
	}
}

func TestAgentPermissionGate_should_grant_on_resolve(t *testing.T) {
	a := run.New(multiagent.AgentConfig{
		SessionID:         "sess_grant",
		WorkDir:           "/tmp",
		PermissionTimeout: 2 * time.Second,
	}, types.NewSession("sess_grant", "cli", "/tmp"), multiagent.AgentDeps{
		Engine:        &run.StubEngine{},
		AgentObserver: observer.NoOpAgentObserver{},
	}, nil)

	gate := a.PermissionGate()
	done := make(chan bool, 1)
	go func() {
		done <- gate.Request(context.Background(), "sess_grant", "bash", "rm -rf /", types.RiskLevelCritical)
	}()

	time.Sleep(30 * time.Millisecond)
	a.ResolvePermission("bash", true)

	select {
	case granted := <-done:
		if !granted {
			t.Fatal("expected permission granted")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for permission gate")
	}
}

func TestAgentPermissionGate_should_deny_on_resolve_false(t *testing.T) {
	a := run.New(multiagent.AgentConfig{
		SessionID:         "sess_deny",
		WorkDir:           "/tmp",
		PermissionTimeout: 2 * time.Second,
	}, types.NewSession("sess_deny", "cli", "/tmp"), multiagent.AgentDeps{
		Engine:        &run.StubEngine{},
		AgentObserver: observer.NoOpAgentObserver{},
	}, nil)

	gate := a.PermissionGate()
	done := make(chan bool, 1)
	go func() {
		done <- gate.Request(context.Background(), "sess_deny", "bash", "danger", types.RiskLevelCritical)
	}()

	time.Sleep(30 * time.Millisecond)
	a.ResolvePermission("bash", false)

	select {
	case granted := <-done:
		if granted {
			t.Fatal("expected permission denied")
		}
		if a.State() != multiagent.AgentStateTerminated {
			t.Fatalf("state = %s, want TERMINATED", a.State())
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for permission gate")
	}
}

func TestAgentPermissionGate_should_timeout_when_unresolved(t *testing.T) {
	a := run.New(multiagent.AgentConfig{
		SessionID:         "sess_timeout",
		WorkDir:           "/tmp",
		PermissionTimeout: 50 * time.Millisecond,
	}, types.NewSession("sess_timeout", "cli", "/tmp"), multiagent.AgentDeps{
		Engine:        &run.StubEngine{},
		AgentObserver: observer.NoOpAgentObserver{},
	}, nil)

	gate := a.PermissionGate()
	granted := gate.Request(context.Background(), "sess_timeout", "bash", "x", types.RiskLevelCritical)
	if granted {
		t.Fatal("expected timeout to deny permission")
	}
}
