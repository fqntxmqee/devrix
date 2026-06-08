package renderers

import (
	"github.com/devrix/devrix/internal/layers/communication/core"
	"github.com/devrix/devrix/internal/shared/types"
)

// DingTalkCardRenderer renders cards for DingTalk (markdown fallback).
type DingTalkCardRenderer struct{}

// NewDingTalkCardRenderer creates a DingTalk card renderer.
func NewDingTalkCardRenderer() *DingTalkCardRenderer {
	return &DingTalkCardRenderer{}
}

// RenderMilestone renders a milestone card as markdown text.
func (r *DingTalkCardRenderer) RenderMilestone(m *types.Milestone) string {
	if m == nil {
		return ""
	}
	card := core.NewCard().
		Title(m.Name, "blue").
		Markdownf("状态: %s", m.Status).
		Markdown(NewProgressBar(m.Progress, 20).Render()).
		Build()
	return card.RenderText()
}

// RenderTaskFlow renders a task flow summary for DingTalk.
func (r *DingTalkCardRenderer) RenderTaskFlow(tf *types.TaskFlow) string {
	if tf == nil {
		return ""
	}
	return NewTaskFlowSummary(tf).Render()
}
