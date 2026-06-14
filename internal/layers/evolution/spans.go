package evolution

import "github.com/devrix/devrix/internal/layers/observability/coverage"

func init() {
	coverage.RegisterProvider(spansProvider{})
}

type spansProvider struct{}

func (spansProvider) Spans() []coverage.OperationMeta {
	return []coverage.OperationMeta{
		// D6 Evolution - Runtime Validation (D6-S4-A01)
		{Name: "D6_S4_Validation_Decision", Layer: "evolution", Component: "validation", SinceVersion: "2.1.0", Instrumented: true},
	}
}
