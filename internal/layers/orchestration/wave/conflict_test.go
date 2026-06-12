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
