// Package layout enforces the D7 orchestration physical layout invariants
// declared in openspec/specs/architecture/code-layout.md §4.2.
//
// See openspec/changes/devrix-d7-physical-layout-alignment/ for the
// design rationale (DM-20260701-004) and D7-PL-T07..T12 in
// openspec/specs/d7-orchestration/t-registry.md for the failing conditions
// each guard test covers.
package layout