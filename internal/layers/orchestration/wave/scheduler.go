package wave

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SchedulerMetrics captures aggregate counters surfaced for testing / observability.
type SchedulerMetrics struct {
	Started         int
	Completed       int
	Failed          int
	Cancelled       int
	PeakRunning     int
	TotalDispatches int
}

// ContextResolverIface is the minimal contract WaveScheduler needs from a
// context resolver. We define it as an interface so tests can wrap or
// substitute resolvers without re-implementing the full struct.
type ContextResolverIface interface {
	Resolve(n TaskNode) (ResolvedContext, error)
}

// WaveScheduler is the DAG-driven, 5-slot worker pool. It does NOT make LLM
// scheduling decisions: it reads ready nodes from the in-memory TaskGraph,
// acquires slots from WorkerPool, checks ConflictGuard, and dispatches
// WorkerRunners. The dispatch loop is "continuous" (D2): any slot release
// triggers an immediate re-dispatch attempt.
type WaveScheduler struct {
	pool      *WorkerPool
	guard     *ConflictGuard
	resolver  ContextResolverIface
	artifacts *ArtifactStore
	runners   map[WorkerType]WorkerRunner

	// Per-wave runtime state. A wave is uniquely identified by sessionID —
	// v1.0 supports one active wave per session.
	mu    sync.Mutex
	waves map[string]*schedulerWaveState

	// metricsMu guards the metrics counters.
	metricsMu sync.Mutex
	metrics   SchedulerMetrics
}

type schedulerWaveState struct {
	sessionID string
	graph     *TaskGraph

	// Per-task runtime handles: cancel func + slotID + backgroundID.
	mu      sync.Mutex
	handles map[string]*workerHandle
	doneCh  chan struct{} // closed when all tasks reach terminal state
	done    bool
	cancels []context.CancelFunc // aggregated for CancelAll

	// wakeup is closed when the wave should be torn down (cancel / done).
	// The dispatch loop uses a separate trigger channel for slot-release
	// re-dispatch signals.
	wakeupCh chan struct{}
}

type workerHandle struct {
	taskID     string
	workerType WorkerType
	slotID     SlotID
	bgID       string
	cancel     context.CancelFunc
	startedAt  time.Time
}

// SchedulerDeps wires the scheduler.
type SchedulerDeps struct {
	Pool      *WorkerPool
	Guard     *ConflictGuard
	Resolver  ContextResolverIface
	Artifacts *ArtifactStore
	Runners   map[WorkerType]WorkerRunner
}

// NewWaveScheduler constructs a scheduler.
func NewWaveScheduler(deps SchedulerDeps) *WaveScheduler {
	runners := deps.Runners
	if runners == nil {
		runners = make(map[WorkerType]WorkerRunner)
	}
	return &WaveScheduler{
		pool:      deps.Pool,
		guard:     deps.Guard,
		resolver:  deps.Resolver,
		artifacts: deps.Artifacts,
		runners:   runners,
		waves:     make(map[string]*schedulerWaveState),
	}
}

// Metrics returns a snapshot of the scheduler's aggregate counters.
func (s *WaveScheduler) Metrics() SchedulerMetrics {
	if s == nil {
		return SchedulerMetrics{}
	}
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()
	return s.metrics
}

func (s *WaveScheduler) incMetric(field string) {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()
	switch field {
	case "started":
		s.metrics.Started++
	case "completed":
		s.metrics.Completed++
	case "failed":
		s.metrics.Failed++
	case "cancelled":
		s.metrics.Cancelled++
	case "dispatch":
		s.metrics.TotalDispatches++
	}
}

// Start registers a new wave for the session. If a wave already exists, it is
// cancelled first (per design §6.3, Plan reentry semantics).
func (s *WaveScheduler) Start(ctx context.Context, sessionID string, graph *TaskGraph) error {
	if s == nil {
		return errWave("nil scheduler")
	}
	if graph == nil {
		return errWave("nil graph")
	}
	if sessionID == "" {
		return errWave("sessionID is required")
	}
	for _, n := range graph.nodes {
		if err := n.Validate(); err != nil {
			return err
		}
	}

	s.mu.Lock()
	existing, hasExisting := s.waves[sessionID]
	state := &schedulerWaveState{
		sessionID: sessionID,
		graph:     graph,
		handles:   make(map[string]*workerHandle),
		doneCh:    make(chan struct{}),
		wakeupCh:  make(chan struct{}, 64),
	}
	s.waves[sessionID] = state
	s.mu.Unlock()

	// Reentry: cancel prior wave first.
	if hasExisting {
		slog.Info("wave: reentry — cancelling prior wave", "session", sessionID)
		s.cancelWaveLocked(existing)
	}

	// Register release hooks on the pool so we wake up the dispatch loop.
	if s.pool != nil {
		s.pool.OnRelease(func(_ SlotID) {
			// No-op signal — Start's main loop wakes via select on doneCh.
		})
	}

	// Spawn the dispatch loop.
	go s.dispatchLoop(ctx, sessionID, state)
	return nil
}

func (s *WaveScheduler) dispatchLoop(ctx context.Context, sessionID string, state *schedulerWaveState) {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	// Hook slot-release notifications into the wakeup channel.
	if s.pool != nil {
		s.pool.OnRelease(func(_ SlotID) {
			select {
			case state.wakeupCh <- struct{}{}:
			default:
			}
		})
	}

	for {
		// Check global done.
		if state.graph.AllTerminal() {
			s.markWaveDone(state)
			return
		}
		// Check ctx.
		if ctx.Err() != nil {
			s.cancelWaveLocked(state)
			s.markWaveDone(state)
			return
		}

		// Try to dispatch ready tasks.
		ready := state.graph.ReadyNodes()
		dispatched := 0
		for _, node := range ready {
			if !s.guard.Allow(node, s.guard.Running()) {
				continue
			}
			slotID, ok := s.pool.Acquire(node.WorkerType, node.ID)
			if !ok {
				// No slot for this type — try another ready task.
				continue
			}
			s.dispatchOne(ctx, sessionID, state, node, slotID)
			dispatched++
		}

		// Update peak metric.
		s.recordPeakRunning(state)

		// Wait for next tick OR wakeup OR done OR ctx cancel.
		select {
		case <-ctx.Done():
			s.cancelWaveLocked(state)
			s.markWaveDone(state)
			return
		case <-state.wakeupCh:
			// A slot was released OR a task completed — re-check ready tasks.
		case <-ticker.C:
			// Periodic check.
		}
	}
}

func (s *WaveScheduler) dispatchOne(parentCtx context.Context, sessionID string, state *schedulerWaveState, node TaskNode, slotID SlotID) {
	runner, ok := s.runners[node.WorkerType]
	if !ok {
		// No runner — fail immediately and release slot.
		s.pool.Release(slotID)
		state.graph.SetState(node.ID, StateFailed)
		s.incMetric("failed")
		s.finalizeTask(sessionID, state, node.ID, Artifact{
			TaskID:    node.ID,
			SessionID: sessionID,
			Error:     fmt.Sprintf("no runner for kind %q", node.WorkerType),
			ExitCode:  -1,
			StartedAt: time.Now(),
			EndedAt:   time.Now(),
		})
		return
	}

	// Resolve context.
	resolved, err := s.resolver.Resolve(node)
	if err != nil {
		s.pool.Release(slotID)
		state.graph.SetState(node.ID, StateFailed)
		s.incMetric("failed")
		s.finalizeTask(sessionID, state, node.ID, Artifact{
			TaskID:    node.ID,
			SessionID: sessionID,
			Error:     err.Error(),
			ExitCode:  -1,
			StartedAt: time.Now(),
			EndedAt:   time.Now(),
		})
		return
	}

	// Build a per-task context that the scheduler can cancel via CancelWorker.
	// Detach from parentCtx: cancelling the parent should NOT kill in-flight
	// workers (Plan Engine expects the wave to keep going on leader-ctx cancel).
	taskCtx, cancel := context.WithCancel(context.Background())
	state.graph.SetState(node.ID, StateRunning)
	s.guard.Register(RunningTask{Node: node, SlotID: slotID})
	s.incMetric("dispatch")

	workerID := "wkr-" + uuid.New().String()[:8]
	handle := &workerHandle{
		taskID:     node.ID,
		workerType: node.WorkerType,
		slotID:     slotID,
		cancel:     cancel,
		startedAt:  time.Now(),
	}
	state.mu.Lock()
	state.handles[node.ID] = handle
	state.cancels = append(state.cancels, cancel)
	state.mu.Unlock()

	spec := WorkerRunSpec{
		SessionID: sessionID,
		TaskID:    node.ID,
		WorkerID:  workerID,
		WorkDir:   node.WorkspaceDir(), // empty for now; tests pass workdir via Directive
		Directive: node.Directive,
		Context:   resolved,
		Emit: func(ev WorkerEvent) {
			// Hook for IM card renderer (L5-ORCH-14).
			slog.Debug("wave: worker event",
				"session", sessionID,
				"task", node.ID,
				"type", ev.Type,
				"content.len", len(ev.Content),
			)
		},
	}

	// Spawn worker goroutine.
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("wave: worker panic",
					"session", sessionID, "task", node.ID, "panic", r)
				s.completeTask(sessionID, state, node.ID, slotID, Artifact{
					TaskID:    node.ID,
					SessionID: sessionID,
					Error:     fmt.Sprintf("worker panic: %v", r),
					ExitCode:  -1,
					StartedAt: handle.startedAt,
					EndedAt:   time.Now(),
				})
			}
		}()
		runnerErr := runner.Run(taskCtx, spec)
		exitCode := 0
		errMsg := ""
		if runnerErr != nil {
			errMsg = runnerErr.Error()
			if runnerErr == context.Canceled {
				exitCode = -2
			} else {
				exitCode = -1
			}
		}
		// Build a per-task background id note (for SubAgent upstream cancel).
		bgID := ""
		if handle != nil {
			bgID = handle.bgID
		}
		_ = bgID
		// Determine artifact summary from spec — in production, runner emits
		// a "complete" event with content that we capture. v1.0 keeps it simple:
		// we record the worker's directive as the summary placeholder.
		art := Artifact{
			TaskID:     node.ID,
			SessionID:  sessionID,
			WorkerType: node.WorkerType,
			Summary:    spec.Directive,
			ExitCode:   exitCode,
			Error:      errMsg,
			StartedAt:  handle.startedAt,
			EndedAt:    time.Now(),
		}
		s.completeTask(sessionID, state, node.ID, slotID, art)
	}()
}

func (s *WaveScheduler) completeTask(sessionID string, state *schedulerWaveState, taskID string, slotID SlotID, art Artifact) {
	// Determine terminal state.
	if state.graph == nil {
		return
	}
	current, _ := state.graph.State(taskID)
	terminal := StateCompleted
	if art.Error != "" {
		if art.ExitCode == -2 || current == StateCancelled {
			terminal = StateCancelled
		} else {
			terminal = StateFailed
		}
	}
	// Side-effects MUST land before SetState flips the node terminal — once a
	// task is observable as terminal, dispatchLoop can call AllTerminal() →
	// markWaveDone() → close(doneCh) and WaitForCompletion returns. Anything
	// written after SetState risks being missed by the awaiter.
	s.artifacts.Put(art)
	switch terminal {
	case StateCompleted:
		s.incMetric("completed")
	case StateFailed:
		s.incMetric("failed")
	case StateCancelled:
		s.incMetric("cancelled")
	}
	s.guard.Unregister(slotID)
	s.pool.Release(slotID)
	s.finalizeTask(sessionID, state, taskID, art)
	state.graph.SetState(taskID, terminal)

	// Update handle.
	state.mu.Lock()
	if h, ok := state.handles[taskID]; ok {
		h.cancel = nil
		delete(state.handles, taskID)
	}
	state.mu.Unlock()

	// Wake the dispatch loop.
	select {
	case state.wakeupCh <- struct{}{}:
	default:
	}
}

// finalizeTask is a no-op extension point for IM / metrics side effects.
func (s *WaveScheduler) finalizeTask(sessionID string, state *schedulerWaveState, taskID string, art Artifact) {
	_ = sessionID
	_ = state
	_ = taskID
	_ = art
}

func (s *WaveScheduler) recordPeakRunning(state *schedulerWaveState) {
	if state == nil {
		return
	}
	running := len(state.graph.RunningNodes())
	s.metricsMu.Lock()
	if running > s.metrics.PeakRunning {
		s.metrics.PeakRunning = running
	}
	s.metricsMu.Unlock()
}

func (s *WaveScheduler) markWaveDone(state *schedulerWaveState) {
	state.mu.Lock()
	if state.done {
		state.mu.Unlock()
		return
	}
	state.done = true
	close(state.doneCh)
	state.mu.Unlock()
}

// CancelWorker cancels a single worker. Returns nil if the task was already
// terminal or unknown (idempotent).
func (s *WaveScheduler) CancelWorker(sessionID, taskID string) error {
	if s == nil {
		return errWave("nil scheduler")
	}
	s.mu.Lock()
	state, ok := s.waves[sessionID]
	s.mu.Unlock()
	if !ok {
		return errWave("session %q has no active wave", sessionID)
	}
	state.mu.Lock()
	h, exists := state.handles[taskID]
	state.mu.Unlock()
	if !exists {
		// Possibly already finished; treat as success.
		return nil
	}
	// Mark cancelled first so the worker's terminal transition lands in
	// the cancelled bucket even if Run returns normally.
	state.graph.SetState(taskID, StateCancelled)
	if h.cancel != nil {
		h.cancel()
	}
	return nil
}

// CancelAll cancels every running task for the session. Returns the count of
// tasks that were in-flight.
func (s *WaveScheduler) CancelAll(sessionID string) int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	state, ok := s.waves[sessionID]
	s.mu.Unlock()
	if !ok {
		return 0
	}
	return s.cancelWaveLocked(state)
}

func (s *WaveScheduler) cancelWaveLocked(state *schedulerWaveState) int {
	state.mu.Lock()
	defer state.mu.Unlock()
	count := 0
	for _, h := range state.handles {
		if h.cancel != nil {
			h.cancel()
			count++
		}
	}
	return count
}

// WaitForCompletion blocks until the wave reaches AllTerminal or ctx is
// cancelled. Returns the artifacts collected.
func (s *WaveScheduler) WaitForCompletion(ctx context.Context, sessionID string) ([]Artifact, error) {
	if s == nil {
		return nil, errWave("nil scheduler")
	}
	s.mu.Lock()
	state, ok := s.waves[sessionID]
	s.mu.Unlock()
	if !ok {
		return nil, errWave("session %q has no active wave", sessionID)
	}
	select {
	case <-state.doneCh:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return s.artifacts.List(), nil
}

// PeakRunning exposes the high-water mark of concurrent workers seen since
// boot. Test helper.
func (s *WaveScheduler) PeakRunning() int {
	if s == nil {
		return 0
	}
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()
	return s.metrics.PeakRunning
}
