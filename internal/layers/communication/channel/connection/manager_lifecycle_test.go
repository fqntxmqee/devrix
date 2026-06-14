package connection

import (
	"testing"
	"time"

	"github.com/devrix/devrix/internal/shared/types"
)

func TestConnectionManager_should_register_and_count_connection(t *testing.T) {
	m := NewConnectionManager(30*time.Second, 5*time.Second)
	defer m.Stop()

	conn := &Connection{ID: "conn-1", AdapterID: "feishu", Type: "websocket"}
	m.Register(conn)

	if m.Count() != 1 {
		t.Fatalf("expected count 1, got %d", m.Count())
	}

	got, ok := m.Get("conn-1")
	if !ok {
		t.Fatal("expected connection to exist")
	}
	if got.Status != "connected" {
		t.Fatalf("expected status connected, got %s", got.Status)
	}

	list := m.List()
	if len(list) != 1 {
		t.Fatalf("expected list length 1, got %d", len(list))
	}
}

func TestConnectionManager_should_unregister_connection(t *testing.T) {
	m := NewConnectionManager(30*time.Second, 5*time.Second)
	defer m.Stop()

	conn := &Connection{ID: "conn-1", AdapterID: "feishu", Type: "websocket"}
	m.Register(conn)
	m.Unregister("conn-1")

	if m.Count() != 0 {
		t.Fatalf("expected count 0, got %d", m.Count())
	}
	if _, ok := m.Get("conn-1"); ok {
		t.Fatal("expected connection to be removed")
	}
}

func TestConnectionManager_should_reset_heartbeat_on_activity(t *testing.T) {
	m := NewConnectionManager(80*time.Millisecond, 10*time.Millisecond)
	defer m.Stop()

	lost := make(chan struct{}, 1)
	conn := &Connection{
		ID:        "conn-1",
		AdapterID: "feishu",
		Type:      "websocket",
		OnLost: func(*Connection) { lost <- struct{}{} },
	}
	m.Register(conn)

	time.Sleep(30 * time.Millisecond)
	m.Heartbeat("conn-1")
	time.Sleep(40 * time.Millisecond)

	select {
	case <-lost:
		t.Fatal("connection should not be lost after heartbeat")
	default:
	}
}

func TestConnectionManager_should_detect_connection_lost_on_timeout(t *testing.T) {
	m := NewConnectionManager(50*time.Millisecond, 10*time.Millisecond)
	defer m.Stop()

	lost := make(chan *Connection, 1)
	conn := &Connection{
		ID:        "conn-1",
		AdapterID: "feishu",
		Type:      "websocket",
		OnLost: func(c *Connection) { lost <- c },
	}
	m.Register(conn)

	select {
	case c := <-lost:
		if c.Status != "disconnected" {
			t.Fatalf("expected disconnected status, got %s", c.Status)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected connection lost callback")
	}
}

func TestConnectionManager_should_restore_connection(t *testing.T) {
	m := NewConnectionManager(30*time.Second, 5*time.Second)
	defer m.Stop()

	restored := make(chan *Connection, 1)
	conn := &Connection{
		ID:        "conn-1",
		AdapterID: "feishu",
		Type:      "websocket",
		OnRestored: func(c *Connection) { restored <- c },
	}
	m.Register(conn)
	m.handleConnectionRestored(conn)

	select {
	case c := <-restored:
		if c.Status != "connected" {
			t.Fatalf("expected connected status, got %s", c.Status)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected connection restored callback")
	}
}

func TestConnectionManager_should_stop_and_clear_connections(t *testing.T) {
	m := NewConnectionManager(30*time.Second, 5*time.Second)

	conn := &Connection{ID: "conn-1", AdapterID: "feishu", Type: "websocket"}
	m.Register(conn)
	m.Stop()

	if m.Count() != 0 {
		t.Fatalf("expected count 0 after stop, got %d", m.Count())
	}
}

func TestConnectionManager_heartbeat_should_ignore_unknown_connection(t *testing.T) {
	m := NewConnectionManager(30*time.Second, 5*time.Second)
	defer m.Stop()

	m.Heartbeat("missing")
}

func TestConnectionManager_unregister_should_ignore_unknown_connection(t *testing.T) {
	m := NewConnectionManager(30*time.Second, 5*time.Second)
	defer m.Stop()

	m.Unregister("missing")
}

func TestConnectionManager_emitEvent_should_handle_restored_data(t *testing.T) {
	m := NewConnectionManager(30*time.Second, 5*time.Second)
	event := types.NewDomainEvent(
		types.EventConnectionRestored,
		"sess-1",
		&types.EventConnectionRestoredData{ConnectionID: "conn-1", AdapterID: "feishu"},
	)
	m.emitEvent(event)
}
