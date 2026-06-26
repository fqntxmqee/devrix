package workmodel

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
)

func TestWaveNodesFromSubtree_UpstreamWhenFlagOn(t *testing.T) {
	t.Setenv(FeatureWorkItemContextGraphEnv, "1")
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

func TestWaveNodesFromSubtree_FreshWhenFlagOff(t *testing.T) {
	t.Setenv(FeatureWorkItemContextGraphEnv, "0")
	tm := NewTaskManager()
	batch, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{Kind: WorkKindPlan, Title: "batch"})
	upstream, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{
		ParentID: batch.ID, Kind: WorkKindImplement, Title: "upstream",
	})
	dependent, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{
		ParentID: batch.ID, Kind: WorkKindImplement, Title: "dependent",
	})
	_ = tm.Tree().AddDependency("s1", dependent.ID, upstream.ID)

	nodes := tm.WaveNodesFromSubtree("s1", batch.ID)
	for _, n := range nodes {
		if n.ID == dependent.ID && n.ContextPolicy != wavescheduler.ContextFresh {
			t.Fatalf("flag off: policy=%q want fresh", n.ContextPolicy)
		}
	}
}
