package orchestration

import "github.com/devrix/devrix/internal/shared/config"

// OrchestrationConfig is the runtime configuration for the orchestration validator.
type OrchestrationConfig = config.OrchestrationConfig

// DefaultOrchestrationConfig returns sensible defaults.
func DefaultOrchestrationConfig() OrchestrationConfig {
	return config.DefaultOrchestrationConfig()
}
