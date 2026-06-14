package run_test

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/run"
	
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D4-S10-A01-T03
func TestAgent_should_reject_fork_from_worker(t *testing.T) {
	f := newTestFactory(&run.StubEngine{Events: []*contracts.EngineEvent{{Type: "complete"}}})
	session := types.NewSession("sess_worker_fork", "cli", "/tmp")
	parent, err := f.Create(context.Background(), multiagent.AgentConfig{
		SessionID:   session.SessionID,
		WorkDir:     session.WorkDir,
		MaxChildren: 2,
	}, session)
	if err != nil {
		t.Fatalf("Create parent: %v", err)
	}
	worker, err := parent.Fork(context.Background(), multiagent.AgentConfig{
		WorkerRole: "explore",
	})
	if err != nil {
		t.Fatalf("Fork worker: %v", err)
	}
	if _, err := worker.Fork(context.Background(), multiagent.AgentConfig{}); err == nil {
		t.Fatal("expected worker fork to be rejected")
	}
}

// T: D4-S10-A01-T02
func TestWorkerEngine_should_inject_process_overlay(t *testing.T) {
	var gotOverlay contextengine.ProcessOverlay
	inner := &overlayCaptureEngine{capture: func(ctx context.Context) {
		if ov, ok := contextengine.ProcessOverlayFromContext(ctx); ok {
			gotOverlay = ov
		}
	}}
	wrapped := run.NewWorkerEngine(inner, multiagent.AgentConfig{
		ParentID:     "parent-1",
		WorkerRole:   "explore",
		SystemPrompt: "explore prompt",
	}, "worker-1")
	session := types.NewSession("sess_overlay", "cli", "/tmp")
	ch := wrapped.Process(context.Background(), session, "hi")
	for range ch {
	}
	if !gotOverlay.IsWorker || gotOverlay.AgentID != "worker-1" || gotOverlay.WorkerRole != "explore" {
		t.Fatalf("unexpected overlay: %+v", gotOverlay)
	}
}

type overlayCaptureEngine struct {
	capture func(context.Context)
}

func (e *overlayCaptureEngine) Process(ctx context.Context, _ *types.Session, _ string) <-chan *contracts.EngineEvent {
	if e.capture != nil {
		e.capture(ctx)
	}
	ch := make(chan *contracts.EngineEvent, 1)
	ch <- &contracts.EngineEvent{Type: "complete"}
	close(ch)
	return ch
}
