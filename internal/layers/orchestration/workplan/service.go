package workplan

import (
	"sync"
	"time"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// Service maintains the WorkPlan read model (session-scoped execution flows).
type Service struct {
	mu     sync.RWMutex
	buf    int
	flows  map[string]map[string]*flowState // sessionID -> flowID -> state
}

type flowState struct {
	snap contracts.ExecutionFlowSnapshot
}

// NewService creates a WorkPlan service.
func NewService(eventBufferSize int) *Service {
	if eventBufferSize <= 0 {
		eventBufferSize = 32
	}
	return &Service{
		buf:   eventBufferSize,
		flows: make(map[string]map[string]*flowState),
	}
}

// Apply merges a flow event into the read model.
func (s *Service) Apply(ev contracts.FlowEvent) {
	if s == nil || ev.SessionID == "" {
		return
	}
	flowID := ev.FlowID
	if flowID == "" {
		flowID = ev.WorkerID
	}
	if flowID == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	bySession := s.flows[ev.SessionID]
	if bySession == nil {
		bySession = make(map[string]*flowState)
		s.flows[ev.SessionID] = bySession
	}
	st, ok := bySession[flowID]
	if !ok {
		st = &flowState{
			snap: contracts.ExecutionFlowSnapshot{
				FlowID:   flowID,
				WorkerID: ev.WorkerID,
				TaskID:   ev.TaskID,
				Source:   ev.Source,
				Role:     ev.Role,
				Status:   "pending",
			},
		}
		bySession[flowID] = st
	}
	st.snap.WorkerID = ev.WorkerID
	st.snap.TaskID = ev.TaskID
	st.snap.Source = ev.Source
	st.snap.Role = ev.Role
	st.snap.LastEvent = ev
	st.snap.Status = statusForKind(ev.Kind, st.snap.Status)
	st.snap.RecentEvents = append(st.snap.RecentEvents, ev)
	if len(st.snap.RecentEvents) > s.buf {
		st.snap.RecentEvents = st.snap.RecentEvents[len(st.snap.RecentEvents)-s.buf:]
	}
}

func statusForKind(kind contracts.FlowEventKind, prev string) string {
	switch kind {
	case contracts.FlowStarted, contracts.FlowIterating, contracts.FlowToolCall, contracts.FlowProgress:
		return "running"
	case contracts.FlowWaitingPermission:
		return "waiting_permission"
	case contracts.FlowCompleted, contracts.FlowJoined:
		return "completed"
	case contracts.FlowFailed:
		return "failed"
	default:
		if prev != "" {
			return prev
		}
		return "running"
	}
}

// Snapshot returns the current WorkPlan for a session.
func (s *Service) Snapshot(sessionID string) contracts.WorkPlanSnapshot {
	out := contracts.WorkPlanSnapshot{
		SessionID: sessionID,
		UpdatedAt: time.Now(),
	}
	if s == nil {
		return out
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	bySession := s.flows[sessionID]
	if len(bySession) == 0 {
		return out
	}
	out.ExecutionFlows = make([]contracts.ExecutionFlowSnapshot, 0, len(bySession))
	for _, st := range bySession {
		out.ExecutionFlows = append(out.ExecutionFlows, st.snap)
	}
	return out
}
