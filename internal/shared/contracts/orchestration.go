package contracts

import "context"

// IOrchestrationEntry is the contract D7 implements for the D1 capture.
//
// D1 RouteInbound calls ProcessMessage and routes the returned event channel
// to handleEngineEvents. Cancel is used by StopProcess and HandleInterrupt.
// D7 fans out to D2/D4 via internal executors — D1 MUST NOT call IEngine.Process.
type IOrchestrationEntry interface {
	ProcessMessage(ctx context.Context, sessionID, message string) (<-chan *EngineEvent, error)
	// Cancel cancels any in-flight orchestration for the given session.
	// Idempotent. Used by D1 StopProcess and HandleInterrupt.
	Cancel(ctx context.Context, sessionID string) error
}

// ISessionSnapshotExporter persists in-memory session context for D1 session store.
// Implemented by D2 ContextEngine; wired into capture via SetSessionSnapshotExporter.
type ISessionSnapshotExporter interface {
	ExportSessionSnapshot(sessionID string) ([]byte, error)
}
