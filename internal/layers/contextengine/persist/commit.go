package persist

import (
	"fmt"

	"github.com/devrix/devrix/internal/shared/types"
)

// CommitDeps holds dependencies for S17-A04 CommitWindow operations.
type CommitDeps struct {
	Store     MessageStore
	Bootstrap SessionBootstrap
}

// AppendAndTrimMessages appends messages to the session working set and trims
// to the configured token budget (D2-S17-A04 CommitWindow).
//
// DM-20260617-003: bridges D7 SessionPersister.PersistTurn to D2 memory.
func AppendAndTrimMessages(deps CommitDeps, sessionID string, msgs []types.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	if deps.Store == nil {
		return fmt.Errorf("persist: MessageStore is nil")
	}
	sc, ok := deps.Store.Get(sessionID)
	if !ok || sc == nil {
		if deps.Bootstrap == nil {
			return fmt.Errorf("persist: session %s not found and no bootstrap", sessionID)
		}
		var err error
		sc, err = deps.Bootstrap(sessionID)
		if err != nil {
			return fmt.Errorf("persist: bootstrap session %s: %w", sessionID, err)
		}
		if sc == nil {
			return fmt.Errorf("persist: bootstrap returned nil for %s", sessionID)
		}
	}
	for i := range msgs {
		deps.Store.AppendFullMessage(sc, msgs[i])
	}
	deps.Store.TrimMessages(sc)
	return nil
}
