package wave

import (
	"sort"
	"testing"
)

func newNode(id string, deps ...string) TaskNode {
	return TaskNode{
		ID:            id,
		WorkerType:    WorkerSubAgent,
		ContextPolicy: ContextFresh,
		Directive:     "do " + id,
		DependsOn:     deps,
	}
}

func TestTaskGraph_ReadyNodes(t *testing.T) {
	// L5-ORCH-17: only ready (deps all completed) nodes are dispatchable.
	g := NewTaskGraph([]TaskNode{
		newNode("a"),
		newNode("b", "a"),
		newNode("c", "a"),
		newNode("d", "b", "c"),
	})
	// Initially: only 'a' has no deps.
	ready := g.ReadyNodes()
	if len(ready) != 1 || ready[0].ID != "a" {
		t.Fatalf("expected only 'a' ready, got %+v", idsOf(ready))
	}

	// Mark 'a' completed.
	g.SetState("a", StateCompleted)
	ready = g.ReadyNodes()
	got := idsOf(ready)
	sort.Strings(got)
	want := []string{"b", "c"}
	if !equalStrings(got, want) {
		t.Fatalf("expected [b c] ready, got %v", got)
	}

	// Mark b and c completed.
	g.SetState("b", StateCompleted)
	g.SetState("c", StateCompleted)
	ready = g.ReadyNodes()
	if len(ready) != 1 || ready[0].ID != "d" {
		t.Fatalf("expected only 'd' ready, got %v", idsOf(ready))
	}
}

func TestTaskGraph_TerminalDetection(t *testing.T) {
	g := NewTaskGraph([]TaskNode{newNode("a")})
	if g.AllTerminal() {
		t.Fatal("expected not all terminal initially")
	}
	g.SetState("a", StateRunning)
	if g.AllTerminal() {
		t.Fatal("expected not terminal while running")
	}
	g.SetState("a", StateCompleted)
	if !g.AllTerminal() {
		t.Fatal("expected all terminal after completion")
	}
}

func TestTaskGraph_RunningNodes(t *testing.T) {
	g := NewTaskGraph([]TaskNode{newNode("a"), newNode("b", "a")})
	g.SetState("a", StateRunning)
	running := g.RunningNodes()
	if len(running) != 1 || running[0].ID != "a" {
		t.Fatalf("expected 'a' running, got %+v", idsOf(running))
	}
}

func TestTaskGraph_NodeByID(t *testing.T) {
	g := NewTaskGraph([]TaskNode{newNode("a"), newNode("b")})
	if _, ok := g.Node("a"); !ok {
		t.Fatal("expected to find node 'a'")
	}
	if _, ok := g.Node("missing"); ok {
		t.Fatal("expected 'missing' to be absent")
	}
}

func TestTaskGraph_ReadyNodesCascade(t *testing.T) {
	// After completing the only blocker of a chain, only the next step becomes ready.
	g := NewTaskGraph([]TaskNode{
		newNode("a"),
		newNode("b", "a"),
		newNode("c", "b"),
	})
	g.SetState("a", StateCompleted)
	ready := g.ReadyNodes()
	if len(ready) != 1 || ready[0].ID != "b" {
		t.Fatalf("expected 'b' ready, got %v", idsOf(ready))
	}
	g.SetState("b", StateCompleted)
	ready = g.ReadyNodes()
	if len(ready) != 1 || ready[0].ID != "c" {
		t.Fatalf("expected 'c' ready, got %v", idsOf(ready))
	}
}

func idsOf(nodes []TaskNode) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.ID)
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
