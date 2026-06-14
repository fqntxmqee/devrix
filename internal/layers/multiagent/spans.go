package multiagent

import "github.com/devrix/devrix/internal/layers/observability/coverage"

func init() {
	coverage.RegisterProvider(spansProvider{})
}

type spansProvider struct{}

func (spansProvider) Spans() []coverage.OperationMeta {
	return []coverage.OperationMeta{
		// D4 MultiAgent (D4-S4)
		{Name: "D4_S4_Agent_Run", Layer: "agent", Component: "agent_tool", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D4_S4_Agent_Tool_Call", Layer: "agent", Component: "agent_tool", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D4_S4_Agent_Fork", Layer: "agent", Component: "agent_tool", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D4_S4_Agent_Join", Layer: "agent", Component: "agent_tool", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D4_S4_Agent_Terminate", Layer: "agent", Component: "agent_tool", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D4_S4_Agent_State_Transition", Layer: "agent", Component: "agent_tool", SinceVersion: "2.0.0", Instrumented: true},
	}
}
