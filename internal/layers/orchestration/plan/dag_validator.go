package plan

import (
	"errors"
	"fmt"

	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// =====================================================================
// PlanDAG validator (DM-20260707-001 PR-A1 T13/T14)
//
// Validation is intentionally a grammar check — PlanDAG is data with no
// behavior. The DAG executor (PR-B) wires the runtime fan-out cap (4) to
// WaveScheduler; this validator owns the AUTHORING cap (8 fan-out, 10 nodes).
//
//   8 vs 4 distinction:
//     - Authoring cap = 8: a single PlanNode may declare up to 8 outgoing
//       edges; catches over-fragmented graphs at authoring time.
//     - Runtime cap   = 4: WaveScheduler hard limit on concurrent workers
//       (resource guard). This is a separate concept and lives in PR-B.
//
// The validator is invoked from Plan.Validate() when Plan.DAG != nil; the
// caller passes ValidateOpts to override the authoring caps (mostly for
// tests; production uses the defaults).
//
// Validation order (cheapest first, fail-fast):
//   1. empty (DAG.Nodes empty → invalid state — an empty DAG with edges is
//      a contradiction; a DAG with neither is harmless and skipped).
//   2. duplicate-id (catches authoring typos).
//   3. too-many-nodes (resource budget).
//   4. fan-out (per-node outgoing edge cap).
//   5. dangling-edge (edge endpoint not in node set).
//   6. cycle (topological sort; self-loop is a 1-cycle).
// =====================================================================

// Sentinel errors for PlanDAG validation. They follow the existing
// plan/errors.go pattern: inner error returned unwrapped, wrap helpers
// expose canonical ORCH_*_7xxx codes.
//
// Code allocation (PR-A1, see reviews/pr-a1-consensus-packet.md):
//   ORCH_DAG_EMPTY_7200
//   ORCH_DAG_DUPLICATE_NODE_7201
//   ORCH_DAG_TOO_MANY_NODES_7202
//   ORCH_DAG_FANOUT_EXCEEDED_7203
//   ORCH_DAG_DANGLING_EDGE_7204
//   ORCH_DAG_CYCLE_7205
var (
	ErrPlanDAGEmpty           = errors.New("plan: PlanDAG is empty (Nodes must be non-empty when DAG is present)")
	ErrPlanDAGDuplicateNodeID = errors.New("plan: PlanDAG has duplicate node ID")
	ErrPlanDAGTooManyNodes    = errors.New("plan: PlanDAG exceeds max nodes limit")
	ErrPlanDAGFanOutExceeded  = errors.New("plan: PlanDAG node exceeds fan-out limit")
	ErrPlanDAGDanglingEdge    = errors.New("plan: PlanDAG edge endpoint not in node set")
	ErrPlanDAGContainsCycle   = errors.New("plan: PlanDAG contains a cycle (topological sort failed)")
)

// Authoring cap defaults. Override via ValidateOpts.
//   - MaxDAGNodes = 10  (per-DAG node count, excludes edges).
//   - MaxFanOut   = 8   (per-node outgoing edge count).
// Runtime fan-out cap (4) is enforced by WaveScheduler, not here.
const (
	DefaultMaxDAGNodes = 10
	DefaultMaxFanOut   = 8
)

// PlanDAGValidationOpts tunes validateDAG thresholds. Zero-value falls back
// to the Default* constants above.
type PlanDAGValidationOpts struct {
	MaxDAGNodes int
	MaxFanOut   int
}

func (o PlanDAGValidationOpts) maxNodes() int {
	if o.MaxDAGNodes > 0 {
		return o.MaxDAGNodes
	}
	return DefaultMaxDAGNodes
}

func (o PlanDAGValidationOpts) maxFanOut() int {
	if o.MaxFanOut > 0 {
		return o.MaxFanOut
	}
	return DefaultMaxFanOut
}

// validateDAG checks a non-nil PlanDAG for authoring correctness. The
// nil-DAG case is the caller's decision (Plan may legitimately have a
// nil DAG on the 4-channel PR-B1 path); this function MUST NOT be called
// with d == nil.
//
// Validation order is deterministic — the first failing check wins. This
// matches the existing Plan.Validate() convention (cheap first, fail-fast).
//
// Complexity: O(N + E) for the structural checks + O(N + E) for the cycle
// detection (Kahn's algorithm) = O(N + E) overall.
func validateDAG(d *PlanDAG, opts PlanDAGValidationOpts) error {
	// 1. Empty
	if len(d.Nodes) == 0 {
		return NewPlanDAGEmptyError()
	}

	// 2. Duplicate node IDs
	seen := make(map[string]struct{}, len(d.Nodes))
	for _, n := range d.Nodes {
		if _, dup := seen[n.ID]; dup {
			return NewPlanDAGDuplicateNodeIDError(n.ID)
		}
		seen[n.ID] = struct{}{}
	}

	// 3. Too many nodes
	if len(d.Nodes) > opts.maxNodes() {
		return NewPlanDAGTooManyNodesError(len(d.Nodes), opts.maxNodes())
	}

	// 4. Fan-out cap + build adjacency for the cycle check
	out := make(map[string][]string, len(d.Nodes))
	fanLimit := opts.maxFanOut()
	for _, e := range d.Edges {
		if _, ok := seen[e.From]; !ok {
			return NewPlanDAGDanglingEdgeError("from", e.From, e.To)
		}
		if _, ok := seen[e.To]; !ok {
			return NewPlanDAGDanglingEdgeError("to", e.To, e.From)
		}
		if len(out[e.From]) >= fanLimit {
			return NewPlanDAGFanOutExceededError(e.From, len(out[e.From])+1, fanLimit)
		}
		out[e.From] = append(out[e.From], e.To)
		// Self-loop fan-out still counts (1 outgoing edge to self).
		// Cycle check below catches the self-loop.
	}

	// 5. Cycle detection via Kahn's algorithm. A cycle is present iff the
	// topological order leaves ≥1 node unprocessed.
	indeg := make(map[string]int, len(d.Nodes))
	for id := range seen {
		indeg[id] = 0
	}
	for _, tos := range out {
		for _, to := range tos {
			indeg[to]++
		}
	}
	queue := make([]string, 0, len(d.Nodes))
	for id, d := range indeg {
		if d == 0 {
			queue = append(queue, id)
		}
	}
	processed := 0
	for len(queue) > 0 {
		head := queue[0]
		queue = queue[1:]
		processed++
		for _, to := range out[head] {
			indeg[to]--
			if indeg[to] == 0 {
				queue = append(queue, to)
			}
		}
	}
	if processed < len(d.Nodes) {
		return NewPlanDAGContainsCycleError(processed, len(d.Nodes))
	}

	return nil
}

// =====================================================================
// Wrap helpers — emit *sharederrors.SentinelError with stable
// ORCH_DAG_*_72xx codes for upstream audit logs / metrics.
// =====================================================================

func NewPlanDAGEmptyError() *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_DAG_EMPTY_7200",
		"PlanDAG is empty (Nodes must be non-empty when DAG is present)",
		ErrPlanDAGEmpty,
	)
}

func NewPlanDAGDuplicateNodeIDError(id string) *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_DAG_DUPLICATE_NODE_7201",
		fmt.Sprintf("PlanDAG has duplicate node ID %q", id),
		ErrPlanDAGDuplicateNodeID,
	)
}

func NewPlanDAGTooManyNodesError(observed, limit int) *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_DAG_TOO_MANY_NODES_7202",
		fmt.Sprintf("PlanDAG has %d nodes, limit is %d", observed, limit),
		ErrPlanDAGTooManyNodes,
	)
}

func NewPlanDAGFanOutExceededError(nodeID string, observed, limit int) *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_DAG_FANOUT_EXCEEDED_7203",
		fmt.Sprintf("PlanDAG node %q fan-out=%d exceeds limit %d", nodeID, observed, limit),
		ErrPlanDAGFanOutExceeded,
	)
}

func NewPlanDAGDanglingEdgeError(side, missingID, otherID string) *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_DAG_DANGLING_EDGE_7204",
		fmt.Sprintf("PlanDAG edge %s=%q not in node set (other=%q)", side, missingID, otherID),
		ErrPlanDAGDanglingEdge,
	)
}

func NewPlanDAGContainsCycleError(processed, total int) *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_DAG_CYCLE_7205",
		fmt.Sprintf("PlanDAG contains a cycle: Kahn processed %d/%d nodes", processed, total),
		ErrPlanDAGContainsCycle,
	)
}
