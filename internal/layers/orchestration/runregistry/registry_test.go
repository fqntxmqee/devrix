package runregistry

import (
	"testing"
)

func TestRegistry_RegisterAndTerminal(t *testing.T) {
	r := NewRegistry(t.TempDir())
	runID, _ := r.Register("sess", "wi_abc", "implement")
	if runID == "" {
		t.Fatal("expected run id")
	}
	r.SetTerminal(runID, StatusCompleted, "done", "")
	e, ok := r.Get(runID)
	if !ok || e.Status != StatusCompleted {
		t.Fatalf("unexpected entry: %+v ok=%v", e, ok)
	}
}

func TestRegistry_OutputDelta(t *testing.T) {
	r := NewRegistry(t.TempDir())
	runID, _ := r.Register("sess", "wi_1", "agent")
	_ = r.AppendOutput(runID, []byte("hello "))
	_ = r.AppendOutput(runID, []byte("world"))
	delta, off, _, err := r.GetOutputDelta(runID, 0)
	if err != nil || delta != "hello world" || off != 11 {
		t.Fatalf("delta=%q off=%d err=%v", delta, off, err)
	}
}

func TestRegistry_NotifiedOnce(t *testing.T) {
	r := NewRegistry("")
	runID, _ := r.Register("s", "w", "k")
	called := 0
	r.OnTerminal(runID, func(e Entry) { called++ })
	r.SetTerminal(runID, StatusCompleted, "", "")
	r.SetTerminal(runID, StatusCompleted, "", "")
	if called != 1 {
		t.Fatalf("expected 1 callback, got %d", called)
	}
}
