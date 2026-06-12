package wave

import (
	"testing"
	"time"
)

func TestArtifactStore_PutGet(t *testing.T) {
	s := NewArtifactStore()
	art := Artifact{
		TaskID:    "a",
		Summary:   "done",
		ExitCode:  0,
		StartedAt: time.Now(),
		EndedAt:   time.Now().Add(time.Second),
	}
	s.Put(art)
	got, ok := s.Get("a")
	if !ok {
		t.Fatal("expected to retrieve artifact")
	}
	if got.Summary != "done" {
		t.Fatalf("expected summary 'done', got %q", got.Summary)
	}
}

func TestArtifactStore_Unknown(t *testing.T) {
	s := NewArtifactStore()
	if _, ok := s.Get("missing"); ok {
		t.Fatal("expected missing artifact to be absent")
	}
}

func TestArtifactStore_SessionScoped(t *testing.T) {
	s := NewArtifactStore()
	s.PutForSession("sess-1", Artifact{TaskID: "a", Summary: "x"})
	if _, ok := s.Get("a"); !ok {
		t.Fatal("expected global lookup to find task 'a'")
	}
	art, ok := s.GetForSession("sess-1", "a")
	if !ok || art.Summary != "x" {
		t.Fatalf("expected session-scoped artifact, got ok=%v", ok)
	}
	if _, ok := s.GetForSession("sess-2", "a"); ok {
		t.Fatal("expected other session not to see artifact")
	}
}

func TestArtifactStore_List(t *testing.T) {
	s := NewArtifactStore()
	s.Put(Artifact{TaskID: "a"})
	s.Put(Artifact{TaskID: "b"})
	if got := s.List(); len(got) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(got))
	}
}
