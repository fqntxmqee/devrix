package tracer

import (
	"context"
	"testing"
)

func TestBaggageManager_should_set_get_and_list(t *testing.T) {
	t.Parallel()

	m := NewBaggageManager(32)
	ctx := context.Background()
	ctx = m.Set(ctx, "session.id", "sess_1")
	ctx = m.Set(ctx, "user.id", "user_1")

	if val, ok := m.Get(ctx, "session.id"); !ok || val != "sess_1" {
		t.Fatalf("session.id: got %q ok=%v", val, ok)
	}

	items := m.List(ctx)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
}

func TestBaggageManager_should_enforce_max_items(t *testing.T) {
	t.Parallel()

	m := NewBaggageManager(2)
	ctx := context.Background()
	ctx = m.Set(ctx, "a", "1")
	ctx = m.Set(ctx, "b", "2")
	ctx = m.Set(ctx, "c", "3")

	items := m.List(ctx)
	if len(items) != 2 {
		t.Fatalf("expected max 2 items, got %d", len(items))
	}
}

func TestBaggageManager_should_roundtrip_header(t *testing.T) {
	t.Parallel()

	m := NewBaggageManager(32)
	ctx := m.Set(context.Background(), "session.id", "sess_hdr")
	ctx = m.Set(ctx, "user.id", "user_hdr")

	header := m.FormatHeader(ctx)
	if header == "" {
		t.Fatal("expected non-empty baggage header")
	}

	out := m.ApplyHeader(context.Background(), header)
	if val, ok := m.Get(out, "session.id"); !ok || val != "sess_hdr" {
		t.Fatalf("session.id after extract: %q ok=%v", val, ok)
	}
	if val, ok := m.Get(out, "user.id"); !ok || val != "user_hdr" {
		t.Fatalf("user.id after extract: %q ok=%v", val, ok)
	}
}

func TestBaggageManager_clear_should_remove_all(t *testing.T) {
	t.Parallel()

	m := NewBaggageManager(32)
	ctx := m.Set(context.Background(), "session.id", "sess_x")
	ctx = m.Clear(ctx)
	if len(m.List(ctx)) != 0 {
		t.Fatal("expected empty baggage after clear")
	}
}
