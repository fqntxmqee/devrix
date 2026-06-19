// Package hubspoke is a legacy shim. Dispatch lives in sessionorchestrator; bridges in executionflow/bridge.
package hubspoke

import (
	"github.com/devrix/devrix/internal/layers/orchestration/executionflow/bridge"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
)

type (
	Dispatcher      = sessionorchestrator.Dispatcher
	DispatchRequest = sessionorchestrator.DispatchRequest
	DispatchResult  = sessionorchestrator.DispatchResult
	LeaderResolver  = sessionorchestrator.LeaderResolver
	SubQueryRunner  = sessionorchestrator.SubQueryRunner
	AgentBridge     = bridge.AgentBridge
	SubQueryBridge  = bridge.SubQueryBridge
	FlowReporter    = bridge.FlowReporter
)

var (
	NewDispatcher     = sessionorchestrator.NewDispatcher
	NewAgentBridge    = bridge.NewAgentBridge
	NewSubQueryBridge = bridge.NewSubQueryBridge
	NewFlowReporter   = bridge.NewFlowReporter
)
