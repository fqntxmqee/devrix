package runners

import "github.com/devrix/devrix/internal/layers/orchestration/wave"

// RegistryToolName maps wave worker kinds to agent tool registry names.
func RegistryToolName(kind wave.WorkerType) string {
	switch kind {
	case wave.WorkerClaudeCode:
		return "claude-code"
	default:
		return string(kind)
	}
}
