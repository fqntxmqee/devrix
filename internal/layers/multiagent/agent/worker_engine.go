package agent

import (
	"context"

	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// WorkerEngine wraps an IEngine and injects D4 worker session context per Process.
type WorkerEngine struct {
	inner        contracts.IEngine
	agentID      string
	workerRole   string
	systemPrompt string
}

// NewWorkerEngine wraps inner when the agent is a forked worker.
func NewWorkerEngine(inner contracts.IEngine, cfg multiagent.AgentConfig, agentID string) contracts.IEngine {
	if inner == nil || cfg.ParentID == "" {
		return inner
	}
	return &WorkerEngine{
		inner:        inner,
		agentID:      agentID,
		workerRole:   cfg.WorkerRole,
		systemPrompt: cfg.SystemPrompt,
	}
}

// Process implements contracts.IEngine.
func (w *WorkerEngine) Process(ctx context.Context, session *types.Session, message string) <-chan *contracts.EngineEvent {
	ov := contextengine.ProcessOverlay{
		AgentID:      w.agentID,
		IsWorker:     true,
		WorkerRole:   w.workerRole,
		SystemPrompt: w.systemPrompt,
	}
	return w.inner.Process(contextengine.WithProcessOverlay(ctx, ov), session, message)
}
