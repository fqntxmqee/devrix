package kernel

import "github.com/devrix/devrix/internal/shared/types"

// Emitter delivers outbound messages to adapters (S17 Encode downstream).
//
// DSAFT: D1-S14–S16 → S17
type Emitter interface {
	OnMessage(msg *types.OutboundMessage)
	OnStatus(sessionID string, state types.SessionState)
}
