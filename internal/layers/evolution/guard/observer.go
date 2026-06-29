package guard

import (
	"context"
	"log/slog"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/orchtypes"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/google/uuid"
)

// AgentObserver is the public interface that other packages can use.
type AgentObserver = multiagent.AgentObserver

// GuardObserver implements multiagent.AgentObserver and bridges agent
// lifecycle events into the validation pipeline.
//
// PR-B (DM-20260621-011): renamed from OrchestrationObserver to align with
// guard/ domain naming (v2.0 物理路径迁移时仅迁移目录, 类型名未同步).
type GuardObserver struct {
	validator *RuntimeGuardValidator
	ctx       context.Context
	session   *types.Session
}

// NewGuardObserver creates an observer that feeds agent events into the validator.
func NewGuardObserver(
	validator *RuntimeGuardValidator,
	ctx context.Context,
	session *types.Session,
) *GuardObserver {
	if validator != nil && session != nil {
		slog.Info("guard: observer created",
			"session_id", session.SessionID,
			"validator_enabled", validator.config.Enabled,
		)
	}
	if validator != nil {
		validator.metrics.signalObserverActive()
	}
	return &GuardObserver{
		validator: validator,
		ctx:       ctx,
		session:   session,
	}
}

// EmitAgentEvent implements multiagent.AgentObserver.
func (o *GuardObserver) EmitAgentEvent(ev multiagent.AgentEvent) {
	var rec *DecisionRecord

	switch ev.EventType {
	case orchtypes.EventPermissionRequired:
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

	case orchtypes.EventAgentForked:
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

var _ multiagent.AgentObserver = (*GuardObserver)(nil)

// OrchestrationObserver is an alias for GuardObserver kept for backward compatibility.
//
// Deprecated: use GuardObserver. This alias will be removed in v2.5.0 (DM-20260621-011).
//go:deprecated
type OrchestrationObserver = GuardObserver

// NewOrchestrationObserver is the deprecated constructor for GuardObserver.
//
// Deprecated: use NewGuardObserver. This alias will be removed in v2.5.0 (DM-20260621-011).
//go:deprecated
func NewOrchestrationObserver(
	validator *RuntimeGuardValidator,
	ctx context.Context,
	session *types.Session,
) *GuardObserver {
	return NewGuardObserver(validator, ctx, session)
}
