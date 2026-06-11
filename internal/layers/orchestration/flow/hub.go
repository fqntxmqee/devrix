package flow

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/queue"
	"github.com/devrix/devrix/internal/layers/contextengine/tasks"
	"github.com/devrix/devrix/internal/layers/orchestration/workplan"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// IMSink delivers worker progress to the communication layer (Gateway / IM).
type IMSink interface {
	EmitWorkerProgress(ev contracts.FlowEvent)
}

// Hub implements contracts.ExecutionFlowHub with dual-channel dispatch.
type Hub struct {
	cfg      config.ExecutionFlowConfig
	q        *queue.SessionQueue
	workPlan *workplan.Service
	tasks    *tasks.TaskManager
	im       IMSink

	mu           sync.Mutex
	lastToolEmit map[string]time.Time
}

// HubDeps wires Hub dependencies.
type HubDeps struct {
	Config   config.ExecutionFlowConfig
	Queue    *queue.SessionQueue
	WorkPlan *workplan.Service
	Tasks    *tasks.TaskManager
	IM       IMSink
}

// NewHub creates an ExecutionFlowHub.
func NewHub(deps HubDeps) *Hub {
	cfg := config.NormalizeExecutionFlowConfig(deps.Config)
	wp := deps.WorkPlan
	if wp == nil {
		wp = workplan.NewService(cfg.EventBufferSize)
	}
	q := deps.Queue
	if q == nil {
		q = queue.GlobalSessionQueue
	}
	return &Hub{
		cfg:          cfg,
		q:            q,
		workPlan:     wp,
		tasks:        deps.Tasks,
		im:           deps.IM,
		lastToolEmit: make(map[string]time.Time),
	}
}

// GlobalHub is the process-wide execution flow hub (NoOp until wired).
var GlobalHub contracts.ExecutionFlowHub = contracts.NoOpExecutionFlowHub{}

// SetGlobalHub replaces GlobalHub (bootstrap).
func SetGlobalHub(h contracts.ExecutionFlowHub) {
	if h == nil {
		GlobalHub = contracts.NoOpExecutionFlowHub{}
		return
	}
	GlobalHub = h
}

// Publish records a flow event and fans out to Leader queue and IM.
func (h *Hub) Publish(ctx context.Context, ev contracts.FlowEvent) {
	if h == nil || !h.cfg.Enabled {
		return
	}
	if ev.At.IsZero() {
		ev.At = time.Now()
	}
	if ev.SessionID == "" {
		return
	}
	if ev.Kind == contracts.FlowToolCall && !h.allowToolEmit(ev) {
		return
	}

	h.workPlan.Apply(ev)
	h.linkTask(ev)

	body, _ := json.Marshal(ev)
	h.q.Enqueue(ev.SessionID, queue.QueuedCommand{
		Value: string(body),
		Mode:  queue.ModeDelegateProgress,
	})

	if h.cfg.IMProgress && h.im != nil && shouldEmitFlowEventToIM(ev) {
		h.im.EmitWorkerProgress(ev)
	}
	_ = ctx
}

func shouldEmitFlowEventToIM(ev contracts.FlowEvent) bool {
	switch ev.Kind {
	case contracts.FlowToolCall, contracts.FlowIterating:
		return false
	default:
		return true
	}
}

func (h *Hub) allowToolEmit(ev contracts.FlowEvent) bool {
	key := ev.SessionID + ":" + ev.WorkerID
	h.mu.Lock()
	defer h.mu.Unlock()
	last, ok := h.lastToolEmit[key]
	if ok && time.Since(last) < h.cfg.ToolSummaryThrottle() {
		return false
	}
	h.lastToolEmit[key] = time.Now()
	return true
}

func (h *Hub) linkTask(ev contracts.FlowEvent) {
	if !h.cfg.LinkTasks || h.tasks == nil || ev.TaskID == "" {
		return
	}
	sessionID := ev.SessionID
	switch ev.Kind {
	case contracts.FlowStarted:
		_ = h.tasks.SetOwner(sessionID, ev.TaskID, ev.WorkerID)
		_ = h.tasks.UpdateStatus(sessionID, ev.TaskID, tasks.TaskStatusInProgress)
	case contracts.FlowCompleted, contracts.FlowJoined:
		_ = h.tasks.UpdateStatus(sessionID, ev.TaskID, tasks.TaskStatusCompleted)
	case contracts.FlowFailed:
		_ = h.tasks.UpdateStatus(sessionID, ev.TaskID, tasks.TaskStatusFailed)
	}
}

// Snapshot returns the WorkPlan read model for a session.
func (h *Hub) Snapshot(sessionID string) contracts.WorkPlanSnapshot {
	if h == nil || h.workPlan == nil {
		return contracts.WorkPlanSnapshot{SessionID: sessionID}
	}
	snap := h.workPlan.Snapshot(sessionID)
	snap.Tasks = h.taskSnapshots(sessionID)
	return snap
}

func (h *Hub) taskSnapshots(sessionID string) []contracts.TaskSnapshot {
	if h == nil || h.tasks == nil || sessionID == "" {
		return nil
	}
	list := h.tasks.List(sessionID)
	if len(list) == 0 {
		return nil
	}
	out := make([]contracts.TaskSnapshot, 0, len(list))
	for _, t := range list {
		if t == nil {
			continue
		}
		out = append(out, contracts.TaskSnapshot{
			ID:      t.ID,
			Subject: t.Subject,
			Status:  string(t.Status),
			Owner:   t.Owner,
		})
	}
	return out
}