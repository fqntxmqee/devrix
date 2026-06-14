package delegate

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/worktree"
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/orchestration/flow"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionqueue"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// LeaderResolver returns the session leader agent when D4 is active.
type LeaderResolver interface {
	Leader(sessionID string) (multiagent.Agent, bool)
}

// SubQueryFallback runs L1 SubQuery when D4 delegate is unavailable.
type SubQueryFallback interface {
	RunSubQuery(ctx context.Context, parent *types.SessionContext, spec WorkerSpec) (summary string, err error)
}

// Service implements D4-S10 delegate orchestration.
//
// Deprecated: Hub-Spoke orchestration (DelegateOrFallback, FlowBridge wiring)
// has migrated to D7 hubspoke.Dispatcher (v2.0-b). The execution logic
// (forkWorker, run, join) is now in execute.Executor (D4-S14).
//
// This type is preserved for backward compatibility during the v2.0
// migration cycle and will be removed in the re-export cleanup (v2.0-e).
// New code should use hubspoke.Dispatcher for orchestration and
// execute.WorkerExecutor for execution.
//
// DSAFT: D4-S10-A01 (DelegateTask) — Legacy, canonical → D7-S2/D4-S14
type Service struct {
	cfg      config.DelegateConfig
	fallback SubQueryFallback
	worktree *worktree.Manager
	queue    *sessionqueue.SessionQueue
}

// NewService creates a DelegateService.
func NewService(cfg config.DelegateConfig, fallback SubQueryFallback, wt *worktree.Manager, q *sessionqueue.SessionQueue) *Service {
	if q == nil {
		q = sessionqueue.GlobalSessionQueue
	}
	return &Service{cfg: cfg, fallback: fallback, worktree: wt, queue: q}
}

// Enabled reports whether D4 delegate is active.
func (s *Service) Enabled() bool {
	return s != nil && s.cfg.Enabled
}

// DelegateSync forks a worker, runs it, joins, and returns the result.
func (s *Service) DelegateSync(ctx context.Context, leader multiagent.Agent, spec WorkerSpec) (DelegateResult, error) {
	if leader == nil {
		return DelegateResult{}, fmt.Errorf("delegate: leader is nil")
	}
	child, _, wtPath, err := s.forkWorker(ctx, leader, spec)
	if err != nil {
		return DelegateResult{}, err
	}
	if wtPath != "" && s.worktree != nil {
		defer func() { _ = s.worktree.Exit(context.Background(), wtPath, false) }()
	}
	result, runErr := child.Run(ctx)
	summary := extractSummary(result)
	s.publishTerminal(ctx, leader.Config().SessionID, child.ID(), spec, summary, runErr)
	if runErr != nil {
		_ = leader.Join(ctx, child)
		return DelegateResult{
			WorkerID: child.ID(),
			Role:     spec.Role,
			Summary:  summary,
			Error:    runErr,
		}, runErr
	}
	if err := leader.Join(ctx, child); err != nil {
		return DelegateResult{WorkerID: child.ID(), Role: spec.Role, Summary: summary, Error: err}, err
	}
	msgs := child.GetMessages()
	return DelegateResult{
		WorkerID: child.ID(),
		Role:     spec.Role,
		Summary:  summary,
		Messages: msgs,
	}, nil
}

// DelegateAsync starts a worker in the background; Join happens when the goroutine completes.
func (s *Service) DelegateAsync(ctx context.Context, leader multiagent.Agent, spec WorkerSpec) (string, error) {
	if !s.cfg.AllowAsync {
		return "", fmt.Errorf("delegate: async not enabled")
	}
	child, _, wtPath, err := s.forkWorker(ctx, leader, spec)
	if err != nil {
		return "", err
	}
	sessionID := leader.Config().SessionID
	go func(l multiagent.Agent) {
		bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if wtPath != "" && s.worktree != nil {
			defer func() { _ = s.worktree.Exit(bgCtx, wtPath, false) }()
		}
		result, runErr := child.Run(bgCtx)
		summary := extractSummary(result)
		s.publishTerminal(bgCtx, sessionID, child.ID(), spec, summary, runErr)
		_ = l.Join(bgCtx, child)
		s.notifyLeaderAsyncComplete(sessionID, child.ID(), spec, summary, runErr)
	}(leader)
	return child.ID(), nil
}

// DelegateOrFallback uses D4 when leader is present; otherwise SubQuery.
func (s *Service) DelegateOrFallback(
	ctx context.Context,
	leader multiagent.Agent,
	parent *types.SessionContext,
	spec WorkerSpec,
) (DelegateResult, error) {
	if s != nil && s.Enabled() && leader != nil {
		if spec.Async {
			workerID, err := s.DelegateAsync(ctx, leader, spec)
			if err != nil {
				return DelegateResult{}, err
			}
			return DelegateResult{WorkerID: workerID, Role: spec.Role, Summary: "async worker started: " + workerID}, nil
		}
		return s.DelegateSync(ctx, leader, spec)
	}
	if s == nil || s.fallback == nil || parent == nil {
		return DelegateResult{}, fmt.Errorf("delegate: no leader and no subquery fallback")
	}
	summary, err := s.fallback.RunSubQuery(ctx, parent, spec)
	return DelegateResult{Role: spec.Role, Summary: summary, Error: err}, err
}

func (s *Service) forkWorker(ctx context.Context, leader multiagent.Agent, spec WorkerSpec) (multiagent.Agent, *FlowBridge, string, error) {
	maxTurns := spec.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 50
	}
	workDir := leader.Config().WorkDir
	var wtPath string
	if s.worktree != nil && s.worktree.Enabled() && strings.TrimSpace(spec.WorktreeSlug) != "" {
		path, err := s.worktree.Enter(ctx, leader.Config().SessionID, spec.WorktreeSlug, workDir)
		if err != nil {
			return nil, nil, "", err
		}
		wtPath = path
		workDir = path
	}
	childCfg := multiagent.AgentConfig{
		SessionID:    leader.Config().SessionID,
		WorkDir:      workDir,
		InitialInput: spec.Directive,
		SystemPrompt: SystemPromptForRole(spec.Role),
		MaxIter:      maxTurns,
		MaxChildren:  0,
		WorkerRole:   string(spec.Role),
		TaskID:       spec.TaskID,
		ModelTier:    spec.ModelTier,
	}
	child, err := leader.Fork(ctx, childCfg)
	if err != nil {
		if wtPath != "" && s.worktree != nil {
			_ = s.worktree.Exit(ctx, wtPath, false)
		}
		return nil, nil, "", err
	}
	bridge := NewFlowBridge(flow.GlobalHub, childCfg.SessionID, child.ID(), child.ID(), spec.TaskID, spec.Role)
	child.SetAgentObserver(bridge)
	child.SetEngineEventSink(bridge.EngineEventSink())
	return child, bridge, wtPath, nil
}

func (s *Service) publishTerminal(
	ctx context.Context,
	sessionID, workerID string,
	spec WorkerSpec,
	summary string,
	runErr error,
) {
	if flow.GlobalHub == nil {
		return
	}
	kind := contracts.FlowCompleted
	if runErr != nil {
		kind = contracts.FlowFailed
	}
	flow.GlobalHub.Publish(ctx, contracts.FlowEvent{
		SessionID: sessionID,
		FlowID:    workerID,
		TaskID:    spec.TaskID,
		WorkerID:  workerID,
		Source:    contracts.ExecutionSourceD4Worker,
		Role:      string(spec.Role),
		Kind:      kind,
		Summary:   summary,
	})
	flow.GlobalHub.Publish(ctx, contracts.FlowEvent{
		SessionID: sessionID,
		FlowID:    workerID,
		TaskID:    spec.TaskID,
		WorkerID:  workerID,
		Source:    contracts.ExecutionSourceD4Worker,
		Role:      string(spec.Role),
		Kind:      contracts.FlowJoined,
		Summary:   summary,
	})
}

func (s *Service) notifyLeaderAsyncComplete(sessionID, workerID string, spec WorkerSpec, summary string, runErr error) {
	if s == nil || s.queue == nil || sessionID == "" {
		return
	}
	body := summary
	if runErr != nil {
		body = fmt.Sprintf("async worker %s failed: %v", workerID, runErr)
	} else if body == "" {
		body = fmt.Sprintf("async worker %s completed", workerID)
	} else {
		body = fmt.Sprintf("async worker %s (%s) completed: %s", workerID, spec.Role, summary)
	}
	s.queue.Enqueue(sessionID, contracts.QueuedCommand{
		Value: body,
		Mode:  contracts.ModeTaskNotification,
	})
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
