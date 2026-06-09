package bootstrap

import "testing"

func TestResolveAgentWorkDir_should_fallback_when_requested_missing(t *testing.T) {
	session := t.TempDir()
	got := resolveAgentWorkDir("/home/user/nonexistent", session)
	if got != session {
		t.Fatalf("got %q, want session dir %q", got, session)
	}
}

func TestResolveAgentWorkDir_should_use_requested_when_valid(t *testing.T) {
	dir := t.TempDir()
	got := resolveAgentWorkDir(dir, t.TempDir())
	if got != dir {
		t.Fatalf("got %q, want %q", got, dir)
	}
}
