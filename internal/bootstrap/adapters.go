package bootstrap

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// turnOrchExecutor adapts sessionorchestrator.TurnOrchestrator to coordinator.TurnExecutor.
// DM-020 D-c: this replaces the legacy executor as the FastPath executor.
type turnOrchExecutor struct {
	orch sessionorchestrator.TurnOrchestrator
}

func newTurnOrchExecutor(orch sessionorchestrator.TurnOrchestrator) *turnOrchExecutor {
	return &turnOrchExecutor{orch: orch}
}

func (e *turnOrchExecutor) RunTurn(ctx context.Context, req sessionorchestrator.QueryRequest) (<-chan *contracts.EngineEvent, error) {
	if len(req.Messages) == 0 {
		return nil, fmt.Errorf("turn executor: at least one message required")
	}
	return e.orch.RunTurn(ctx, sessionorchestrator.TurnRequest{
		SessionID:    req.SessionID,
		UserMessage:  req.Messages[0],
		SystemPrompt: req.SystemPrompt,
		MaxTurns:     req.MaxTurns,
		Scope:        sessionorchestrator.TurnScopeMain,
	})
}

type gatewayEventPublisher struct {
	gw *capture.CommunicationGateway
}

func newGatewayEventPublisher(gw *capture.CommunicationGateway) *gatewayEventPublisher {
	return &gatewayEventPublisher{gw: gw}
}

func (p *gatewayEventPublisher) Publish(ctx context.Context, ev *contracts.EngineEvent) {
	if p.gw == nil || ev == nil {
		return
	}
	p.gw.PublishEngineEvent(ev)
}
