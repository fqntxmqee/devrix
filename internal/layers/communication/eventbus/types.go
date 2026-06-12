// Package eventbus implements the BackpressureEventBus with a
// Drain→Compact→Reconnect state machine.
//
// DM-20260611-003 (devrix-event-channel).
package eventbus

import (
	"time"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// Priority classifies events for backpressure handling. Lower numeric value
// means higher importance. Critical events are never dropped by Drain or
// Compact; this is a P0 invariant.
type Priority int

const (
	// PriorityCritical is reserved for terminator events (complete / error).
	// Critical events traverse an independent unbuffered channel and are
	// guaranteed to reach every subscriber regardless of bus state.
	PriorityCritical Priority = 0
	// PriorityNormal is the default for user-visible events
	// (text / tool_call / tool_result / thinking).
	PriorityNormal Priority = 1
	// PriorityLow is for progress events (milestone_progress / worker_progress)
	// that may be safely merged or dropped under backpressure.
	PriorityLow Priority = 2
)

// Event wraps a contracts.EngineEvent with backpressure metadata.
//
// Event is treated as an immutable value: the underlying *contracts.EngineEvent
// is shared, but Priority / Sequence / PublishedAt are value fields and must
// not be mutated. Use WithPriority / WithSequence to derive new copies.
type Event struct {
	*contracts.EngineEvent
	Priority    Priority
	Sequence    uint64
	PublishedAt time.Time
}

// WithPriority returns a copy of the event with the given priority.
// The underlying *contracts.EngineEvent pointer is shared (immutable contract).
func (e Event) WithPriority(p Priority) Event {
	e.Priority = p
	return e
}

// WithSequence returns a copy of the event with the given sequence number.
func (e Event) WithSequence(seq uint64) Event {
	e.Sequence = seq
	return e
}

// WithPublishedAt returns a copy of the event with the given timestamp.
func (e Event) WithPublishedAt(t time.Time) Event {
	e.PublishedAt = t
	return e
}

// IsTerminator reports whether the wrapped event is a complete/error event.
// Terminating events are force-promoted to PriorityCritical by the bus.
func (e Event) IsTerminator() bool {
	if e.EngineEvent == nil {
		return false
	}
	switch e.EngineEvent.Type {
	case "complete", "error":
		return true
	default:
		return false
	}
}

// State is the bus lifecycle state.
type State int

const (
	// StateRunning is the normal operating state.
	StateRunning State = iota
	// StateDraining means backlog exceeded high watermark; bus is shedding
	// Low/Normal events and waiting for backlog to drop below low watermark.
	StateDraining
	// StateCompacting means backlog is below low watermark; bus is merging
	// adjacent same-type Low/Normal events.
	StateCompacting
	// StateReconnecting means the bus is rebuilding its internal channel
	// pipeline. Critical events still flow through; Normal/Low events are
	// held in a pending buffer until the new pipeline is ready.
	StateReconnecting
	// StateClosed means Close() was called; no further events accepted.
	StateClosed
)

// String returns a human-readable state name (for logging / metrics).
func (s State) String() string {
	switch s {
	case StateRunning:
		return "running"
	case StateDraining:
		return "draining"
	case StateCompacting:
		return "compacting"
	case StateReconnecting:
		return "reconnecting"
	case StateClosed:
		return "closed"
	default:
		return "unknown"
	}
}
