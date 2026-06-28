package testutil

import (
	"github.com/devrix/devrix/internal/bootstrap/sessionagents"
	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/multiagent"
)

// WireGatewaySessionAgents wires D4 session leader provision outside D1 capture.
func WireGatewaySessionAgents(gw *capture.CommunicationGateway, factory multiagent.IAgentFactory) *sessionagents.Manager {
	if gw == nil || factory == nil {
		return nil
	}
	mgr := sessionagents.NewManager(factory)
	mgr.SetPermissionRouter(gw)
	mgr.SetActiveProcessChecker(gw)
	mgr.SetOrphanEngineEventSink(gw.DeliverOrphanEngineEvent)
	gw.SetBeforeDispatch(mgr.EnsureSessionLeader)
	return mgr
}
