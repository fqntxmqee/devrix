package execute

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// WorkerObserver is a callback interface so D7 hubspoke can wire
// its own FlowBridge without the execute package importing flow.
type WorkerObserver interface {
	OnWorkerForked(workerID, sessionID string, agent multiagent.Agent)
	OnWorkerCompleted(workerID, sessionID string, summary string, runErr error)
}

// Executor implements WorkerExecutor using the agent fork→run→join lifecycle.
//
// DSAFT: D4-S14-A01 (ExecuteWorker)
type Executor struct {
	cfg      config.DelegateConfig
	sandbox contracts.WorkerDirSandbox
	observer WorkerObserver

	// metrics is an optional counter sink for sandbox cleanup failures.
	// DM-20260621-010 PR-B: nil-safe; non-nil counters are incremented
	// when sandbox.Exit returns an error during cleanup paths.
	metrics *ExecutorMetrics
}

// NewExecutor creates a WorkerExecutor.
func NewExecutor(cfg config.DelegateConfig, sb contracts.WorkerDirSandbox, obs WorkerObserver) *Executor {
	return &Executor{cfg: cfg, sandbox: sb, observer: obs}
}

// WithMetrics attaches a metrics sink. Backward-compatible setter for
// callers that constructed Executor via NewExecutor; safe to call
// before any ExecuteSync/ExecuteAsync. nil disables metric recording.
func (e *Executor) WithMetrics(m *ExecutorMetrics) *Executor {
	e.metrics = m
	return e
}

// recordSandboxExitFailed is the single emission point for sandbox cleanup
// failures; combines counter + slog.Warn.
func (e *Executor) recordSandboxExitFailed(where, sessionID, sandboxPath string, err error) {
	if e.metrics != nil {
		e.metrics.SandboxExitFailed.Add(1)
	}
	slog.Warn("execute: sandbox exit failed",
		"where", where,
		"sessionID", sessionID,
		"sandboxPath", sandboxPath,
		"err", err,
		"metric", "sandbox_exit_failed")
}

// ExecuteSync forks a worker, runs it synchronously, joins, and returns the result.
func (e *Executor) resolveObserver(spec WorkerRunSpec) WorkerObserver {
	if spec.Observer != nil {
		return spec.Observer
	}
	return e.observer
}

func (e *Executor) ExecuteSync(ctx context.Context, leader multiagent.Agent, spec WorkerRunSpec) (WorkerResult, error) {
	if leader == nil {
		return WorkerResult{}, fmt.Errorf("execute: leader is nil")
	}
	child, sbPath, err := e.forkWorker(ctx, leader, spec)
	if err != nil {
		return WorkerResult{}, err
	}
	if sbPath != "" && e.sandbox != nil {
		defer func() {
			if err := e.sandbox.Exit(context.Background(), sbPath, false); err != nil {
				e.recordSandboxExitFailed("ExecuteSync", leader.Config().SessionID, sbPath, err)
			}
		}()
	}

	obs := e.resolveObserver(spec)
	if obs != nil {
		obs.OnWorkerForked(child.ID(), leader.Config().SessionID, child)
	}

	result, runErr := child.Run(ctx)
	summary := extractSummary(result)

	if obs != nil {
		obs.OnWorkerCompleted(child.ID(), leader.Config().SessionID, summary, runErr)
	}

	if runErr != nil {
		_ = leader.Join(ctx, child)
		return WorkerResult{
			WorkerID: child.ID(),
			Summary:  summary,
			Error:    runErr,
		}, runErr
	}
	if err := leader.Join(ctx, child); err != nil {
		return WorkerResult{WorkerID: child.ID(), Summary: summary, Error: err}, err
	}
	return WorkerResult{
		WorkerID: child.ID(),
		Summary:  summary,
		Messages: child.GetMessages(),
	}, nil
}

// ExecuteAsync starts a worker in the background; Join happens when the goroutine completes.
func (e *Executor) ExecuteAsync(ctx context.Context, leader multiagent.Agent, spec WorkerRunSpec) (string, error) {
	if !e.cfg.AllowAsync {
		return "", fmt.Errorf("execute: async not enabled")
	}
	child, sbPath, err := e.forkWorker(ctx, leader, spec)
	if err != nil {
		return "", err
	}

	obs := e.resolveObserver(spec)
	if obs != nil {
		obs.OnWorkerForked(child.ID(), leader.Config().SessionID, child)
	}

	sessionID := leader.Config().SessionID
	go func(l multiagent.Agent) {
		bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if sbPath != "" && e.sandbox != nil {
			defer func() {
				if err := e.sandbox.Exit(bgCtx, sbPath, false); err != nil {
					e.recordSandboxExitFailed("ExecuteAsync", sessionID, sbPath, err)
				}
			}()
		}
		result, runErr := child.Run(bgCtx)
		summary := extractSummary(result)

		if obs != nil {
			obs.OnWorkerCompleted(child.ID(), sessionID, summary, runErr)
		}

		_ = l.Join(bgCtx, child)
	}(leader)
	return child.ID(), nil
}

func (e *Executor) forkWorker(ctx context.Context, leader multiagent.Agent, spec WorkerRunSpec) (multiagent.Agent, string, error) {
	maxTurns := spec.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 50
	}
	workDir := leader.Config().WorkDir
	var sbPath string
	if e.sandbox != nil && e.sandbox.Enabled() && strings.TrimSpace(spec.SandboxSlug) != "" {
		path, err := e.sandbox.Enter(ctx, leader.Config().SessionID, spec.SandboxSlug, workDir)
		if err != nil {
			return nil, "", err
		}
		sbPath = path
		workDir = path
	}
	childCfg := multiagent.AgentConfig{
		SessionID:    leader.Config().SessionID,
		WorkDir:      workDir,
		InitialInput: spec.Directive,
		SystemPrompt: systemPromptForRole(spec.Role),
		MaxIter:      maxTurns,
		MaxChildren:  0,
		WorkerRole:   spec.Role,
		TaskID:       spec.TaskID,
		ModelTier:    spec.ModelTier,
	}
	child, err := leader.Fork(ctx, childCfg)
	if err != nil {
		if sbPath != "" && e.sandbox != nil {
			if exitErr := e.sandbox.Exit(ctx, sbPath, false); exitErr != nil {
				e.recordSandboxExitFailed("forkWorker", leader.Config().SessionID, sbPath, exitErr)
			}
		}
		return nil, "", err
	}
	return child, sbPath, nil
}

func systemPromptForRole(role string) string {
	switch role {
	case "explore":
		return explorePrompt
	case "plan":
		return planPrompt
	default:
		return implementPrompt
	}
}

func extractSummary(result *multiagent.AgentResult) string {
	if result == nil {
		return ""
	}
	if result.Error != nil {
		return result.Error.Error()
	}
	for i := len(result.Messages) - 1; i >= 0; i-- {
		if result.Messages[i].Role == types.MessageRoleAssistant {
			if c := strings.TrimSpace(result.Messages[i].Content); c != "" {
				return c
			}
		}
	}
	for i := len(result.Messages) - 1; i >= 0; i-- {
		if result.Messages[i].Role == types.MessageRoleTool {
			if c := strings.TrimSpace(result.Messages[i].Content); c != "" {
				if len(c) > 4000 {
					c = c[:4000] + "…"
				}
				return c
			}
		}
	}
	for i := len(result.Messages) - 1; i >= 0; i-- {
		if c := strings.TrimSpace(result.Messages[i].Content); c != "" {
			return c
		}
	}
	return ""
}
