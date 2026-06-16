package wave

import (
	"testing"
)

func TestConflictGuard_ConflictGroup(t *testing.T) {
	g := NewConflictGuard()
	a := TaskNode{ID: "a", WorkerType: WorkerSubAgent, ConflictGroup: "db"}
	b := TaskNode{ID: "b", WorkerType: WorkerSubAgent, ConflictGroup: "db"}

	running := []RunningTask{{Node: a, SlotID: "s1"}}
	if g.Allow(b, running) {
		t.Fatalf("expected conflict (same group) to be denied")
	}
}

func TestConflictGuard_DifferentGroupAllowed(t *testing.T) {
	g := NewConflictGuard()
	a := TaskNode{ID: "a", WorkerType: WorkerSubAgent, ConflictGroup: "db"}
	b := TaskNode{ID: "b", WorkerType: WorkerSubAgent, ConflictGroup: "fs"}

	running := []RunningTask{{Node: a, SlotID: "s1"}}
	if !g.Allow(b, running) {
		t.Fatalf("expected different conflict_group to be allowed")
	}
}

func TestConflictGuard_FileScopeIntersect(t *testing.T) {
	g := NewConflictGuard()
	a := TaskNode{ID: "a", WorkerType: WorkerSubAgent, FileScope: []string{"src/api/**", "src/db/**"}}
	b := TaskNode{ID: "b", WorkerType: WorkerSubAgent, FileScope: []string{"src/db/migrations/**"}}

	running := []RunningTask{{Node: a, SlotID: "s1"}}
	if g.Allow(b, running) {
		t.Fatalf("expected file scope intersection to be denied")
	}
}

func TestConflictGuard_FileScopeDisjoint(t *testing.T) {
	g := NewConflictGuard()
	a := TaskNode{ID: "a", WorkerType: WorkerSubAgent, FileScope: []string{"src/api/**"}}
	b := TaskNode{ID: "b", WorkerType: WorkerSubAgent, FileScope: []string{"docs/**"}}

	running := []RunningTask{{Node: a, SlotID: "s1"}}
	if !g.Allow(b, running) {
		t.Fatalf("expected disjoint file scope to be allowed")
	}
}

func TestConflictGuard_ReadOnlyCanParallelWrite(t *testing.T) {
	g := NewConflictGuard()
	a := TaskNode{ID: "a", WorkerType: WorkerSubAgent, ReadOnly: true, FileScope: []string{"src/**"}}
	b := TaskNode{ID: "b", WorkerType: WorkerSubAgent, ReadOnly: false, FileScope: []string{"src/api/**"}}

	running := []RunningTask{{Node: a, SlotID: "s1"}}
	// a is read-only, b writes — both allowed to run.
	if !g.Allow(b, running) {
		t.Fatalf("read-only running task should not block writer")
	}
}

func TestConflictGuard_RegisterUnregister(t *testing.T) {
	g := NewConflictGuard()
	a := TaskNode{ID: "a", WorkerType: WorkerSubAgent, ConflictGroup: "x"}
	b := TaskNode{ID: "b", WorkerType: WorkerSubAgent, ConflictGroup: "x"}

	g.Register(RunningTask{Node: a, SlotID: "s1"})
	if g.Allow(b, nil) {
		t.Fatalf("expected deny after register of same group")
	}
	// Unregister — should allow next.
	g.Unregister("s1")
	if !g.Allow(b, nil) {
		t.Fatalf("expected allow after unregister")
	}
}

func TestConflictGuard_GlobMatching(t *testing.T) {
	if !globMatch("src/api/users.go", "src/api/**") {
		t.Fatal("expected glob match")
	}
	if globMatch("docs/readme.md", "src/api/**") {
		t.Fatal("expected no glob match across roots")
	}
	if !globMatch("src/api/users.go", "**/*.go") {
		t.Fatal("expected **/*.go to match")
	}
}

// T: D7-S3-A01-F03-T01 — AllowAndRegister succeeds and registers.
func TestConflictGuard_AllowAndRegister_NoConflict(t *testing.T) {
	g := NewConflictGuard()
	node := TaskNode{ID: "a", WorkerType: WorkerSubAgent, ConflictGroup: "db"}

	if !g.AllowAndRegister(node, "s1", nil) {
		t.Fatal("expected AllowAndRegister to succeed with empty guard")
	}
	if len(g.Running()) != 1 {
		t.Fatalf("expected 1 running task after registration, got %d", len(g.Running()))
	}
}

// T: D7-S3-A01-F03-T02 — AllowAndRegister blocks on conflict group.
func TestConflictGuard_AllowAndRegister_ConflictGroup(t *testing.T) {
	g := NewConflictGuard()
	a := TaskNode{ID: "a", WorkerType: WorkerSubAgent, ConflictGroup: "db"}
	b := TaskNode{ID: "b", WorkerType: WorkerSubAgent, ConflictGroup: "db"}

	g.Register(RunningTask{Node: a, SlotID: "s1"})
	if g.AllowAndRegister(b, "s2", nil) {
		t.Fatal("expected AllowAndRegister to block on same conflict group")
	}
	if len(g.Running()) != 1 {
		t.Fatalf("expected 1 running task (b not registered), got %d", len(g.Running()))
	}
}

// T: D7-S3-A01-F03-T03 — AllowAndRegister allows different groups.
func TestConflictGuard_AllowAndRegister_DifferentGroup(t *testing.T) {
	g := NewConflictGuard()
	a := TaskNode{ID: "a", WorkerType: WorkerSubAgent, ConflictGroup: "db"}
	b := TaskNode{ID: "b", WorkerType: WorkerSubAgent, ConflictGroup: "fs"}

	g.Register(RunningTask{Node: a, SlotID: "s1"})
	if !g.AllowAndRegister(b, "s2", nil) {
		t.Fatal("expected AllowAndRegister to allow different conflict group")
	}
	if len(g.Running()) != 2 {
		t.Fatalf("expected 2 running tasks, got %d", len(g.Running()))
	}
}

// T: D7-S3-A01-F03-T04 — AllowAndRegister blocks on file scope intersection.
func TestConflictGuard_AllowAndRegister_FileScope(t *testing.T) {
	g := NewConflictGuard()
	a := TaskNode{ID: "a", WorkerType: WorkerSubAgent, FileScope: []string{"src/auth/**"}}
	b := TaskNode{ID: "b", WorkerType: WorkerSubAgent, FileScope: []string{"src/auth/login.go"}}

	g.Register(RunningTask{Node: a, SlotID: "s1"})
	if g.AllowAndRegister(b, "s2", nil) {
		t.Fatal("expected AllowAndRegister to block on file scope intersection")
	}
}
