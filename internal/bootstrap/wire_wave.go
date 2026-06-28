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
	"github.com/devrix/devrix/internal/layers/orchestration/executionflow"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler/runners"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// WaveSchedulerDeps wires the production WaveScheduler (tests / background workers).
type WaveSchedulerDeps struct {
	GW         *capture.CommunicationGateway
	Engine     contracts.IEngine
	AgentTools *external.Registry
	ObsBridge  *observability.Bridge
}

// WireWaveScheduler builds a WaveScheduler with SubAgent and optional AgentTool runners.
func WireWaveScheduler(deps WaveSchedulerDeps) *wavescheduler.WaveScheduler {
	pool := wavescheduler.NewWorkerPool(wavescheduler.DefaultPoolCapacity)
	artifacts := wavescheduler.NewArtifactStore()
	runnerMap := make(map[wavescheduler.WorkerType]wavescheduler.WorkerRunner)

	if WiredSubTurn() != nil || contextEngineFrom(deps.Engine) != nil {
		runnerMap[wavescheduler.WorkerSubAgent] = runners.NewSubAgentRunner(buildSubAgentDeps(deps.GW, deps.Engine))
		slog.Info("d7: wave subagent runner wired")
	} else {
		slog.Warn("d7: wave subagent runner skipped (SubTurn executor unavailable)")
	}

	if deps.AgentTools != nil {
		for _, kind := range []wavescheduler.WorkerType{wavescheduler.WorkerCursor, wavescheduler.WorkerClaudeCode} {
			name := runners.RegistryToolName(kind)
			if _, err := deps.AgentTools.Get(name); err == nil {
				runnerMap[kind] = runners.NewAgentToolRunner(kind, runners.AgentToolDeps{
					Registry: deps.AgentTools,
				})
				slog.Info("d7: wave agent tool runner wired", "kind", kind, "tool", name)
			}
		}
	}

	sched := wavescheduler.NewWaveScheduler(wavescheduler.SchedulerDeps{
		Pool:          pool,
		Guard:         wavescheduler.NewConflictGuard(),
		Resolver: wavescheduler.NewMaterializingContextResolver(wavescheduler.ContextResolverDeps{
			Artifacts:    artifacts,
			Materializer: newDefaultMaterializer(),
		}),
		Artifacts:     artifacts,
		Runners:       runnerMap,
		Observability: deps.ObsBridge,
	})
	return sched
}

func contextEngineFrom(engine contracts.IEngine) *contextengine.ContextEngine {
	if engine == nil {
		return nil
	}
	ce, _ := engine.(*contextengine.ContextEngine)
	return ce
}

func buildSubAgentDeps(gw *capture.CommunicationGateway, engine contracts.IEngine) runners.SubAgentDeps {
	reg := enforce.GlobalBackgroundRegistry
	ce := contextEngineFrom(engine)
	return runners.SubAgentDeps{
		Start: func(ctx context.Context, params runners.SubAgentParams) (string, error) {
			subTurn := WiredSubTurn()
			if subTurn == nil {
				return "", fmt.Errorf("wave subagent: SubTurn executor not wired")
			}
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
			return enforce.RunBackground(ctx, enforce.SubQueryDeps{
				SubTurn: subTurn,
			}, enforce.SubQueryParams{
				ParentSC:       parentSC,
				AgentID:        params.AgentID,
				AgentName:      params.AgentName,
				SystemPrompt:   params.SystemPrompt,
				PromptMessages: promptMsgs,
				MaxTurns:       maxTurns,
				ModelTier:      params.ModelTier,
				ReadOnlyTools:  params.ReadOnlyTools,
				// DM-20260626-002 — forward streaming Emit so the LLM
				// loop's per-event streams reach the worker channel
				// (and feishu card) in real time.
				Emit: params.Emit,
			}, reg, executionflow.NewSessionQueue())
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
