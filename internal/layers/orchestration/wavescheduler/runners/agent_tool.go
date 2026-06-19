package runners

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/multiagent/external"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
)

// AgentToolDeps wires AgentToolRunner.
type AgentToolDeps struct {
	// Registry maps worker kind → underlying AgentTool (cursor / claude-code).
	Registry AgentToolLookup
}

// AgentToolLookup resolves agent tools by registry name.
type AgentToolLookup interface {
	Get(name string) (external.AgentTool, error)
}

// AgentToolRunner implements wavescheduler.WorkerRunner for CLI Agent Tools (cursor /
// claude-code). It uses the existing D4 external.AgentTool interface and bridges
// external.Event to wavescheduler.WorkerEvent.
type AgentToolRunner struct {
	kind wavescheduler.WorkerType
	deps AgentToolDeps
}

// NewAgentToolRunner returns a runner for the given worker kind. The kind
// determines which AgentTool is fetched from the registry; the AgentTool's
// Info().Name must equal the runner kind.
func NewAgentToolRunner(kind wavescheduler.WorkerType, deps AgentToolDeps) *AgentToolRunner {
	return &AgentToolRunner{kind: kind, deps: deps}
}

// Kind returns the runner's worker kind.
func (r *AgentToolRunner) Kind() wavescheduler.WorkerType { return r.kind }

// Run starts the AgentTool and blocks until the event channel is closed or
// ctx is cancelled. On cancel, the runner relies on the AgentTool itself
// honouring ctx — production cursor / claude-code adapters wrap
// exec.CommandContext which sends SIGTERM on ctx cancel.
func (r *AgentToolRunner) Run(ctx context.Context, spec wavescheduler.WorkerRunSpec) error {
	if r == nil {
		return fmt.Errorf("agent_tool runner: nil")
	}
	if r.deps.Registry == nil {
		return fmt.Errorf("agent_tool runner: registry is nil")
	}
	agt, err := r.deps.Registry.Get(RegistryToolName(r.kind))
	if err != nil {
		return fmt.Errorf("agent_tool runner: %w", err)
	}

	emit := spec.Emit
	if emit == nil {
		emit = func(wavescheduler.WorkerEvent) {}
	}

	workDir := spec.WorkDir
	if workDir == "" {
		workDir = "."
	}

	evtCh, err := agt.Execute(ctx, spec.SessionID, external.Request{
		Task:    spec.Directive,
		WorkDir: workDir,
	})
	if err != nil {
		emit(wavescheduler.WorkerEvent{Type: "error", Content: err.Error(), At: time.Now()})
		return err
	}

	var wg sync.WaitGroup
	wg.Add(1)
	var firstErr error
	go func() {
		defer wg.Done()
		for ev := range evtCh {
			switch ev.Type {
			case "thinking":
				emit(wavescheduler.WorkerEvent{Type: "thinking", Content: ev.Content, At: time.Now()})
			case "text", "tool_use":
				emit(wavescheduler.WorkerEvent{Type: ev.Type, Content: ev.Content, At: time.Now()})
			case "error":
				emit(wavescheduler.WorkerEvent{Type: "error", Content: ev.Content, At: time.Now()})
				if firstErr == nil {
					firstErr = fmt.Errorf("agent_tool: %s", ev.Content)
				}
			case "complete":
				emit(wavescheduler.WorkerEvent{Type: "complete", Content: ev.Content, At: time.Now()})
			default:
				// Unknown event type — pass through as text for visibility.
				emit(wavescheduler.WorkerEvent{Type: "text", Content: ev.Content, At: time.Now()})
			}
		}
	}()

	// Wait for either ctx cancel or the event channel to close.
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-ctx.Done():
		// Cursor / claude-code adapters should observe ctx and shut down.
		// Drain the event channel to avoid leaking the goroutine.
		<-done
		emit(wavescheduler.WorkerEvent{Type: "cancelled", Content: "cancelled by scheduler", At: time.Now()})
		return ctx.Err()
	case <-done:
		return firstErr
	}
}
