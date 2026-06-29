package run

import (
	"context"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/orchtypes"
	"github.com/devrix/devrix/internal/shared/types"
)

// agentPermissionGate implements contracts.IPermissionGate for a single agent.
type agentPermissionGate struct {
	agent *Impl
	mu    sync.Mutex
	pending map[string]chan bool
}

func newAgentPermissionGate(agent *Impl) *agentPermissionGate {
	return &agentPermissionGate{
		agent:   agent,
		pending: make(map[string]chan bool),
	}
}

// Request approves non-CRITICAL tools immediately; CRITICAL tools block until ResolvePermission.
func (g *agentPermissionGate) Request(
	ctx context.Context,
	_ string,
	toolName, _ string,
	risk types.RiskLevel,
) bool {
	if risk != types.RiskLevelCritical {
		return true
	}

	ch := make(chan bool, 1)
	g.mu.Lock()
	g.pending[toolName] = ch
	g.mu.Unlock()

	_ = g.agent.setState(multiagent.AgentStateWaitingPermission)
	g.agent.emit(orchtypes.EventPermissionRequired, map[string]any{"tool": toolName})

	timeout := g.agent.cfg.PermissionTimeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	select {
	case granted := <-ch:
		if granted {
			_ = g.agent.setState(multiagent.AgentStateIterating)
			return true
		}
		_ = g.agent.setState(multiagent.AgentStateTerminated)
		return false
	case <-ctx.Done():
		_ = g.agent.setState(multiagent.AgentStateTerminated)
		return false
	case <-time.After(timeout):
		_ = g.agent.setState(multiagent.AgentStateTerminated)
		return false
	}
}

// resolve writes the user decision into the pending channel.
func (g *agentPermissionGate) resolve(toolName string, granted bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if ch, ok := g.pending[toolName]; ok {
		ch <- granted
		delete(g.pending, toolName)
	}
}
