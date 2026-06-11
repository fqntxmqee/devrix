package workplan_test

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/workplan"
	"github.com/devrix/devrix/internal/shared/contracts"
)

func TestWorkPlan_should_snapshot_execution_flows(t *testing.T) {
	svc := workplan.NewService(8)
	svc.Apply(contracts.FlowEvent{
		SessionID: "sess1",
		FlowID:    "w1",
		WorkerID:  "w1",
		Source:    contracts.ExecutionSourceSubQuery,
		Role:      "explore",
		Kind:      contracts.FlowStarted,
		Summary:   "start",
	})
	svc.Apply(contracts.FlowEvent{
		SessionID: "sess1",
		FlowID:    "w1",
		WorkerID:  "w1",
		Kind:      contracts.FlowToolCall,
		Summary:   "grep auth",
	})

	snap := svc.Snapshot("sess1")
	if len(snap.ExecutionFlows) != 1 {
		t.Fatalf("flows = %d", len(snap.ExecutionFlows))
	}
	if snap.ExecutionFlows[0].Status != "running" {
		t.Fatalf("status = %q", snap.ExecutionFlows[0].Status)
	}
	if len(snap.ExecutionFlows[0].RecentEvents) != 2 {
		t.Fatalf("recent = %d", len(snap.ExecutionFlows[0].RecentEvents))
	}
}
