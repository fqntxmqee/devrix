// Package orchtypes — engine event constants.
//
// These are the canonical names emitted by D4 agents via AgentObserver.EmitAgentEvent
// (run/lifecycle.go, run/forkjoin.go) and consumed by D7 FlowEvent subscription and
// D6 evolution/guard observer. Constants live here so emit / receive stay in lockstep
// and grepping for one string yields both producer and consumer in a single hit.
//
// DM-20260629-004 PR-6 #4 span-coverage: literal strings → constants.
package orchtypes

// 7 EngineEvent 字面量治理常量（DM-20260629-004 PR-6 常量化）。
//
// Aligns with D2/D3/D7 orchtypes pattern; producer (run/lifecycle, run/forkjoin)
// and consumers (orchestration/executionflow/bridge/agent_bridge,
// evolution/guard/observer) MUST reference these constants instead of literals.
const (
	EventAgentStarted       = "agent.started"
	EventAgentError         = "agent.error"
	EventAgentTerminated    = "agent.terminated"
	EventAgentIterating     = "agent.iterating"
	EventAgentForked        = "agent.forked"
	EventAgentJoined        = "agent.joined"
	EventPermissionRequired = "permission_required"
)
