package bootstrap

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/layers/multiagent/external"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/orchestration/coordinator"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionqueue"
	"github.com/devrix/devrix/internal/layers/orchestration/wave"
	"github.com/devrix/devrix/internal/layers/orchestration/wave/runners"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// WaveSchedulerDeps wires the production WaveScheduler for OrchestratePath.
type WaveSchedulerDeps struct {
	GW         *capture.CommunicationGateway
	Engine     contracts.IEngine
	AgentTools *external.Registry
	ObsBridge  *observability.Bridge
}

// WireWaveScheduler builds a WaveScheduler with SubAgent and optional AgentTool runners.
func WireWaveScheduler(deps WaveSchedulerDeps) *wave.WaveScheduler {
	pool := wave.NewWorkerPool(wave.DefaultPoolCapacity)
	artifacts := wave.NewArtifactStore()
	runnerMap := make(map[wave.WorkerType]wave.WorkerRunner)

	if ce := contextEngineFrom(deps.Engine); ce != nil && ce.QueryLoop() != nil {
		runnerMap[wave.WorkerSubAgent] = runners.NewSubAgentRunner(buildSubAgentDeps(ce, deps.GW))
		slog.Info("d7: wave subagent runner wired")
	} else {
		slog.Warn("d7: wave subagent runner skipped (context engine loop unavailable)")
	}

	if deps.AgentTools != nil {
		for _, kind := range []wave.WorkerType{wave.WorkerCursor, wave.WorkerClaudeCode} {
			name := runners.RegistryToolName(kind)
			if _, err := deps.AgentTools.Get(name); err == nil {
				runnerMap[kind] = runners.NewAgentToolRunner(kind, runners.AgentToolDeps{
					Registry: deps.AgentTools,
				})
				slog.Info("d7: wave agent tool runner wired", "kind", kind, "tool", name)
			}
		}
	}

	sched := wave.NewWaveScheduler(wave.SchedulerDeps{
		Pool:          pool,
		Guard:         wave.NewConflictGuard(),
		Resolver:      wave.NewContextResolver(wave.ContextResolverDeps{Artifacts: artifacts}),
		Artifacts:     artifacts,
		Runners:       runnerMap,
		Observability: deps.ObsBridge,
	})
	return sched
}

// BuildOrchestratePath wires TaskDecomposer + WaveScheduler for IntentOrchestrate.
func BuildOrchestratePath(
	sink coordinator.EventPublisher,
	llmDecomp coordinator.LLMTaskDecomposer,
	deps WaveSchedulerDeps,
) *coordinator.OrchestratePath {
	decomp := coordinator.NewTaskDecomposer()
	if llmDecomp != nil {
		decomp.SetLLMDecomposer(llmDecomp)
	}
	sched := WireWaveScheduler(deps)
	op := coordinator.NewOrchestratePath(decomp, sched, sink)
	op.SetObsBridge(deps.ObsBridge)
	return op
}

func contextEngineFrom(engine contracts.IEngine) *contextengine.ContextEngine {
	if engine == nil {
		return nil
	}
	ce, _ := engine.(*contextengine.ContextEngine)
	return ce
}

func buildSubAgentDeps(ce *contextengine.ContextEngine, gw *capture.CommunicationGateway) runners.SubAgentDeps {
	reg := enforce.GlobalBackgroundRegistry
	return runners.SubAgentDeps{
		Start: func(ctx context.Context, params runners.SubAgentParams) (string, error) {
			parentSC, err := resolveParentSessionContext(ce, gw, params.SessionID)
			if err != nil {
				return "", err
			}
			promptMsgs := params.PromptMessages
			if len(promptMsgs) == 0 && params.Directive != "" {
				promptMsgs = []types.Message{{
					Role:      types.MessageRoleUser,
					Content:   params.Directive,
					SessionID: params.SessionID,
				}}
			}
			maxTurns := params.MaxTurns
			if maxTurns <= 0 {
				maxTurns = 30
			}
			return enforce.RunBackground(ctx, enforce.LoopDeps{Loop: ce.QueryLoop()}, enforce.SubQueryParams{
				ParentSC:       parentSC,
				AgentID:        params.AgentID,
				AgentName:      params.AgentName,
				SystemPrompt:   params.SystemPrompt,
				PromptMessages: promptMsgs,
				MaxTurns:       maxTurns,
				ModelTier:      params.ModelTier,
				ReadOnlyTools:  params.ReadOnlyTools,
			}, reg, sessionqueue.NewSessionQueue())
		},
		Cancel: func(taskID string) bool {
			if reg == nil {
				return false
			}
			return reg.Cancel(taskID)
		},
		IsTerminal: func(taskID string) bool {
			if reg == nil {
				return true
			}
			return reg.IsTerminal(taskID)
		},
		TerminalResult: func(taskID string) (string, string, bool) {
			if reg == nil {
				return "", "background registry unavailable", false
			}
			task, ok := reg.Get(taskID)
			if !ok {
				return "", fmt.Sprintf("unknown background task %q", taskID), false
			}
			return task.Result, task.Error, true
		},
	}
}

func resolveParentSessionContext(
	ce *contextengine.ContextEngine,
	gw *capture.CommunicationGateway,
	sessionID string,
) (*types.SessionContext, error) {
	if ce != nil {
		if sc, ok := ce.SessionContext(sessionID); ok && sc != nil {
			return sc, nil
		}
	}
	if gw != nil {
		sess, err := gw.GetSession(sessionID)
		if err == nil && sess != nil {
			return &types.SessionContext{
				SessionID: sessionID,
				WorkDir:   sess.WorkDir,
				Model:     sess.Model,
			}, nil
		}
	}
	return nil, fmt.Errorf("session context not found: %s", sessionID)
}
