// Package orchtypes is the governance root for D4 multiagent cross-cutting
// constants: runtime span operations (coverage hook), engine event names, and
// boundary debt decisions. New constants MUST be added here instead of being
// scattered as string literals across the codebase; this keeps the D4 cross-
// domain emit/receive contract auditable from a single import path.
//
// DM-20260629-004: package promoted from multiagent/spans.go (DM-20260628
// PR-D1) and extended with events.go + boundary_decision.go governance.
package orchtypes

import "github.com/devrix/devrix/internal/layers/observability/diagnose/coverage"

func init() {
	coverage.RegisterProvider(spansProvider{})
}

type spansProvider struct{}

func (spansProvider) Spans() []coverage.OperationMeta {
	return []coverage.OperationMeta{
		// D4 MultiAgent (D4-S4)
		{Name: "D4_Agent_Run", Layer: "agent", Component: "agent_tool", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D4_Agent_Tool_Call", Layer: "agent", Component: "agent_tool", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D4_Agent_Fork", Layer: "agent", Component: "agent_tool", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D4_Agent_Join", Layer: "agent", Component: "agent_tool", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D4_Agent_Terminate", Layer: "agent", Component: "agent_tool", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D4_Agent_State_Transition", Layer: "agent", Component: "agent_tool", SinceVersion: "2.0.0", Instrumented: true},
	}
}
