package llmgateway

import "github.com/devrix/devrix/internal/layers/observability/diagnose/coverage"

func init() {
	coverage.RegisterProvider(spansProvider{})
}

type spansProvider struct{}

func (spansProvider) Spans() []coverage.OperationMeta {
	return []coverage.OperationMeta{
		// D3 LLM Gateway (D3-S3)
		{Name: "D3_LLM_Stream", Layer: "llm", Component: "llm_gateway", SinceVersion: "1.2.0", Instrumented: true},
		{Name: "D3_LLM_Provider_Route", Layer: "llm", Component: "llm_gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D3_LLM_CircuitBreaker", Layer: "llm", Component: "llm_gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D3_LLM_Retry", Layer: "llm", Component: "llm_gateway", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D3_LLM_Adapter_Stream", Layer: "llm", Component: "llm_adapter", SinceVersion: "2.0.0", Instrumented: true},
		{Name: "D3_LLM_Stream_Consume", Layer: "llm", Component: "llm_gateway", SinceVersion: "3.3.0", Instrumented: true},
	}
}
