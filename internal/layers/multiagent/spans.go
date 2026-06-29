// Package multiagent — self-registers D4 MultiAgent span operations with the
// observability coverage registry on init(). The 6 D4_S4_* runtime operations
// are defined in telemetry/names.go; this file only mirrors them into the
// coverage registry so the doctor CLI can audit instrumentation.
//
// DM-20260629-004 PR-1 #0: kept at multiagent root (matching D3 llmgateway/
// spans.go pattern); orchtypes/ subpackage handles events + boundary_decision
// governance constants (see multiagent/orchtypes/).
package multiagent

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
