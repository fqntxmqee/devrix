package contextengine

import "github.com/devrix/devrix/internal/layers/observability"

// LLMLogSettings controls LLM content capture for tracing and local files.
type LLMLogSettings = observability.LLMLogSettings

// ConfigureLLMLogging applies observability.llm settings at process startup.
func ConfigureLLMLogging(settings LLMLogSettings) {
	observability.ConfigureLLMLogging(settings)
}

func currentLLMLogSettings() observability.LLMLogSettings {
	return observability.CurrentLLMLogSettings()
}
