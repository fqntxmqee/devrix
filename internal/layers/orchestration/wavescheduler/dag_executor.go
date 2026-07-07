// Package wavescheduler — DAGExecutor adapter (DM-20260707-001 PR-B).
//
// DAGExecutor is the thin adapter that converts a plan.PlanDAG (the
// per-segment execution graph emitted by the Plan node when a directive
// decomposes into ≥2 segments) into the WaveScheduler's TaskGraph and
// drives the existing 4-worker pool. It is NOT a rewrite of the
// scheduler — it is a coordinator that translates + observes.
//
// Boundaries:
//
//   - WaveScheduler is the runtime (Start / dispatchLoop / WaitForCompletion).
//     PR-B does NOT modify scheduler.go, pool.go, or conflict.go except for
//     two additive helpers (TaskGraph.CancelPending and
//     WaveScheduler.AbortSession) required by the strict abort path
//     (cursor Q4 HIGH risk).
//
//   - PlanDAG validation is owned by plan/dag_validator.go. The executor
//     trusts the input — re-validation is the caller's responsibility.
//
//   - Idempotency / dedup at the (sessionID, segmentID) level is owned by
//     PR-C's IM adapter. The executor emits one SegmentEmit per terminal
//     node; downstream consumers deduplicate.
//
//   - ItemPipelineRunner wiring (`Plan.DAG != nil` fork) is PR-D's job.
//     PR-B ships the runtime + isolated tests only.
//
// Contract surface (cursor Q5 ADOPT-WITH-CHANGE):
//
//   - The returned channel is CLOSED when RunPlanDAG returns. Callers MUST
//     start a consumer goroutine before calling RunPlanDAG, or the polling
//     goroutine will block on the unbuffered / full channel and the wave
//     will stall.
//   - "Last emit" carrying IsFinal=true is emitted exactly once, on the
//     chronologically-last SUCCESSFUL terminal (deterministic tie-break:
//     highest EndedAt, then lex SegmentID asc).
//   - Cancel (ctx, reentry, or child-error abort) closes the channel
//     WITHOUT emitting IsFinal=true. Callers treat "channel closed and
//     no IsFinal observed" as the abort signal.
package wavescheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
)

// SegmentEmit is the per-terminal-node streaming event produced by the
// DAGExecutor. One SegmentEmit per child completion; the wave's last emit
// carries IsFinal=true (deterministic tie-break below). The channel is
// closed when the wave reaches natural AllTerminal success (IsFinal=true
// was emitted) OR when an abort path cancels the wave (no IsFinal).
type SegmentEmit struct {
	// SessionID identifies the parent session; mirrors Plan.SessionID.
	SessionID string
	// PlanDAGID is the opaque id passed to RunPlanDAG (typically
	// plan.PlanDAG.ID); used as a dedup key by PR-C.
	PlanDAGID string
	// SegmentID is the corresponding IntentSegment.ID (== PlanNode.SegmentID
	// for v1, since PlanNode has 1:1 correspondence with IntentSegment).
	SegmentID string
	// WorkerType is the resolved WorkerType after the WorkerHint mapping.
	WorkerType WorkerType
	// IsFinal is true on the chronologically-last SUCCESSFUL terminal emit.
	// Absent on abort paths (ctx cancel, reentry cancel, child error).
	IsFinal bool
	// StartedAt / EndedAt are sourced from the worker's Artifact (codex
	// new risk #5) — not from the polling tick — so they reflect the
	// worker's actual run window, not the executor's observation latency.
	StartedAt time.Time
	EndedAt   time.Time
	// Summary is the worker's joined output (text + tool_use events from
	// WaveScheduler.completeTask).
	Summary string
	// ExitCode mirrors Artifact.ExitCode: 0=success, -1=failure, -2=cancel.
	ExitCode int
	// Error is empty on success; carries the worker error message on
	// failure / cancellation.
	Error string
	// WorkerHint echoes the original PlanNode.WorkerHint for downstream
	// observability (e.g. "workitem" hint lineage tracking).
	WorkerHint string
}

// DAGExecutor converts a plan.PlanDAG into a TaskGraph, drives the
// WaveScheduler, and exposes a streaming `<-chan SegmentEmit` API. One
// instance per session orchestrator (the adapter is stateless apart from
// the scheduler reference).
type DAGExecutor interface {
	// RunPlanDAG validates inputs, converts the DAG, starts the wave, and
	// returns a channel that emits one SegmentEmit per terminal node plus
	// a final IsFinal=true on natural completion. The channel is closed
	// when the wave reaches terminal state (success or abort).
	//
	// Errors returned synchronously are conversion-time failures only
	// (nil inputs, missing segments). Runtime failures (child errors,
	// ctx cancel) surface via the channel close without IsFinal AND via
	// the error returned by the synchronous return path (ErrDAGExecutionFailed
	// or ctx.Err()).
	RunPlanDAG(ctx context.Context, sessionID, planDAGID string,
		dag *plan.PlanDAG, segSet *interfaces.IntentSegmentSet) (
		<-chan SegmentEmit, error)
}

type dagExecutor struct {
	scheduler *WaveScheduler
	// pollInterval is the consumer-goroutine tick. 5ms is fast enough
	// for human-in-the-loop IM latency and cheap enough to run on every
	// wave. Tests can override via newTestDAGExecutor (white-box).
	pollInterval time.Duration
}

// NewDAGExecutor builds a DAGExecutor bound to the given scheduler. The
// executor reads the scheduler's pool / guard / resolver / artifacts /
// runners off `scheduler` directly (codex new risk #2: drop redundant
// SchedulerDeps arg) — no second deps bag is needed.
func NewDAGExecutor(scheduler *WaveScheduler) DAGExecutor {
	return &dagExecutor{
		scheduler:    scheduler,
		pollInterval: 5 * time.Millisecond,
	}
}

// =====================================================================
// Conversion: plan.PlanDAG → wavescheduler.TaskGraph
// =====================================================================

// convertDAG translates a plan.PlanDAG into a TaskGraph with the
// SortReadyNodes priority hook installed. Conversion is O(N+E) and
// returns a SentinelError (7210-7212) on invalid input.
func (d *dagExecutor) convertDAG(dag *plan.PlanDAG, segSet *interfaces.IntentSegmentSet, planDAGID string) (*TaskGraph, error) {
	if dag == nil {
		return nil, NewDAGExecutorNilDAGError()
	}
	if segSet == nil {
		return nil, NewDAGExecutorNilSegmentSetError()
	}

	// 1. Build segment-id index for cross-reference integrity.
	segIndex := make(map[string]string, len(segSet.Segments)) // segmentID -> Text
	for _, s := range segSet.Segments {
		segIndex[s.ID] = s.Text
	}

	// 2. Build TaskNodes.
	nodes := make([]TaskNode, 0, len(dag.Nodes))
	depIndex := make(map[string][]string, len(dag.Nodes))
	for _, e := range dag.Edges {
		depIndex[e.To] = append(depIndex[e.To], e.From)
	}
	for _, pn := range dag.Nodes {
		directive, ok := segIndex[pn.SegmentID]
		if !ok || directive == "" {
			return nil, NewDAGExecutorMissingSegmentError(pn.ID, pn.SegmentID)
		}
		wType, isWorkitem := convertWorkerHint(pn.WorkerHint)
		md := map[string]any{
			"priority":    priorityOrDefault(dag.Priorities, pn.ID),
			"plan_dag_id": planDAGID,
			"segment_id":  pn.SegmentID,
			"worker_hint": pn.WorkerHint,
		}
		if isWorkitem {
			// Cursor Q3 ADOPT-WITH-CHANGE: route workitem hint to subagent
			// (WorkerWorkItem has no slot) + stamp the lineage metadata so
			// downstream metrics/observability can still surface the hint.
			md["workitem_tag"] = true
		}
		nodes = append(nodes, TaskNode{
			ID:            pn.ID,
			Directive:     directive,
			WorkerType:    wType,
			DependsOn:     depIndex[pn.ID],
			ContextPolicy: ContextFresh,
			Metadata:      md,
		})
	}

	g := NewTaskGraph(nodes)
	// Q2 ADOPT-WITH-CHANGE: install the in-place sort.Slice hook. Runs
	// under g.mu.RLock(); MUST NOT call any TaskGraph write method.
	g.SortReadyNodes = func(ready []TaskNode) {
		sort.SliceStable(ready, func(i, j int) bool {
			pi := priorityOf(ready[i])
			pj := priorityOf(ready[j])
			if pi != pj {
				return pi > pj // higher priority first
			}
			return ready[i].ID < ready[j].ID // lex tie-break for determinism
		})
	}
	return g, nil
}

// convertWorkerHint maps a plan.WorkerHint string to a wavescheduler.WorkerType.
// Cursor Q3 ADOPT-WITH-CHANGE:
//
//	"" / "subagent"        → WorkerSubAgent
//	"cursor"               → WorkerCursor
//	"claude_code"          → WorkerClaudeCode
//	"workitem"             → WorkerSubAgent + isWorkitem=true (NO WorkerWorkItem — slot-less!)
//	<unknown>              → WorkerSubAgent + slog.Error
//
// The boolean return is true iff the original hint was "workitem", so the
// caller can stamp the lineage metadata.
func convertWorkerHint(hint string) (WorkerType, bool) {
	switch hint {
	case "", "subagent":
		return WorkerSubAgent, false
	case "cursor":
		return WorkerCursor, false
	case "claude_code":
		return WorkerClaudeCode, false
	case "workitem":
		// CRITICAL (cursor Q3 verification): WorkerWorkItem has no slot in
		// DefaultPoolCapacity. Routing a node to it would livelock the
		// dispatchLoop. Route to subagent and stamp metadata instead.
		return WorkerSubAgent, true
	default:
		// Codex Q3 ADOPT-WITH-CHANGE: bump unknown to slog.Error (an LLM
		// emitting a bogus hint is a contract violation, not a soft anomaly).
		slog.Error("dag_executor: unknown worker_hint — falling back to subagent",
			"hint", hint)
		return WorkerSubAgent, false
	}
}

// priorityOrDefault returns the priority for a node ID, falling back to
// 50 when absent (per design §2.3: 50 is the default for unprioritised
// nodes — matches IntentSegment.Priority default).
func priorityOrDefault(m map[string]int, id string) int {
	if v, ok := m[id]; ok {
		return v
	}
	return 50
}

func priorityOf(n TaskNode) int {
	if v, ok := n.Metadata["priority"].(int); ok {
		return v
	}
	return 50
}

// =====================================================================
// Runtime: RunPlanDAG
// =====================================================================

func (d *dagExecutor) RunPlanDAG(ctx context.Context, sessionID, planDAGID string,
	dag *plan.PlanDAG, segSet *interfaces.IntentSegmentSet) (
	<-chan SegmentEmit, error) {

	// Convert first so conversion-time errors return synchronously.
	graph, err := d.convertDAG(dag, segSet, planDAGID)
	if err != nil {
		return nil, err
	}

	// Buffer the channel to node count so the polling goroutine never
	// blocks on emit (cursor risk: unbuffered channel deadlock).
	out := make(chan SegmentEmit, len(dag.Nodes)+1)

	// Start the wave. Reentry is handled by WaveScheduler.Start itself —
	// prior wave is cancelled before this one begins (mirrors Start's
	// "session has no active wave" guard flipped into "cancel prior").
	if err := d.scheduler.Start(ctx, sessionID, graph); err != nil {
		return nil, fmt.Errorf("dag_executor: scheduler.Start: %w", err)
	}
	slog.Info("dag_executor: run_start",
		"session", sessionID, "plan_dag_id", planDAGID, "node_count", len(dag.Nodes))

	// Spawn the consumer goroutine that watches the ArtifactStore and
	// emits SegmentEmits. We capture `graph` (a stable *TaskGraph) at
	// start so we never re-read s.waves (codex new risk #3: polling
	// goroutine vs WaitForCompletion teardown race).
	d.spawnConsumer(ctx, sessionID, planDAGID, graph, out)

	return out, nil
}

// spawnConsumer launches the polling goroutine that bridges
// ArtifactStore → SegmentEmit channel. It returns immediately; the
// goroutine owns the channel-close lifecycle.
func (d *dagExecutor) spawnConsumer(ctx context.Context, sessionID, planDAGID string,
	graph *TaskGraph, out chan SegmentEmit) {

	go func() {
		// Capture graph at start — never re-read s.waves (codex #3).
		defer close(out)

		// emitted tracks which taskIDs have already been emitted, so the
		// polling loop doesn't double-emit (the polling tick can race
		// with itself if a node transitions across two ticks).
		emitted := make(map[string]bool, graph.NodeCount())
		// failedIDs is collected as nodes reach StateFailed; used for
		// the abort-path error message.
		var failedMu sync.Mutex
		failedIDs := make([]string, 0)

		ticker := time.NewTicker(d.pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				// Drain any final emissions first — workers that completed
				// between the last tick and the cancel signal would otherwise
				// be silently dropped (abortAndDrain skips StateCompleted /
				// StateFailed nodes that drainAnyNewTerminals hasn't yet
				// emitted).
				drainAnyNewTerminals(d.scheduler, sessionID, planDAGID, graph, emitted, &failedMu, &failedIDs, out)
				// Caller cancel / reentry cancel. Abort the wave
				// (running + pending → cancelled) and drain remaining
				// pending as StateCancelled emits.
				d.abortAndDrain(sessionID, planDAGID, graph, emitted, &failedMu, failedIDs, out, ctx.Err())
				return
			case <-ticker.C:
				// 1. Scan artifact store for new terminal nodes; emit SegmentEmit.
				// drainAnyNewTerminals drives emission from artifact presence,
				// which avoids the Put→SetState race in scheduler.completeTask
				// (artifact lands first, state transition lags).
				drainAnyNewTerminals(d.scheduler, sessionID, planDAGID, graph, emitted, &failedMu, &failedIDs, out)

				// 2. Detect failure: any failure seen this tick triggers
				//    abort. drainAnyNewTerminals appends to failedIDs in-line
				//    with the artifact emission, so a single tick can both
				//    emit a failure AND drive the abort path.
				failedMu.Lock()
				abortDue := len(failedIDs) > 0
				var abortErr error
				if abortDue {
					abortErr = NewDAGExecutionFailedError(append([]string(nil), failedIDs...))
				}
				failedMu.Unlock()
				if abortDue {
					d.abortAndDrain(sessionID, planDAGID, graph, emitted, &failedMu, failedIDs, out, abortErr)
					return
				}

				// 3. All terminal & no failures → drain again first, then emit
				//    IsFinal on the chronologically-last successful terminal.
				//
				// Why re-drain? completeTask writes Put then SetState, so a
				// tick can observe state-X-Completed while state-Y's Put is
				// still in flight (graph lock held). If we emit IsFinal on
				// the first tick where AllTerminal returns true without a
				// re-drain, the late node's regular SegmentEmit is dropped.
				// We loop until AllTerminal AND no new emissions in one drain
				// pass — bounded by graph.NodeCount() in the worst case but
				// normally converges in 1-2 passes.
				if graph.AllTerminal() {
					for {
						preLen := len(emitted)
						drainAnyNewTerminals(d.scheduler, sessionID, planDAGID, graph, emitted, &failedMu, &failedIDs, out)
						if len(emitted) == preLen {
							break
						}
					}
					emitFinalIfMissing(d.scheduler, sessionID, planDAGID, graph, emitted, out)
					return
				}
			}
		}
	}()
}

// drainAnyNewTerminals scans the artifact store for any task that has
// produced an Artifact but has not yet been emitted on the channel. We
// drive emission from the artifact store (NOT from graph.State) because
// WaveScheduler.completeTask writes the artifact BEFORE flipping the
// graph state to terminal — the state check is racy: a tick can observe
// state-RUNNING + artifact-present and skip, and then AllTerminal fires
// before the state catches up, losing the node's regular SegmentEmit.
func drainAnyNewTerminals(sched *WaveScheduler, sessionID, planDAGID string, graph *TaskGraph,
	emitted map[string]bool, failedMu *sync.Mutex, failedIDs *[]string, out chan<- SegmentEmit) {

	arts := sched.artifacts.ListForSession(sessionID)
	for _, art := range arts {
		if emitted[art.TaskID] {
			continue
		}
		emitted[art.TaskID] = true
		if isFailureArtifact(art) {
			failedMu.Lock()
			*failedIDs = append(*failedIDs, art.TaskID)
			failedMu.Unlock()
		}
		out <- artifactToEmit(art, sessionID, planDAGID, false)
	}
}

// isFailureArtifact reports whether an Artifact represents a failed task
// (vs. success or cancellation). ExitCode == -2 = cancellation,
// ExitCode == 0 = success; any other non-zero ExitCode is failure.
func isFailureArtifact(art Artifact) bool {
	return art.ExitCode != 0 && art.ExitCode != -2
}

// emitFinalIfMissing emits IsFinal=true on the chronologically-last
// successful terminal artifact. Deterministic tie-break per cursor Q5:
// highest EndedAt desc, then lex SegmentID asc.
func emitFinalIfMissing(sched *WaveScheduler, sessionID, planDAGID string, graph *TaskGraph,
	emitted map[string]bool, out chan<- SegmentEmit) {

	arts := sched.artifacts.ListForSession(sessionID)
	// Only consider successful (StateCompleted) terminals — failed and
	// cancelled nodes were already emitted without IsFinal.
	type cand struct {
		endedAt time.Time
		segID   string
		art     Artifact
	}
	cands := make([]cand, 0, len(arts))
	for _, a := range arts {
		st, ok := graph.State(a.TaskID)
		if !ok || st != StateCompleted {
			continue
		}
		cands = append(cands, cand{endedAt: a.EndedAt, segID: a.segmentIDOrTaskID(), art: a})
	}
	if len(cands) == 0 {
		return // All-terminal but every node failed / cancelled → abort path, not success.
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if !cands[i].endedAt.Equal(cands[j].endedAt) {
			return cands[i].endedAt.After(cands[j].endedAt)
		}
		return cands[i].segID < cands[j].segID
	})
	last := cands[0]
	emitted[last.art.TaskID] = true
	out <- artifactToEmit(last.art, sessionID, planDAGID, true)
}

// abortAndDrain is the unified abort path. It:
//
//  1. Calls WaveScheduler.AbortSession (running cancel + wave ctx cancel
//     + pending → cancelled).
//  2. Emits one SegmentEmit per un-emitted node that has not yet
//     transitioned to a final state. We walk graph.NodeIDs() rather
//     than graph.TerminalArtifacts() because the per-worker goroutines
//     transition to StateCancelled only AFTER Run() returns — between
//     the AbortSession call and the worker actually exiting, the node
//     is still StateRunning. Without the NodeIDs() walk, those nodes
//     would be missed and the channel would close with no cancel emit.
//  3. Returns; the deferred close(out) in the consumer goroutine then
//     closes the channel WITHOUT emitting IsFinal=true (per cursor Q5
//     cancel-without-IsFinal contract).
func (d *dagExecutor) abortAndDrain(sessionID, planDAGID string, graph *TaskGraph,
	emitted map[string]bool, failedMu *sync.Mutex, failedIDs []string,
	out chan<- SegmentEmit, abortErr error) {

	running, pending := d.scheduler.AbortSession(sessionID)
	slog.Info("dag_executor: run_abort",
		"session", sessionID, "plan_dag_id", planDAGID,
		"running_cancelled", running, "pending_cancelled", pending,
		"abort_err", abortErr)

	// Walk all known graph nodes; emit cancel for any not yet emitted
	// AND not already in StateCompleted / StateFailed (those are
	// already handled by drainAnyNewTerminals). We tolerate the race
	// where a worker lands in StateCancelled between the AbortSession
	// call and this walk — graph.State() reads under the same RLock
	// as the iteration so we won't miss it.
	for _, id := range graph.NodeIDs() {
		if emitted[id] {
			continue
		}
		st, _ := graph.State(id)
		if st == StateCompleted || st == StateFailed {
			// drainAnyNewTerminals already emitted these.
			continue
		}
		emitted[id] = true
		out <- SegmentEmit{
			SessionID:  sessionID,
			PlanDAGID:  planDAGID,
			SegmentID:  id,
			WorkerType: WorkerSubAgent,
			IsFinal:    false,
			ExitCode:   -2,
			Error:      fmt.Sprintf("cancelled by executor: %v", abortErr),
		}
	}
}

// artifactToEmit converts a wavescheduler.Artifact into a SegmentEmit.
// All timestamp data comes from the Artifact (codex new risk #5), not
// from the polling tick, so they reflect the worker's actual run window.
func artifactToEmit(art Artifact, sessionID, planDAGID string, isFinal bool) SegmentEmit {
	var segIDStr, hint string
	if art.Metadata != nil {
		if v, ok := art.Metadata["segment_id"].(string); ok {
			segIDStr = v
		}
		if v, ok := art.Metadata["worker_hint"].(string); ok {
			hint = v
		}
	}
	return SegmentEmit{
		SessionID:  sessionID,
		PlanDAGID:  planDAGID,
		SegmentID:  segIDOrFallback(segIDStr, art.TaskID),
		WorkerType: art.WorkerType,
		IsFinal:    isFinal,
		StartedAt:  art.StartedAt,
		EndedAt:    art.EndedAt,
		Summary:    art.Summary,
		ExitCode:   art.ExitCode,
		Error:      art.Error,
		WorkerHint: hint,
	}
}

func segIDOrFallback(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// segmentIDOrTaskID extracts the segment ID from the artifact metadata
// or returns the task ID as a fallback. Used by emitFinalIfMissing.
func (a Artifact) segmentIDOrTaskID() string {
	if a.Metadata != nil {
		if s, ok := a.Metadata["segment_id"].(string); ok && s != "" {
			return s
		}
	}
	return a.TaskID
}
