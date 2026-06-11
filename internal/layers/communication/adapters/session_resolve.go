package adapters

import (
	"sync"

	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/shared/types"
)

// resolveOrCreateSession reuses the in-memory map, restores from disk, or creates fresh.
func resolveOrCreateSession(
	gw gateway.GatewayAPI,
	sessionMap *sync.Map,
	sessionKey string,
) (*types.Session, error) {
	if existingSessionID, ok := sessionMap.Load(sessionKey); ok {
		if sid, ok := existingSessionID.(string); ok && sid != "" {
			session, err := gw.GetSession(sid)
			if err == nil && session != nil {
				return session, nil
			}
		}
		sessionMap.Delete(sessionKey)
	}

	if session, err := gw.ResolveSessionByChatID(sessionKey); err != nil {
		return nil, err
	} else if session != nil {
		sessionMap.Store(sessionKey, session.SessionID)
		return session, nil
	}

	session, err := gw.CreateSession(sessionKey, "")
	if err != nil {
		return nil, err
	}
	sessionMap.Store(sessionKey, session.SessionID)
	return session, nil
}
