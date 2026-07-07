// Package plan — PlanDAG grammar (DM-20260707-001 PR-A1 T10-T12).
//
// PlanDAG is the per-segment execution graph emitted by the Plan node when a
// directive decomposes into ≥2 segments (multi-intent path, 方案 β). It is
// consumed by the DAG executor (PR-B) which routes each PlanNode to its own
// worker slot, up to the WaveScheduler hard cap of 4 concurrent workers.
//
// Boundaries (must remain explicit so a future PR doesn't "helpfully" wire
// these to the wrong place):
//
//   - PlanDAG is data — no behavior. Validity is owned by dag_validator.go.
//   - Plan.Validate() does NOT inspect DAG. Plan is orthogonal to the DAG.
//   - PR-A1 only ships the type surface + validator; PR-B wires the executor.
package plan

// PlanNode is the per-segment execution slot. One PlanNode corresponds 1:1
// to one IntentSegment in the parent IntentSegmentSet, but PlanNode itself
// does not carry the segment metadata — workers look it up via SegmentID
// against the parent Set.
//
// IDs must be unique within a PlanDAG; the validator enforces this.
type PlanNode struct {
	ID                   string   `json:"id"`
	SegmentID            string   `json:"segment_id"`
	WorkerHint           string   `json:"worker_hint,omitempty"`
	ExpectedArtifactTags []string `json:"expected_artifact_tags,omitempty"`
}

// DataEdge expresses a dependency edge between two PlanNodes. The
// DependsOnOutputs field is reserved for v2 cross-segment data flow (see
// proposal.md §8.2 v2-1). PR-A1 ignores this field; the validator does not
// dereference it. Presence of the field documents the planned extension so
// downstream consumers can plan without breaking the wire format.
//
// Future-scope: PR-A1 reserves the field. Validation / parse ignores the
// slice. Do NOT introduce a stage that consumes DependsOnOutputs without
// a separate Change covering the v2 protocol.
type DataEdge struct {
	From             string   `json:"from"`
	To               string   `json:"to"`
	DependsOnOutputs []string `json:"depends_on_outputs,omitempty"`
}

// PlanDAG is the per-segment execution graph. Nodes enumerate workers to
// spawn; Edges express dependencies; Priorities influence WaveScheduler
// ready-queue ordering (PR-B); MaxParallelism is informational — the
// WaveScheduler hard cap is 4 (resource guard) and the validator cap is 8
// (authoring guard, see dag_validator.go header).
type PlanDAG struct {
	Nodes          []PlanNode   `json:"nodes"`
	Edges          []DataEdge   `json:"edges,omitempty"`
	Priorities     map[string]int `json:"priorities,omitempty"`      // key must be a node ID
	MaxParallelism int            `json:"max_parallelism,omitempty"` // v1: ignored, hard cap 4
}
