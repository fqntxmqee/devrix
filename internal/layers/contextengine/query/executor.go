package query

import (
	"context"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/shared/types"
)

// executeToolRefs runs tools and returns records + results for one turn.
func (l *Loop) executeToolRefs(
	turnCtx context.Context,
	sc *types.SessionContext,
	refs []conversation.ToolCallRef,
	emit EmitFunc,
	endTurnFn func(),
) ([]types.ToolCallRecord, []ToolRoundResult) {
	var allRecords []types.ToolCallRecord
	var toolResults []ToolRoundResult

	if l.StreamingTools && len(refs) > 1 {
		exec := &StreamingToolExecutor{
			Tools:                 l.Tools,
			Permission:            l.Permission,
			WrapToolContext:       l.WrapToolContext,
			Emit:                  emit,
			WrapToolStreamEmitter: l.WrapToolStreamContext,
		}
		batchRefs := make([]BatchToolRef, len(refs))
		for i, ref := range refs {
			batchRefs[i] = BatchToolRef{ID: ref.ID, Name: ref.Name, Input: ref.Input}
		}
		batch := exec.ExecuteBatch(turnCtx, sc, batchRefs)
		for i, res := range batch {
			ref := refs[i]
			content := conversation.FormatToolResultContent(ref.Name, res.Output, res.Error)
			if emit != nil {
				emitToolCall(emit, sc, ref)
				emitToolResult(emit, sc.SessionID, ref.Name, content, res.Error)
			}
			allRecords = append(allRecords, types.ToolCallRecord{
				CallID: ref.ID, ToolName: ref.Name, Input: ref.Input, Output: res.Output, Error: res.Error,
			})
			toolResults = append(toolResults, ToolRoundResult{Name: ref.Name, Output: res.Output, Error: res.Error})
		}
		return allRecords, toolResults
	}

	for _, ref := range refs {
		if l.Permission != nil && !l.Permission.Request(turnCtx, sc.SessionID, ref.Name, ref.Input) {
			endTurnFn()
			continue
		}
		if emit != nil {
			emitToolCall(emit, sc, ref)
		}
		out, errMsg, execErr := "", "", error(nil)
		if l.Tools != nil {
			toolCtx := turnCtx
			if l.WrapToolContext != nil {
				toolCtx = l.WrapToolContext(turnCtx, sc)
			}
			if emit != nil && l.WrapToolStreamContext != nil {
				toolCtx = l.WrapToolStreamContext(toolCtx, emit, sc.SessionID, ref.Name)
			}
			out, errMsg, execErr = l.Tools.Execute(toolCtx, ToolCall{ID: ref.ID, Name: ref.Name, Input: ref.Input})
		}
		if execErr != nil && errMsg == "" {
			errMsg = execErr.Error()
		}
		content := conversation.FormatToolResultContent(ref.Name, out, errMsg)
		if emit != nil {
			emitToolResult(emit, sc.SessionID, ref.Name, content, errMsg)
		}
		allRecords = append(allRecords, types.ToolCallRecord{
			CallID: ref.ID, ToolName: ref.Name, Input: ref.Input, Output: out, Error: errMsg,
		})
		toolResults = append(toolResults, ToolRoundResult{Name: ref.Name, Output: out, Error: errMsg})
	}
	return allRecords, toolResults
}
