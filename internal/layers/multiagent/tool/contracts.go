package tool

import "context"

// Info describes an agent tool's identity and capabilities.
type Info struct {
	Name         string
	DisplayName  string
	Description  string
	Capabilities []string
	Role         string // LLM role description for tool decision
}

// Request is the input to execute an agent tool.
type Request struct {
	Task    string
	WorkDir string
}

// Event is a streaming event from the agent tool execution.
type Event struct {
	Type    string // "text", "tool_use", "error", "complete"
	Content string
}

// AgentTool is the interface each registered tool must implement.
type AgentTool interface {
	Info() Info
	Execute(ctx context.Context, sessionID string, req Request) (<-chan Event, error)
}
