package wavescheduler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/d7spans"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/google/uuid"
)

// SchedulerMetrics captures aggregate counters surfaced for testing / observability.
//
// DM-20260621-010 PR-B: 4 new counters added — WorkerPanics (recover hits),
// TaskCtxLeaked (cancel still non-nil after normal completion),
// WaveReentryCancelled (Start invoked while a wave is active),
// DispatchLoopWakeups (ticker + wakeupCh total wakeup events).
type SchedulerMetrics struct {
	Started         int
	Completed       int
	Failed          int
	Cancelled       int
	PeakRunning     int
	TotalDispatches int

	// New in PR-B.
	WorkerPanics         int
	TaskCtxLeaked        int
	WaveReentryCancelled int
	DispatchLoopWakeups  int
}

// ContextResolverIface is the minimal contract WaveScheduler needs from a
// context resolver. We define it as an interface so tests can wrap or
// substitute resolvers without re-implementing the full struct.
type ContextResolverIface interface {
	Resolve(n TaskNode) (ResolvedContext, error)
}

// WorkerEventHandler receives streaming worker events for IM / observability.
type WorkerEventHandler func(sessionID, taskID string, ev WorkerEvent)

// WaveScheduler is the DAG-driven, 5-slot worker pool. It does NOT make LLM
// scheduling decisions: it reads ready nodes from the in-memory TaskGraph,
// acquires slots from WorkerPool, checks ConflictGuard, and dispatches
// WorkerRunners. The dispatch loop is "continuous" (D2): any slot release
// triggers an immediate re-dispatch attempt.
//
// DSAFT: ORCH-S3-A01 (ScheduleWave)
type WaveScheduler struct {
	pool      *WorkerPool
	guard     *ConflictGuard
	resolver  ContextResolverIface
	artifacts *ArtifactStore
	runners   map[WorkerType]WorkerRunner
	obsBridge *observability.Bridge
	onWorkerEvent WorkerEventHandler

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
	scheduleSpan tracer.Span

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
	Pool          *WorkerPool
	Guard         *ConflictGuard
	Resolver      ContextResolverIface
	Artifacts     *ArtifactStore
	Runners       map[WorkerType]WorkerRunner
	Observability *observability.Bridge
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
		obsBridge: deps.Observability,
		waves:     make(map[string]*schedulerWaveState),
	}
}

// SetObsBridge wires the D5 observability bridge for wave spans.
func (s *WaveScheduler) SetObsBridge(bridge *observability.Bridge) {
	if s != nil {
		s.obsBridge = bridge
	}
}

// SetWorkerEventHandler forwards worker streaming events to OrchestratePath / IM.
func (s *WaveScheduler) SetWorkerEventHandler(h WorkerEventHandler) {
	if s != nil {
		s.onWorkerEvent = h
	}
}

// startOrchSpan creates a child span for orchestration operations.
func (s *WaveScheduler) startOrchSpan(ctx context.Context, operation string, kind tracer.SpanKind, attrs ...tracer.Attribute) (context.Context, tracer.Span) {
	if s.obsBridge == nil || !s.obsBridge.IsEnabled() {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(kind),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(operation, attrs...)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return s.obsBridge.Tracer().Start(ctx, operation, opts...)
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
	case "worker_panics":
		s.metrics.WorkerPanics++
	case "task_ctx_leaked":
		s.metrics.TaskCtxLeaked++
	case "wave_reentry_cancelled":
		s.metrics.WaveReentryCancelled++
	case "dispatch_loop_wakeups":
		s.metrics.DispatchLoopWakeups++
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
	s.mu.Unlock()

	waveCtx, scheduleSpan := s.startOrchSpan(ctx, telemetry.OpD7_S3_Orchestration_Wave_Schedule, tracer.SpanKindInternal,
		tracer.Attribute{Key: "wave.session_id", Value: sessionID},
		tracer.Attribute{Key: "wave.task_count", Value: fmt.Sprintf("%d", len(graph.nodes))},
		tracer.Attribute{Key: "wave.reentry", Value: boolStr(hasExisting)},
		tracer.Attribute{Key: "context.caller", Value: "d7"},
	)

	s.mu.Lock()
	state := &schedulerWaveState{
		sessionID:    sessionID,
		graph:        graph,
		handles:      make(map[string]*workerHandle),
		doneCh:       make(chan struct{}),
		wakeupCh:     make(chan struct{}, 64),
		scheduleSpan: scheduleSpan,
	}
	s.waves[sessionID] = state
	s.mu.Unlock()

	// Reentry: cancel prior wave first.
	if hasExisting {
		slog.Info("wave: reentry — cancelling prior wave", "session", sessionID)
		s.cancelWaveLocked(existing)
		s.incMetric("wave_reentry_cancelled")
	}

	// Register release hooks on the pool so we wake up the dispatch loop.
	if s.pool != nil {
		s.pool.OnRelease(func(_ SlotID) {
			// No-op signal — Start's main loop wakes via select on doneCh.
		})
	}

	// Spawn the dispatch loop. waveCtx carries Wave_Schedule until markWaveDone.
	go s.dispatchLoop(waveCtx, sessionID, state)
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
			// A4 (DM-20260622-001): Conflict check is atomic inside dispatchOne via
			// AllowAndRegister; the prior split Allow + Register left a TOCTOU
			// window. See design.md §2.4.
			slotID, ok := s.pool.Acquire(node.WorkerType, node.ID)
			if !ok {
				// No slot for this type — try another ready task.
				continue
			}
			if !s.dispatchOne(ctx, sessionID, state, node, slotID) {
				s.pool.Release(slotID)
				continue
			}
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
			s.incMetric("dispatch_loop_wakeups")
		case <-ticker.C:
			// Periodic check.
			s.incMetric("dispatch_loop_wakeups")
		}
	}
}

// dispatchOne atomically reserves the conflict slot and dispatches a worker
// for the given node. Returns true when the task was registered (whether it
// succeeds or fails); returns false when the conflict guard rejected the
// dispatch (the caller must release the slot).
//
// A4 (DM-20260622-001): Replaces the prior split Allow (dispatchLoop) +
// Register (dispatchOne) with a single atomic AllowAndRegister, eliminating
// the TOCTOU window. See design.md §2.4.
func (s *WaveScheduler) dispatchOne(parentCtx context.Context, sessionID string, state *schedulerWaveState, node TaskNode, slotID SlotID) bool {
	// v6.0.0 6 S 精简 S5-A34 P1: emit executor.select Span before the
	// runner lookup so Jaeger captures the candidate pool size, the
	// selected kind, and the selection score. Policy is "kind_match" —
	// the current selection logic is a direct map lookup by node.WorkerType.
	candidatesCount := len(s.runners)
	selectedKind := string(node.WorkerType)
	score := "0.000"
	if _, ok := s.runners[node.WorkerType]; ok {
		score = "1.000"
	} else {
		selectedKind = "none"
	}
	endExecSelect := d7spans.EmitExecutorSelect(parentCtx, sessionID, candidatesCount, selectedKind, score, "kind_match")

	runner, ok := s.runners[node.WorkerType]
	endExecSelect(nil)
	if !ok {
		// No runner — fail immediately, persist artifact, release slot.
		now := time.Now()
		s.completeTask(sessionID, state, node.ID, slotID, Artifact{
			TaskID:     node.ID,
			SessionID:  sessionID,
			WorkerType: node.WorkerType,
			Error:      fmt.Sprintf("no runner for kind %q", node.WorkerType),
			ExitCode:   -1,
			StartedAt:  now,
			EndedAt:    now,
		})
		return true
	}

	// Resolve context.
	resolved, err := s.resolver.Resolve(node)
	if err != nil {
		now := time.Now()
		s.completeTask(sessionID, state, node.ID, slotID, Artifact{
			TaskID:     node.ID,
			SessionID:  sessionID,
			WorkerType: node.WorkerType,
			Error:      err.Error(),
			ExitCode:   -1,
			StartedAt:  now,
			EndedAt:    now,
		})
		return true
	}

	// Atomic conflict check + register; eliminates the prior Allow/Register
	// TOCTOU window.
	if !s.guard.AllowAndRegister(node, slotID, s.guard.Running()) {
		return false
	}

	// Build a per-task context that the scheduler can cancel via CancelWorker.
	// Detach cancellation from parentCtx but preserve trace context so
	// Wave_Task_Execute stays under Wave_Schedule in Jaeger.
	taskCtx, cancel := context.WithCancel(tracer.Detach(parentCtx))
	state.graph.SetState(node.ID, StateRunning)
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

	var outputMu sync.Mutex
	outputParts := make([]string, 0, 4)
	spec := WorkerRunSpec{
		SessionID: sessionID,
		TaskID:    node.ID,
		WorkerID:  workerID,
		WorkDir:   node.WorkspaceDir(), // empty for now; tests pass workdir via Directive
		Directive: node.Directive,
		ModelTier: node.ModelTier,
		Context:   resolved,
		Emit: func(ev WorkerEvent) {
			if s.onWorkerEvent != nil {
				s.onWorkerEvent(sessionID, node.ID, ev)
			}
			slog.Debug("wave: worker event",
				"session", sessionID,
				"task", node.ID,
				"type", ev.Type,
				"content.len", len(ev.Content),
			)
			if ev.Content == "" {
				return
			}
			switch ev.Type {
			case "text", "complete", "tool_use":
				outputMu.Lock()
				outputParts = append(outputParts, ev.Content)
				outputMu.Unlock()
			}
		},
	}

	// Spawn worker goroutine.
	go func() {
		_, taskSpan := s.startOrchSpan(taskCtx, telemetry.OpD7_S3_Orchestration_Wave_Task_Execute, tracer.SpanKindInternal,
			tracer.Attribute{Key: "wave.session_id", Value: sessionID},
			tracer.Attribute{Key: "wave.task_id", Value: node.ID},
			tracer.Attribute{Key: "wave.worker_type", Value: string(node.WorkerType)},
			tracer.Attribute{Key: "wave.worker_id", Value: workerID},
			tracer.Attribute{Key: "wave.slot_id", Value: string(slotID)},
			tracer.Attribute{Key: "wave.model_tier", Value: string(node.ModelTier)},
			tracer.Attribute{Key: "wave.directive_len", Value: fmt.Sprintf("%d", len(node.Directive))},
			tracer.Attribute{Key: "context.caller", Value: "d7"},
		)
		defer func() {
			if r := recover(); r != nil {
				s.incMetric("worker_panics")
				slog.Error("wave: worker panic",
					"session", sessionID, "task", node.ID, "panic", r,
					"worker_id", workerID,
					"metric", "worker_panics")
				s.completeTask(sessionID, state, node.ID, slotID, Artifact{
					TaskID:    node.ID,
					SessionID: sessionID,
					Error:     fmt.Sprintf("worker panic: %v", r),
					ExitCode:  -1,
					StartedAt: handle.startedAt,
					EndedAt:   time.Now(),
				})
			}
			if taskSpan != nil {
				taskSpan.End()
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
		summary := spec.Directive
		outputMu.Lock()
		if len(outputParts) > 0 {
			summary = strings.Join(outputParts, "\n")
		}
		outputMu.Unlock()
		art := Artifact{
			TaskID:     node.ID,
			SessionID:  sessionID,
			WorkerType: node.WorkerType,
			Summary:    summary,
			ExitCode:   exitCode,
			Error:      errMsg,
			StartedAt:  handle.startedAt,
			EndedAt:    time.Now(),
		}
		s.completeTask(sessionID, state, node.ID, slotID, art)
	}()
	return true
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
	state.graph.SetState(taskID, terminal)

	// Update handle.
	state.mu.Lock()
	h, hOK := state.handles[taskID]
	if hOK {
		// taskCtx leak detection: if task reached normal completion (no error,
		// exit 0) but cancel is still non-nil, the caller didn't drive the
		// cancel lifecycle — flag it for observability.
		if h.cancel != nil && art.ExitCode == 0 && art.Error == "" {
			s.incMetric("task_ctx_leaked")
			slog.Warn("wave: taskCtx not cleaned up after normal completion",
				"session", sessionID, "task", taskID,
				"worker_id", h.taskID,
				"metric", "task_ctx_leaked")
		}
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

// finalizeTask (removed 2026-06-24): was a no-op extension point for
// IM / metrics side effects that was never implemented. The actual side
// effects (artifacts.Put, metric increments, guard.Unregister, pool.Release,
// graph.SetState) already happen at the call site. If/when a real
// extension point is needed, re-introduce it with concrete behaviour.

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

// markWaveDone transitions the wave to terminal state. It also clears the
// accumulated cancel-func slice and the per-task handles map so that
// long-lived sessions with repeated wave re-entries do not grow these
// collections unboundedly.
//
// A3 (DM-20260622-001): state.cancels and state.handles were previously
// appended on every dispatch but never released after CancelAll / wave
// completion. markWaveDone is the single wave-terminal point, making it
// the natural place to release those references. See design.md §2.3.
func (s *WaveScheduler) markWaveDone(state *schedulerWaveState) {
	state.mu.Lock()
	if state.done {
		state.mu.Unlock()
		return
	}
	state.done = true
	scheduleSpan := state.scheduleSpan
	state.scheduleSpan = nil
	close(state.doneCh)
	// Release per-wave cancel/handle bookkeeping. cancel funcs are already
	// invoked via cancelWaveLocked (or never needed for naturally completing
	// tasks), so dropping the references does not strand any context.
	state.cancels = nil
	state.handles = make(map[string]*workerHandle)
	state.mu.Unlock()
	if scheduleSpan != nil {
		scheduleSpan.End()
	}
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

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
