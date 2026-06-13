package prompt

import "testing"

func TestResolveCachedSection_should_cache_per_session(t *testing.T) {
	ClearAllDynamicSectionCache()
	calls := 0
	compute := func() string {
		calls++
		return "value"
	}

	v1 := resolveCachedSection("sess_a", "session_context", false, compute)
	v2 := resolveCachedSection("sess_a", "session_context", false, compute)
	if v1 != "value" || v2 != "value" {
		t.Fatalf("unexpected values: %q %q", v1, v2)
	}
	if calls != 1 {
		t.Fatalf("expected 1 compute, got %d", calls)
	}

	v3 := resolveCachedSection("sess_b", "session_context", false, compute)
	if v3 != "value" {
		t.Fatalf("expected value for other session, got %q", v3)
	}
	if calls != 2 {
		t.Fatalf("expected 2 computes for different sessions, got %d", calls)
	}
}

func TestClearDynamicSectionCache_should_drop_session_entries(t *testing.T) {
	ClearAllDynamicSectionCache()
	resolveCachedSection("sess_clear", "git_status", false, func() string { return "git" })
	ClearDynamicSectionCache("sess_clear")
	calls := 0
	v := resolveCachedSection("sess_clear", "git_status", false, func() string {
		calls++
		return "git2"
	})
	if v != "git2" || calls != 1 {
		t.Fatalf("expected recomputation after clear, got %q calls=%d", v, calls)
	}
}
