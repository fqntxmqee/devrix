// Package runners contains WorkerRunner implementations for the WaveScheduler.
// Each runner implements wavescheduler.WorkerRunner and is responsible for translating
// a WorkerRunSpec into a real execution context (SubQuery background, AgentTool
// one-shot process, etc.) and translating streaming events back to
// wavescheduler.WorkerEvent.
package runners

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/shared/contracts"
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
	OnEvent func(taskID string, ev wavescheduler.WorkerEvent)
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
	// Emit forwards per-event streams from the SubQuery loop back to the
	// worker channel (DM-20260626-002). EngineEvent types are translated
	// to WorkerEvent types by the runner; nil = no streaming.
	Emit contracts.EngineEmitFunc
}

// SubAgentRunner implements wavescheduler.WorkerRunner for SubQuery-backed workers.
type SubAgentRunner struct {
	deps SubAgentDeps

	mu      sync.Mutex
	pending map[string]context.CancelFunc // taskID -> cancel func injected
}

// NewSubAgentRunner creates a SubAgent runner.
func NewSubAgentRunner(deps SubAgentDeps) *SubAgentRunner {
	if deps.OnEvent == nil {
		deps.OnEvent = func(string, wavescheduler.WorkerEvent) {}
	}
	return &SubAgentRunner{deps: deps, pending: make(map[string]context.CancelFunc)}
}

// Kind returns WorkerSubAgent.
func (r *SubAgentRunner) Kind() wavescheduler.WorkerType { return wavescheduler.WorkerSubAgent }

// Run starts a SubQuery and blocks until the background task reaches a
// terminal state OR ctx is cancelled. Cancellation comes from the ctx the
// scheduler provides — the runner wires that to BackgroundRegistry.Cancel
// via the deps.Cancel hook.
func (r *SubAgentRunner) Run(ctx context.Context, spec wavescheduler.WorkerRunSpec) error {
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
		emit = func(wavescheduler.WorkerEvent) {}
	}

	// Forward every SubQuery loop event to the worker channel so the
	// OrchestratePath fan-out (workerEventToEngine) and the feishu card
	// renderer see the LLM stream in real time. The engine emits
	// "text"/"thinking"/"tool_call"/"error" — we map them to worker
	// semantics: tool_call → tool_use, others stay as-is.
	streamEmit := func(ev *contracts.EngineEvent) {
		if ev == nil || spec.Emit == nil {
			return
		}
		var workerType string
		switch ev.Type {
		case "thinking":
			workerType = "thinking"
		case "text":
			workerType = "text"
		case "tool_call":
			workerType = "tool_use"
		case "error":
			workerType = "error"
		default:
			// "complete" / "info" / unknown: skip — the worker will
			// emit its own "complete" when the loop drains.
			return
		}
		spec.Emit(wavescheduler.WorkerEvent{Type: workerType, Content: ev.Content, At: time.Now()})
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
		Emit:           streamEmit,
	}

	taskID, err := r.deps.Start(ctx, params)
	if err != nil {
		emit(wavescheduler.WorkerEvent{Type: "error", Content: err.Error(), At: time.Now()})
		return err
	}
	spec.BackgroundID = taskID

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
			emit(wavescheduler.WorkerEvent{Type: "cancelled", Content: "cancelled by scheduler", At: time.Now()})
			return ctx.Err()
		case <-ticker.C:
			if r.deps.IsTerminal(taskID) {
				if r.deps.TerminalResult != nil {
					_, errMsg, _ := r.deps.TerminalResult(taskID)
					if errMsg != "" {
						emit(wavescheduler.WorkerEvent{Type: "error", Content: errMsg, At: time.Now()})
						return fmt.Errorf("subagent: %s", errMsg)
					}
				}
				// Streaming text already reached the feishu reply card via
				// SubQuery.Run's Emit (DM-20260626-002). Re-emitting the
				// terminal result here would duplicate the entire LLM
				// response on the user's card when the early-stage replay
				// dedup in feishu.appendResponseText hasn't accumulated
				// enough buffer runes to detect the overlap (see 2026-06-26
				// hotfix). The BackgroundRegistry still stores the full
				// result for post-mortem / cross-session reuse, but the
				// worker channel only sees a terminal "complete" event.
				emit(wavescheduler.WorkerEvent{Type: "complete", Content: "done", At: time.Now()})
				return nil
			}
		}
	}
}
