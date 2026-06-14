package execute

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/worktree"
	"github.com/devrix/devrix/internal/layers/multiagent"
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
	worktree *worktree.Manager
	observer WorkerObserver
}

// NewExecutor creates a WorkerExecutor.
func NewExecutor(cfg config.DelegateConfig, wt *worktree.Manager, obs WorkerObserver) *Executor {
	return &Executor{cfg: cfg, worktree: wt, observer: obs}
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
	child, wtPath, err := e.forkWorker(ctx, leader, spec)
	if err != nil {
		return WorkerResult{}, err
	}
	if wtPath != "" && e.worktree != nil {
		defer func() { _ = e.worktree.Exit(context.Background(), wtPath, false) }()
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
	child, wtPath, err := e.forkWorker(ctx, leader, spec)
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
		if wtPath != "" && e.worktree != nil {
			defer func() { _ = e.worktree.Exit(bgCtx, wtPath, false) }()
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
	var wtPath string
	if e.worktree != nil && e.worktree.Enabled() && strings.TrimSpace(spec.WorktreeSlug) != "" {
		path, err := e.worktree.Enter(ctx, leader.Config().SessionID, spec.WorktreeSlug, workDir)
		if err != nil {
			return nil, "", err
		}
		wtPath = path
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
		if wtPath != "" && e.worktree != nil {
			_ = e.worktree.Exit(ctx, wtPath, false)
		}
		return nil, "", err
	}
	return child, wtPath, nil
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
