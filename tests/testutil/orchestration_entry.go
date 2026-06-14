package testutil

import (
	"context"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// EngineOrchestrationEntry adapts IEngine to IOrchestrationEntry for tests.
// Simulates D7 dispatch without wiring the full coordinator stack.
type EngineOrchestrationEntry struct {
	Engine capture.IContextEngine
}

var _ contracts.IOrchestrationEntry = (*EngineOrchestrationEntry)(nil)

func NewEngineOrchestrationEntry(engine capture.IContextEngine) *EngineOrchestrationEntry {
	return &EngineOrchestrationEntry{Engine: engine}
}

func (e *EngineOrchestrationEntry) ProcessMessage(ctx context.Context, sessionID, message string) (<-chan *contracts.EngineEvent, error) {
	if e.Engine == nil {
		ch := make(chan *contracts.EngineEvent)
		close(ch)
		return ch, nil
	}
	session := types.NewSession(sessionID, "test", "")
	return e.Engine.Process(ctx, session, message), nil
}

func (e *EngineOrchestrationEntry) Cancel(_ context.Context, _ string) error {
	return nil
}

// WireGatewayOrchestration attaches a test engine via the D7 entry seam.
func WireGatewayOrchestration(gw *capture.CommunicationGateway, engine capture.IContextEngine) {
	if gw == nil || engine == nil {
		return
	}
	gw.SetOrchestrationEntry(NewEngineOrchestrationEntry(engine))
	if exp, ok := engine.(contracts.ISessionSnapshotExporter); ok {
		gw.SetSessionSnapshotExporter(exp)
	}
}
