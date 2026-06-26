package workmodel

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
)

func TestWaveNodesFromSubtree_UpstreamWhenBlockedBy(t *testing.T) {
	tm := NewTaskManager()
	batch, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{
		Kind: WorkKindPlan, Title: "batch", Policy: ExecPolicyParallelOK,
	})
	upstream, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{
		ParentID: batch.ID, Kind: WorkKindImplement, Title: "upstream", Directive: "up",
	})
	dependent, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{
		ParentID: batch.ID, Kind: WorkKindImplement, Title: "dependent", Directive: "dep",
	})
	_ = tm.Tree().AddDependency("s1", dependent.ID, upstream.ID)

	nodes := tm.WaveNodesFromSubtree("s1", batch.ID)
	var depNode *wavescheduler.TaskNode
	for i := range nodes {
		if nodes[i].ID == dependent.ID {
			depNode = &nodes[i]
			break
		}
	}
	if depNode == nil {
		t.Fatal("dependent node not projected")
	}
	if depNode.ContextPolicy != wavescheduler.ContextUpstream {
		t.Fatalf("policy=%q want upstream", depNode.ContextPolicy)
	}
	if depNode.UpstreamTaskID != upstream.ID {
		t.Fatalf("upstream_id=%q want %q", depNode.UpstreamTaskID, upstream.ID)
	}
}

func TestWaveNodesFromSubtree_FreshWithoutBlockedBy(t *testing.T) {
	tm := NewTaskManager()
	batch, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{Kind: WorkKindPlan, Title: "batch"})
	_, _ = tm.CreateWorkItem("s1", CreateWorkItemInput{
		ParentID: batch.ID, Kind: WorkKindImplement, Title: "solo", Directive: "solo",
	})

	nodes := tm.WaveNodesFromSubtree("s1", batch.ID)
	if len(nodes) != 1 || nodes[0].ContextPolicy != wavescheduler.ContextFresh {
		t.Fatalf("nodes=%+v", nodes)
	}
}
