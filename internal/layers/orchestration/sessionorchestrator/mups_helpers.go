package sessionorchestrator

import (
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

func mupsToolSchemasFromPrepared(tools []contracts.MUPSToolDescriptor) []ToolSchema {
	out := make([]ToolSchema, len(tools))
	for i, t := range tools {
		out[i] = ToolSchema{Name: t.Name, Description: t.Description}
	}
	return out
}

func buildObserveMUPSRequest(in ObserveSignalInput, locale string) contracts.MUPSContextRequest {
	return contracts.MUPSContextRequest{
		Phase: contracts.MUPSPhaseObserve,
		Turn: &contracts.MUPSTurnContext{
			SessionID: in.SessionID,
		},
		UserMessage: in.Directive,
		WorkItem: &contracts.MUPSWorkItemSnapshot{
			ID:        in.WorkItemID,
			Directive: in.Directive,
		},
		Policy: contracts.MUPSContextPolicy{Locale: locale},
	}
}

func buildPlanMUPSRequest(in StrategicPlanInput, locale string) contracts.MUPSContextRequest {
	return contracts.MUPSContextRequest{
		Phase: contracts.MUPSPhasePlan,
		Turn: &contracts.MUPSTurnContext{
			SessionID: in.SessionID,
		},
		UserMessage:          in.Directive,
		ContractDimensionDoc: workmodel.ContractDimensionPromptDoc(),
		WorkItem: &contracts.MUPSWorkItemSnapshot{
			ID:        in.WorkItemID,
			Directive: in.Directive,
		},
		Policy: contracts.MUPSContextPolicy{Locale: locale},
	}
}

func buildExecuteMUPSRequest(ctx workItemExecContextBundle) contracts.MUPSContextRequest {
	item := ctx.Item
	req := contracts.MUPSContextRequest{
		Phase:       contracts.MUPSPhaseExecute,
		TaskKind:    taskKindForWorkItem(item),
		ToolProfile: toolProfileForItemWithTasks(ctx.SessionID, item, ctx.Tasks),
		Turn: &contracts.MUPSTurnContext{
			SessionID: ctx.SessionID,
		},
		WorkItem: workItemToMUPSSnapshot(ctx.SessionID, item, ctx.Tasks),
		Policy: contracts.MUPSContextPolicy{
			TokenBudget: ctx.TokenBudget,
			Depth:       ctx.Depth,
		},
	}
	if item != nil {
		req.WorkItem.PriorVerifyReason = ctx.PriorVerifyReason
	}
	return req
}

type workItemExecContextBundle struct {
	SessionID         string
	Item              *workmodel.WorkItem
	Tasks             *workmodel.TaskManager
	TokenBudget       int
	Depth             int
	PriorVerifyReason string
}

func workItemToMUPSSnapshot(sessionID string, item *workmodel.WorkItem, tm *workmodel.TaskManager) *contracts.MUPSWorkItemSnapshot {
	if item == nil {
		return nil
	}
	partition := ResolvePartitionForWorkItem(sessionID, item)
	expectedReturn := workmodel.ExpectedReturnForItem(tm, sessionID, item)
	wi := &contracts.MUPSWorkItemSnapshot{
		ID:             item.ID,
		Directive:      item.Directive,
		ExpectedReturn: expectedReturn,
		PriorVerifyReason: "",
		Partition: contracts.MUPSPartition{
			SessionID:        partition.SessionID,
			WorkItemID:       partition.WorkItemID,
			ParentWorkItemID: partition.ParentWorkItemID,
		},
	}
	schema := workmodel.DeliverableSchemaNotApplicable
	if item.LastRound != nil && item.LastRound.DeliverableSchema != workmodel.DeliverableSchemaNotApplicable {
		schema = item.LastRound.DeliverableSchema
	} else {
		schema = workmodel.InferDeliverableSchema(item, item.Directive, expectedReturn)
	}
	if schema != workmodel.DeliverableSchemaNotApplicable {
		wi.DeliverableSchema = string(schema)
	}
	if item.ScopeContract != nil {
		wi.ScopeContract = &contracts.MUPSScopeContract{
			GoalStatement: item.ScopeContract.GoalStatement,
			InScope:       append([]string(nil), item.ScopeContract.InScope...),
			OutOfScope:    append([]string(nil), item.ScopeContract.OutOfScope...),
		}
	}
	if tm != nil {
		if dl, ok := tm.ChildDownlinkFor(sessionID, item.ID); ok {
			wi.ScopeIn = append([]string(nil), dl.ScopeIn...)
			wi.ScopeOut = append([]string(nil), dl.ScopeOut...)
			if dl.ExpectedReturn != "" {
				wi.ExpectedReturn = dl.ExpectedReturn
			}
		}
	}
	return wi
}

func taskKindForWorkItem(item *workmodel.WorkItem) string {
	if item == nil {
		return ""
	}
	switch item.Kind {
	case workmodel.WorkKindExplore:
		return "observe"
	default:
		return "review"
	}
}

func mergeMUPSPreparedSystem(prepared contracts.MUPSPreparedContext) string {
	return prepared.SystemPrompt
}

func mupsMessagesWithDirective(sessionID, directive string, prepared contracts.MUPSPreparedContext) []types.Message {
	if len(prepared.Messages) > 0 {
		return prepared.Messages
	}
	if directive == "" {
		return nil
	}
	return []types.Message{{
		SessionID: sessionID,
		Role:      types.MessageRoleUser,
		Content:   directive,
	}}
}
