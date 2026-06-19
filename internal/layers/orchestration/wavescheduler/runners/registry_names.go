package runners

import "github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"

// RegistryToolName maps wave worker kinds to agent tool registry names.
func RegistryToolName(kind wavescheduler.WorkerType) string {
	switch kind {
	case wavescheduler.WorkerClaudeCode:
		return "claude-code"
	default:
		return string(kind)
	}
}
