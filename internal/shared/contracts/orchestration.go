package contracts

import "context"

// IOrchestrationEntry is the contract D7 implements for the D1 capture.
//
// D1 RouteInbound (when orchestration.d7_enabled=true) calls ProcessMessage
// and routes the returned event channel to the existing gateway
// handleEngineEvents pipeline. Cancel is used by D1 StopProcess and
// HandleInterrupt.
//
// D7 must NOT call into D2 internals; it goes through IEngine.RunQueryLoop
// (see contracts.IEngine). This interface is the gateway-side seam that
// keeps the dependency direction correct (D1 → D7 → D2/D4).
type IOrchestrationEntry interface {
	ProcessMessage(ctx context.Context, sessionID, message string) (<-chan *EngineEvent, error)
	// Cancel cancels any in-flight orchestration for the given session.
	// Idempotent. Used by D1 StopProcess and HandleInterrupt.
	Cancel(ctx context.Context, sessionID string) error
}
