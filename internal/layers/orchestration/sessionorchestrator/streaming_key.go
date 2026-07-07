// Package sessionorchestrator — streaming idempotency key formatters
// (DM-20260707-001 PR-C).
//
// These formatters live here (NOT in communication/channel/adapters) so
// sessionorchestrator can construct the same key shape the adapter uses
// without taking an import dependency on adapters — which would otherwise
// create a test-only cycle through communication/capture. The adapter
// side has its own typed IdempotencyKey that wraps these strings; the
// shape ("{sessionID}:seg:{segmentID}" and "{sessionID}:rollup:{parentID}")
// is the single shared keyspace both sides agree on.
//
// Why plain string here: the StreamingEmitter interface in execute_plan_dag.go
// uses plain string so the interface can be implemented by any future IM
// adapter (Slack, Teams, ...) without dragging the adapters package's
// import graph through sessionorchestrator.
package sessionorchestrator

import "fmt"

// NewPartialIdempotencyKey formats the per-segment partial key. Mirrors
// adapters.NewPartialIdempotencyKey — same keyshape, separate function
// (no shared package dep).
func NewPartialIdempotencyKey(sessionID, segmentID string) string {
	return fmt.Sprintf("%s:seg:%s", sessionID, segmentID)
}

// NewRollupIdempotencyKey formats the parent-rollup final key. The
// "rollup:" prefix avoids collision with "seg:" prefixed partial keys.
// Mirrors adapters.NewRollupIdempotencyKey.
func NewRollupIdempotencyKey(sessionID, parentID string) string {
	return fmt.Sprintf("%s:rollup:%s", sessionID, parentID)
}