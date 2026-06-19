package run

import (
	"context"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/kernel"
	"github.com/devrix/devrix/internal/layers/multiagent/isolate"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/google/uuid"
)

// Creator creates child agents and tracks session-level quotas.
type Creator interface {
	Create(ctx context.Context, cfg multiagent.AgentConfig, session *types.Session) (multiagent.Agent, error)
	ReleaseSession(sessionID string)
}

// Impl is the concrete Agent implementation.
type Impl struct {
	id              string
	state           multiagent.AgentState
	cfg             multiagent.AgentConfig
	session         *types.Session
	view            *isolate.View // DM-20260611-005: fork-isolated metadata view
	deps            multiagent.AgentDeps
	creator         Creator
	permGate        *agentPermissionGate
	engine          contracts.IEngine
	engineEventSink func(*contracts.EngineEvent)
	childAgents     map[string]multiagent.Agent
	messageBuffer   []types.Message
	joinedToolIDs   map[string]struct{} // DM-20260611-005: dedup state for Join
	result          *multiagent.AgentResult
	finishedAt      time.Time // captured by finishResult; used by Join sort
	done            chan struct{}
	doneOnce        sync.Once

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
		deps.AgentObserver = kernel.NoOpAgentObserver{}
	}
	deps.AgentObserver = multiagent.NewAgentObserverChain(deps.AgentObserver)
	a := &Impl{
		id:            uuid.New().String(),
		state:         multiagent.AgentStateCreated,
		cfg:           cfg,
		session:       session,
		view:          isolate.Fork(session),
		deps:          deps,
		creator:       creator,
		childAgents:   make(map[string]multiagent.Agent),
		messageBuffer: make([]types.Message, 0),
		joinedToolIDs: make(map[string]struct{}, 4),
		done:          make(chan struct{}),
	}
	a.permGate = newAgentPermissionGate(a)
	return a
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

// GetMessages returns a copy of the agent message buffer.
func (a *Impl) GetMessages() []types.Message {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]types.Message, len(a.messageBuffer))
	copy(out, a.messageBuffer)
	return out
}

// ResolvePermission injects the user permission decision for a pending CRITICAL tool.
func (a *Impl) ResolvePermission(toolName string, granted bool) {
	if a.permGate != nil {
		a.permGate.resolve(toolName, granted)
	}
}

func (a *Impl) appendMessages(msgs ...types.Message) {
	if len(msgs) == 0 {
		return
	}
	a.mu.Lock()
	a.messageBuffer = append(a.messageBuffer, msgs...)
	a.mu.Unlock()
}

func (a *Impl) setState(to multiagent.AgentState) error {
	a.mu.Lock()
	if err := transition(a.state, to); err != nil {
		a.mu.Unlock()
		if a.state == multiagent.AgentStateTerminated {
			return sharedTerminated(a.id)
		}
		return err
	}
	from := a.state
	a.state = to
	a.mu.Unlock()

	// Non-blocking state transition trace.
	if a.deps.ObsBridge != nil && a.deps.ObsBridge.Tracer() != nil {
		_, stSpan := a.deps.ObsBridge.Tracer().Start(context.Background(), telemetry.OpD4_S4_Agent_State_Transition,
			tracer.WithSpanKind(tracer.SpanKindInternal),
			tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpD4_S4_Agent_State_Transition,
				tracer.Attribute{Key: "agent.id", Value: a.id},
				tracer.Attribute{Key: "from", Value: from.String()},
				tracer.Attribute{Key: "to", Value: to.String()},
			)...),
		)
		stSpan.End()
	}
	return nil
}

func (a *Impl) emit(eventType string, metadata map[string]any) {
	a.deps.AgentObserver.EmitAgentEvent(multiagent.AgentEvent{
		AgentID:   a.id,
		ParentID:  a.cfg.ParentID,
		SessionID: a.cfg.SessionID,
		EventType: eventType,
		Metadata:  metadata,
	})
}

func (a *Impl) finishResult(result *multiagent.AgentResult) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.state == multiagent.AgentStateTerminated {
		// Agent already terminated (e.g. PermissionGate timeout). Ensure the
		// done channel is closed so Wait() does not block indefinitely.
		a.doneOnce.Do(func() { close(a.done) })
		return
	}
	a.result = result
	a.finishedAt = time.Now()
	a.state = multiagent.AgentStateTerminated
	a.doneOnce.Do(func() { close(a.done) })
	if a.creator != nil {
		a.creator.ReleaseSession(a.cfg.SessionID)
	}
}

// FinishedAt returns the wall-clock timestamp at which the agent entered
// the TERMINATED state. Zero before termination. Used by Join to sort
// child contributions by completion order (DM-20260611-005).
func (a *Impl) FinishedAt() time.Time {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.finishedAt
}

// AttachSessionView binds a COW view to this agent. Subsequent child
// forks will inherit an isolated child view of the bound view.
func (a *Impl) AttachSessionView(v *isolate.View) {
	a.mu.Lock()
	a.view = v
	a.mu.Unlock()
}

// SessionView returns the COW view bound to this agent. Nil only when
// the agent was created without going through a Fork flow.
func (a *Impl) SessionView() *isolate.View {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.view
}

func (a *Impl) activeChildCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.childAgents)
}

// startSpan creates a tracing child span from context if observability is wired.
func (a *Impl) startSpan(ctx context.Context, operation string, kind tracer.SpanKind, attrs ...tracer.Attribute) (context.Context, tracer.Span) {
	if a.deps.ObsBridge == nil || a.deps.ObsBridge.Tracer() == nil {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(kind),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(operation, attrs...)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return a.deps.ObsBridge.Tracer().Start(ctx, operation, opts...)
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

// SetEngine binds the per-agent context engine (with agent permission gate).
func (a *Impl) SetEngine(engine contracts.IEngine) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.engine = engine
	if a.deps.Engine == nil {
		a.deps.Engine = engine
	}
}

// SetAgentObserver adds an observer to the agent lifecycle observer chain.
func (a *Impl) SetAgentObserver(obs multiagent.AgentObserver) {
	if obs == nil {
		obs = kernel.NoOpAgentObserver{}
	}
	if chain, ok := a.deps.AgentObserver.(*multiagent.AgentObserverChain); ok {
		chain.Add(obs)
	} else {
		a.deps.AgentObserver = obs
	}
}

// SetEngineEventSink forwards engine events to gateway/adapters during Run.
func (a *Impl) SetEngineEventSink(sink func(*contracts.EngineEvent)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.engineEventSink = sink
}

func (a *Impl) processEngine() contracts.IEngine {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.engine != nil {
		return a.engine
	}
	return a.deps.Engine
}

// PermissionGate exposes the agent permission gate for turn-runtime tool execution.
func (a *Impl) PermissionGate() multiagent.PermissionGate {
	return a.permGate
}
