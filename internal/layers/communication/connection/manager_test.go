package connection

import (
	"testing"
	"time"

	"github.com/devrix/devrix/internal/shared/types"
)

// Covers: L5-0-1-03
func TestConnectionManager_emitEvent_connectionLost(t *testing.T) {
	m := NewConnectionManager(30*time.Second, 5*time.Second)
	event := types.NewDomainEvent(
		types.EventConnectionLost,
		"sess-1",
		&types.EventConnectionLostData{ConnectionID: "conn-1", AdapterID: "feishu"},
	)
	m.emitEvent(event)
}

// Covers: L5-0-1-04
func TestConnectionManager_emitEvent_connectionRestored(t *testing.T) {
	m := NewConnectionManager(30*time.Second, 5*time.Second)
	event := types.NewDomainEvent(
		types.EventConnectionRestored,
		"sess-1",
		&types.EventConnectionRestoredData{ConnectionID: "conn-1", AdapterID: "feishu"},
	)
	m.emitEvent(event)
}

// Covers: L5-0-1-05
func TestConnectionManager_emitEvent_unknownType(t *testing.T) {
	m := NewConnectionManager(30*time.Second, 5*time.Second)
	event := types.NewDomainEvent(types.EventConnectionLost, "sess-1", struct{ ID string }{ID: "x"})
	m.emitEvent(event)
}
