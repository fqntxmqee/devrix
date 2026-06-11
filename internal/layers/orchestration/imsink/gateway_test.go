package imsink_test

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/imsink"
	"github.com/devrix/devrix/internal/shared/contracts"
)

type captureSink struct {
	last *contracts.EngineEvent
}

func (c *captureSink) Emit(ev *contracts.EngineEvent) {
	c.last = ev
}

// Covers: L5-4-10-05
func TestGatewaySink_should_emit_worker_progress_with_task_id(t *testing.T) {
	sink := &captureSink{}
	gw := imsink.NewGatewaySink(sink)
	gw.EmitWorkerProgress(contracts.FlowEvent{
		SessionID: "sess1",
		FlowID:    "w1",
		WorkerID:  "w1",
		TaskID:    "task_abc",
		Source:    contracts.ExecutionSourceSubQuery,
		Role:      "explore",
		Kind:      contracts.FlowStarted,
		Summary:   "started",
	})
	if sink.last == nil {
		t.Fatal("expected engine event")
	}
	if sink.last.Type != "worker_progress" {
		t.Fatalf("type = %q", sink.last.Type)
	}
	if sink.last.Metadata["task_id"] != "task_abc" {
		t.Fatalf("metadata = %+v", sink.last.Metadata)
	}
}
