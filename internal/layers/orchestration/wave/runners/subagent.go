// Package runners contains WorkerRunner implementations for the WaveScheduler.
// Each runner implements wave.WorkerRunner and is responsible for translating
// a WorkerRunSpec into a real execution context (SubQuery background, AgentTool
// one-shot process, etc.) and translating streaming events back to
// wave.WorkerEvent.
package runners

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/wave"
	"github.com/devrix/devrix/internal/shared/types"
)

// SubAgentDeps wires SubAgentRunner. The actual SubQuery invocation goes
// through the package-level RunBackground hook (DM-009), which the bootstrap
// wires to the engine. We accept a function pointer so this runner does not
// import contextengine directly.
type SubAgentDeps struct {
	// Start kicks off an async SubQuery and returns the background task id
	// (compatible with BackgroundRegistry.Cancel). May be nil for tests.
	Start func(ctx context.Context, params SubAgentParams) (string, error)
	// Cancel cancels a running background task. May be nil for tests.
	Cancel func(taskID string) bool
	// IsTerminal polls the BackgroundRegistry for a terminal state. The
	// runner blocks until this returns true OR ctx is cancelled.
	IsTerminal func(taskID string) bool
	// TerminalResult returns the final result and error from the background
	// task. Called once IsTerminal reports true.
	TerminalResult func(taskID string) (result string, errMsg string, ok bool)
	// OnEvent delivers streaming events from the SubQuery. Optional.
	OnEvent func(taskID string, ev wave.WorkerEvent)
}

// SubAgentParams is what SubAgentRunner needs to start a SubQuery. It mirrors
// enforce.SubQueryParams but is local to avoid an import cycle.
type SubAgentParams struct {
	SessionID string
	AgentID   string
	AgentName string
	WorkDir   string
	Directive string
	// System / messages resolved by ContextResolver.
	SystemPrompt   string
	PromptMessages []types.Message
	// ReadOnlyTools restricts the toolset.
	ReadOnlyTools bool
	// MaxTurns caps the LLM loop.
	MaxTurns int
	// Model overrides the inherited model.
	Model     string
	ModelTier string
}

// SubAgentRunner implements wave.WorkerRunner for SubQuery-backed workers.
type SubAgentRunner struct {
	deps SubAgentDeps

	mu      sync.Mutex
	pending map[string]context.CancelFunc // taskID -> cancel func injected
}

// NewSubAgentRunner creates a SubAgent runner.
func NewSubAgentRunner(deps SubAgentDeps) *SubAgentRunner {
	if deps.OnEvent == nil {
		deps.OnEvent = func(string, wave.WorkerEvent) {}
	}
	return &SubAgentRunner{deps: deps, pending: make(map[string]context.CancelFunc)}
}

// Kind returns WorkerSubAgent.
func (r *SubAgentRunner) Kind() wave.WorkerType { return wave.WorkerSubAgent }

// Run starts a SubQuery and blocks until the background task reaches a
// terminal state OR ctx is cancelled. Cancellation comes from the ctx the
// scheduler provides — the runner wires that to BackgroundRegistry.Cancel
// via the deps.Cancel hook.
func (r *SubAgentRunner) Run(ctx context.Context, spec wave.WorkerRunSpec) error {
	if r == nil {
		return fmt.Errorf("subagent runner: nil")
	}
	if r.deps.Start == nil {
		return fmt.Errorf("subagent runner: deps.Start is nil (test stub?)")
	}
	if r.deps.IsTerminal == nil {
		return fmt.Errorf("subagent runner: deps.IsTerminal is nil")
	}

	emit := spec.Emit
	if emit == nil {
		emit = func(wave.WorkerEvent) {}
	}

	params := SubAgentParams{
		SessionID:      spec.SessionID,
		AgentID:        "wave-" + spec.TaskID,
		AgentName:      "wave/" + spec.TaskID,
		WorkDir:        spec.WorkDir,
		Directive:      spec.Directive,
		SystemPrompt:   spec.Context.SystemPrompt,
		PromptMessages: spec.Context.Messages,
		MaxTurns:       30,
		ModelTier:      spec.ModelTier,
	}

	taskID, err := r.deps.Start(ctx, params)
	if err != nil {
		emit(wave.WorkerEvent{Type: "error", Content: err.Error(), At: time.Now()})
		return err
	}
	spec.BackgroundID = taskID
	emit(wave.WorkerEvent{Type: "thinking", Content: "started", At: time.Now()})

	// Bridge ctx.Done() → BackgroundRegistry.Cancel.
	if r.deps.Cancel != nil {
		stopWatcher := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				if !r.deps.Cancel(taskID) {
					slog.Debug("subagent runner: cancel returned false (already terminal)", "task_id", taskID)
				}
			case <-stopWatcher:
			}
		}()
		defer func() {
			// Signal the watcher to exit if it hasn't already.
			select {
			case <-stopWatcher:
			default:
				close(stopWatcher)
			}
		}()
	}

	// Poll IsTerminal until done or ctx cancelled.
	ticker := time.NewTicker(30 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			emit(wave.WorkerEvent{Type: "cancelled", Content: "cancelled by scheduler", At: time.Now()})
			return ctx.Err()
		case <-ticker.C:
			if r.deps.IsTerminal(taskID) {
				if r.deps.TerminalResult != nil {
					result, errMsg, _ := r.deps.TerminalResult(taskID)
					if errMsg != "" {
						emit(wave.WorkerEvent{Type: "error", Content: errMsg, At: time.Now()})
						return fmt.Errorf("subagent: %s", errMsg)
					}
					emit(wave.WorkerEvent{Type: "text", Content: result, At: time.Now()})
				}
				emit(wave.WorkerEvent{Type: "complete", Content: "done", At: time.Now()})
				return nil
			}
		}
	}
}
