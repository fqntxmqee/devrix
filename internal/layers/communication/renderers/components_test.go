package renderers

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

func TestMilestoneCard_Render(t *testing.T) {
	m := types.NewMilestone("m1", "t1", "Design")
	m.SetProgress(0.5)
	m.SetStatus(types.MilestoneStatusInProgress)

	out := NewMilestoneCard(m).Render()
	if out == "" {
		t.Fatal("expected non-empty render output")
	}
}

func TestDingTalkCardRenderer_RenderMilestone(t *testing.T) {
	m := types.NewMilestone("m1", "t1", "Implement")
	m.SetProgress(0.8)
	m.SetStatus(types.MilestoneStatusInProgress)

	out := NewDingTalkCardRenderer().RenderMilestone(m)
	if out == "" {
		t.Fatal("expected markdown output")
	}
}
