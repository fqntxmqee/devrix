package guard

import (
	"context"
	"log/slog"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/google/uuid"
)

// AgentObserver is the public interface that other packages can use.
type AgentObserver = multiagent.AgentObserver

// OrchestrationObserver implements multiagent.AgentObserver and bridges agent
// lifecycle events into the validation pipeline.
type OrchestrationObserver struct {
	validator *RuntimeOrchestrationValidator
	ctx       context.Context
	session   *types.Session
}

// NewOrchestrationObserver creates an observer that feeds agent events into the validator.
func NewOrchestrationObserver(
	validator *RuntimeOrchestrationValidator,
	ctx context.Context,
	session *types.Session,
) *OrchestrationObserver {
	slog.Info("orchestration: observer created",
		"session_id", session.SessionID,
		"validator_enabled", validator.config.Enabled,
	)
	validator.metrics.signalObserverActive()
	return &OrchestrationObserver{
		validator: validator,
		ctx:       ctx,
		session:   session,
	}
}

// EmitAgentEvent implements multiagent.AgentObserver.
func (o *OrchestrationObserver) EmitAgentEvent(ev multiagent.AgentEvent) {
	var rec *DecisionRecord

	switch ev.EventType {
	case "permission_required":
		toolName, _ := ev.Metadata["tool"].(string)
		if toolName == "" {
			return
		}
		rec = &DecisionRecord{
			ID:            uuid.New().String(),
			SessionID:     ev.SessionID,
			AgentID:       ev.AgentID,
			ParentAgentID: ev.ParentID,
			Category:      DecisionPermit,
			RiskClass:     RiskCritical,
			Timestamp:     ev.Timestamp,
			ToolName:      toolName,
			SessionState:  o.session.State,
		}

	case "agent.forked":
		childID, _ := ev.Metadata["child_id"].(string)
		if childID == "" {
			return
		}
		rec = &DecisionRecord{
			ID:            uuid.New().String(),
			SessionID:     ev.SessionID,
			AgentID:       ev.AgentID,
			ParentAgentID: ev.ParentID,
			Category:      DecisionFork,
			RiskClass:     RiskEvaluate,
			Timestamp:     ev.Timestamp,
			TargetAgentID: childID,
			SessionState:  o.session.State,
		}
	}

	if rec == nil {
		return
	}
	o.validator.OnDecision(o.ctx, *rec, o.session)
}

var _ multiagent.AgentObserver = (*OrchestrationObserver)(nil)
