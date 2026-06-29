package workmodel

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

func TestRecordPeerStatusOnTerminal_CohortSizeGate(t *testing.T) {
	ResetContextGraphState()
	tm := NewTaskManager()
	parent, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{Kind: WorkKindPlan, Title: "p"})
	var ids []string
	for i := 0; i < MinCohortSizeForPeerStatus; i++ {
		c, err := tm.CreateWorkItem("s1", CreateWorkItemInput{
			ParentID: parent.ID, Kind: WorkKindExplore, Title: "sibling",
		})
		if err != nil {
			t.Fatalf("CreateWorkItem: %v", err)
		}
		ids = append(ids, c.ID)
	}
	if len(tm.Tree().ListChildren("s1", parent.ID)) < MinCohortSizeForPeerStatus {
		t.Fatalf("expected %d siblings", MinCohortSizeForPeerStatus)
	}
	targetID := ids[0]
	if err := tm.UpdateStatus("s1", targetID, TaskStatusInProgress); err != nil {
		t.Fatalf("UpdateStatus in_progress: %v", err)
	}
	if err := tm.UpdateStatus("s1", targetID, TaskStatusCompleted); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	target, ok := tm.GetWorkItem("s1", targetID)
	if !ok || target == nil {
		t.Fatal("target missing")
	}
	target.LastRound = &WorkItemPipelineRound{
		VerdictKind:     types.VerdictPass,
		ArtifactSummary: "peer done",
	}
	tm.RecordPeerStatusOnTerminal("s1", target)

	sigs := tm.PeerStatusSignalsForCohort("s1", parent.ID)
	if len(sigs) != 1 {
		t.Fatalf("expected 1 peer status, got %d", len(sigs))
	}
	line := PeerStatusLines(sigs)[0]
	if !strings.Contains(line, "peer_status:") || !strings.Contains(line, "peer done") {
		t.Fatalf("line = %q", line)
	}
}
