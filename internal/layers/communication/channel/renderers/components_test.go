package renderers

import (
	"testing"

	"strings"

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

// T: D1-S8-A01-T02
func TestProgressBar_Render(t *testing.T) {
	out := NewProgressBar(0.5, 10).Render()
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if !containsAll(out, "50%") {
		t.Fatalf("output = %q", out)
	}
}

// T: D1-S8-A01-T02
func TestStatusBadge_Render(t *testing.T) {
	out := NewStatusBadge("in_progress").Render()
	if out == "" {
		t.Fatal("expected non-empty output")
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}
