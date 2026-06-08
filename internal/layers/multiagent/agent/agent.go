package agent

import (
	"context"
	"sync"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/observer"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/google/uuid"
)

// Creator creates child agents without importing the factory package.
type Creator interface {
	Create(ctx context.Context, cfg multiagent.AgentConfig, session *types.Session) (multiagent.Agent, error)
}

// Impl is the concrete Agent implementation.
type Impl struct {
	id          string
	state       multiagent.AgentState
	cfg         multiagent.AgentConfig
	session     *types.Session
	deps        multiagent.AgentDeps
	creator     Creator
	childAgents map[string]multiagent.Agent
	joinedMsgs  []types.Message
	result      *multiagent.AgentResult
	done        chan struct{}
	doneOnce    sync.Once

	mu     sync.RWMutex
	cancel context.CancelFunc
}

var _ multiagent.Agent = (*Impl)(nil)

// New constructs an agent in CREATED state.
func New(
	cfg multiagent.AgentConfig,
	session *types.Session,
	deps multiagent.AgentDeps,
	creator Creator,
) *Impl {
	if deps.AgentObserver == nil {
		deps.AgentObserver = observer.NoOpAgentObserver{}
	}
	return &Impl{
		id:          uuid.New().String(),
		state:       multiagent.AgentStateCreated,
		cfg:         cfg,
		session:     session,
		deps:        deps,
		creator:     creator,
		childAgents: make(map[string]multiagent.Agent),
		done:        make(chan struct{}),
	}
}

// ID returns the agent identifier.
func (a *Impl) ID() string {
	return a.id
}

// State returns the current lifecycle state.
func (a *Impl) State() multiagent.AgentState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

// Config returns a copy of the agent configuration.
func (a *Impl) Config() multiagent.AgentConfig {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cfg
}

func (a *Impl) setState(to multiagent.AgentState) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := transition(a.state, to); err != nil {
		if a.state == multiagent.AgentStateTerminated {
			return sharedTerminated(a.id)
		}
		return err
	}
	a.state = to
	return nil
}

func (a *Impl) emit(eventType string, metadata map[string]any) {
	a.deps.AgentObserver.EmitAgentEvent(multiagent.AgentEvent{
		AgentID:   a.id,
		ParentID:  a.cfg.ParentID,
		SessionID: a.cfg.SessionID,
		EventType: eventType,
		State:     a.State(),
		Mode:      a.cfg.Mode,
		Metadata:  metadata,
	})
}

func (a *Impl) finishResult(result *multiagent.AgentResult) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.state == multiagent.AgentStateTerminated {
		return
	}
	a.result = result
	a.state = multiagent.AgentStateTerminated
	a.doneOnce.Do(func() { close(a.done) })
}

func (a *Impl) activeChildCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.childAgents)
}

func (a *Impl) addChild(child multiagent.Agent) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.childAgents[child.ID()] = child
}

func (a *Impl) removeChild(childID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.childAgents, childID)
}

func (a *Impl) getChild(childID string) (multiagent.Agent, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	child, ok := a.childAgents[childID]
	return child, ok
}

func (a *Impl) terminateChildren(ctx context.Context) {
	a.mu.RLock()
	children := make([]multiagent.Agent, 0, len(a.childAgents))
	for _, c := range a.childAgents {
		children = append(children, c)
	}
	a.mu.RUnlock()
	for _, child := range children {
		_ = child.Terminate(ctx)
	}
}
