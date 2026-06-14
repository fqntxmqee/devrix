package signal

import (
	"sync"
	"time"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// TurnTracker holds per-session inbound turn state for objective anchors.
type TurnTracker struct {
	mu    sync.Mutex
	turns map[string]*TurnState
}

// TurnState tracks one user turn from persist through outbound signals.
type TurnState struct {
	InboundTurnID string
	StartedAt     time.Time
	Sequence      uint64
	SawThinking   bool
	SawTask       bool
	SawConclusion bool
	LastKind      contracts.SignalKind
}

// NewTurnTracker returns an empty turn tracker.
func NewTurnTracker() *TurnTracker {
	return &TurnTracker{turns: make(map[string]*TurnState)}
}

// BeginTurn starts or replaces turn state for a session after inbound persist.
func (t *TurnTracker) BeginTurn(sessionID, inboundTurnID string, at time.Time) {
	if sessionID == "" {
		return
	}
	if inboundTurnID == "" {
		inboundTurnID = sessionID + ":turn"
	}
	if at.IsZero() {
		at = time.Now()
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.turns[sessionID] = &TurnState{
		InboundTurnID: inboundTurnID,
		StartedAt:     at,
	}
}

// Next maps an engine event to a signal and advances turn sequence / chain flags.
func (t *TurnTracker) Next(sessionID string, ev *contracts.EngineEvent) (contracts.IMOutboundSignal, ChainReport, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	st, ok := t.turns[sessionID]
	if !ok || st == nil {
		st = &TurnState{InboundTurnID: sessionID + ":turn", StartedAt: time.Now()}
		t.turns[sessionID] = st
	}
	st.Sequence++
	sig, mapped := contracts.MapEngineEventToSignal(ev, st.Sequence, st.InboundTurnID, st.StartedAt)
	if !mapped {
		return contracts.IMOutboundSignal{}, ChainReport{}, false
	}
	report := st.record(sig.Kind)
	return sig, report, true
}

// EndTurn removes turn state after the inbound processing goroutine finishes.
func (t *TurnTracker) EndTurn(sessionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.turns, sessionID)
}

func (s *TurnState) record(kind contracts.SignalKind) ChainReport {
	report := ChainReport{Sequence: s.Sequence, Kind: kind, Intact: true}
	switch kind {
	case contracts.SignalThinking:
		s.SawThinking = true
	case contracts.SignalTask:
		s.SawTask = true
	case contracts.SignalConclusion:
		s.SawConclusion = true
	}
	if s.LastKind != "" && kindOrder(kind) < kindOrder(s.LastKind) {
		report.Intact = false
		report.BreakAt = kind
	}
	s.LastKind = kind
	report.SawThinking = s.SawThinking
	report.SawTask = s.SawTask
	report.SawConclusion = s.SawConclusion
	return report
}

// ChainReport summarizes signal chain integrity for a single emit.
type ChainReport struct {
	Sequence      uint64
	Kind          contracts.SignalKind
	Intact        bool
	BreakAt       contracts.SignalKind
	SawThinking   bool
	SawTask       bool
	SawConclusion bool
}

func kindOrder(k contracts.SignalKind) int {
	switch k {
	case contracts.SignalThinking:
		return 1
	case contracts.SignalTask:
		return 2
	case contracts.SignalConclusion:
		return 3
	default:
		return 0
	}
}
