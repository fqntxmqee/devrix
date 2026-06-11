package gateway

import (
	"log/slog"

	"github.com/devrix/devrix/internal/shared/types"
)

type sessionSnapshotExporter interface {
	ExportSessionSnapshot(sessionID string) ([]byte, error)
}

func (g *CommunicationGateway) syncSnapshotFromEngine(session *types.Session) {
	if g == nil || g.contextEngine == nil || session == nil {
		return
	}
	exp, ok := g.contextEngine.(sessionSnapshotExporter)
	if !ok {
		return
	}
	data, err := exp.ExportSessionSnapshot(session.SessionID)
	if err != nil {
		slog.Debug("gateway: snapshot export skipped", "sessionID", session.SessionID, "error", err)
		return
	}
	if len(data) > 0 {
		session.ContextSnapshot = data
	}
}

func (g *CommunicationGateway) persistSessionAfterProcess(session *types.Session) {
	g.syncSnapshotFromEngine(session)
	if err := g.sessionStore.Update(session); err != nil {
		slog.Warn("failed to persist session after process", "sessionID", session.SessionID, "error", err)
	}
}
