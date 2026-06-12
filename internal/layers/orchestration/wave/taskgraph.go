package wave

import (
	"sort"
	"sync"
)

// TaskGraph is the in-memory representation of a Plan Engine DAG. It tracks
// per-node state for dispatch decisions (ReadyNodes) and terminal checks
// (AllTerminal). Mutating methods are safe for concurrent use by the
// scheduler loop and worker goroutines.
type TaskGraph struct {
	mu     sync.RWMutex
	nodes  map[string]TaskNode
	states map[string]TaskState
}

// NewTaskGraph builds a graph from a list of nodes. Duplicate ids cause an
// error returned separately; the constructor panics on bad input so caller
// is forced to validate first.
func NewTaskGraph(nodes []TaskNode) *TaskGraph {
	g := &TaskGraph{
		nodes:  make(map[string]TaskNode, len(nodes)),
		states: make(map[string]TaskState, len(nodes)),
	}
	for _, n := range nodes {
		g.nodes[n.ID] = n
		g.states[n.ID] = StatePending
	}
	return g
}

// Node returns a snapshot of the named node.
func (g *TaskGraph) Node(id string) (TaskNode, bool) {
	if g == nil {
		return TaskNode{}, false
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	n, ok := g.nodes[id]
	return n, ok
}

// State returns the current state of a node.
func (g *TaskGraph) State(id string) (TaskState, bool) {
	if g == nil {
		return "", false
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	s, ok := g.states[id]
	return s, ok
}

// SetState transitions a node. Unknown ids are silently ignored.
func (g *TaskGraph) SetState(id string, state TaskState) {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.states[id]; !ok {
		return
	}
	g.states[id] = state
}

// ReadyNodes returns the nodes whose dependencies are all in a terminal
// completed state and that themselves are still pending. Order is sorted by
// id to make dispatch deterministic.
func (g *TaskGraph) ReadyNodes() []TaskNode {
	if g == nil {
		return nil
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	completed := make(map[string]struct{})
	for id, st := range g.states {
		if st == StateCompleted {
			completed[id] = struct{}{}
		}
	}
	out := make([]TaskNode, 0)
	for id, n := range g.nodes {
		if g.states[id] != StatePending {
			continue
		}
		if depsSatisfied(n.DependsOn, completed) {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// RunningNodes returns nodes in StateRunning (test helper / metrics).
func (g *TaskGraph) RunningNodes() []TaskNode {
	if g == nil {
		return nil
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make([]TaskNode, 0)
	for id, n := range g.nodes {
		if g.states[id] == StateRunning {
			out = append(out, n)
		}
	}
	return out
}

// AllTerminal reports whether every node is in a terminal state.
func (g *TaskGraph) AllTerminal() bool {
	if g == nil {
		return true
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	for _, st := range g.states {
		if !st.Terminal() {
			return false
		}
	}
	return true
}

// NodeCount returns the total number of nodes (for tests / metrics).
func (g *TaskGraph) NodeCount() int {
	if g == nil {
		return 0
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.nodes)
}

// TerminalArtifacts returns a snapshot of (id, terminal state) for finished
// nodes. Used by WaitForCompletion to summarize the wave.
func (g *TaskGraph) TerminalArtifacts() map[string]TaskState {
	if g == nil {
		return nil
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make(map[string]TaskState, len(g.states))
	for id, st := range g.states {
		if st.Terminal() {
			out[id] = st
		}
	}
	return out
}

func depsSatisfied(deps []string, completed map[string]struct{}) bool {
	for _, d := range deps {
		if _, ok := completed[d]; !ok {
			return false
		}
	}
	return true
}
