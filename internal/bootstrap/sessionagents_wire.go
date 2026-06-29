package bootstrap

import "github.com/devrix/devrix/internal/bootstrap/sessionagents"

var wiredSessionAgents *sessionagents.Manager

// WireSessionAgents registers the bootstrap session agent manager (D4 leader provision).
func WireSessionAgents(m *sessionagents.Manager) {
	wiredSessionAgents = m
}

// WiredSessionAgents returns the wired manager, or nil if multi-agent is disabled.
func WiredSessionAgents() *sessionagents.Manager {
	return wiredSessionAgents
}
