package workmodel

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
)

func wi(id, parent string) *WorkItem {
	return &WorkItem{ID: id, ParentID: parent, Status: TaskStatusPending}
}

func TestClassifySiblingRelation_NotSibling(t *testing.T) {
	a := wi("a", "p1")
	b := wi("b", "p2")
	if got := ClassifySiblingRelation(a, b); got.Relation != SiblingNotSibling {
		t.Fatalf("got %q", got.Relation)
	}
}

func TestClassifySiblingRelation_Dependent(t *testing.T) {
	a := wi("a", "p1")
	b := wi("b", "p1")
	b.BlockedBy = []string{"a"}
	got := ClassifySiblingRelation(a, b)
	if got.Relation != SiblingDependent || got.UpstreamID != "a" || got.DependentID != "b" {
		t.Fatalf("got %+v", got)
	}
}

func TestClassifySiblingRelation_Independent(t *testing.T) {
	a := wi("a", "p1")
	b := wi("b", "p1")
	if got := ClassifySiblingRelation(a, b); got.Relation != SiblingIndependent {
		t.Fatalf("got %q", got.Relation)
	}
}

func TestClassifySiblingRelation_ParallelBatch(t *testing.T) {
	a := wi("a", "p1")
	b := wi("b", "p1")
	a.Policy = ExecPolicyParallelOK
	b.Policy = ExecPolicyParallelOK
	if got := ClassifySiblingRelation(a, b); got.Relation != SiblingParallelBatch {
		t.Fatalf("got %q", got.Relation)
	}
}

func TestClassifySiblingRelation_HumanReview(t *testing.T) {
	a := wi("a", "p1")
	b := &WorkItem{ID: "b", ParentID: "p1", Kind: WorkKindVerify, Title: HumanReviewItemTitle}
	if got := ClassifySiblingRelation(a, b); got.Relation != SiblingHumanReview {
		t.Fatalf("got %q", got.Relation)
	}
}

func TestClassifySiblingRelation_Projection(t *testing.T) {
	a := wi("a", "p1")
	b := &WorkItem{ID: "b", ParentID: "p1", Ephemeral: true}
	if got := ClassifySiblingRelation(a, b); got.Relation != SiblingProjection {
		t.Fatalf("got %q", got.Relation)
	}
}

func TestDefaultLinkKindForSibling(t *testing.T) {
	dep := SiblingRelationResult{Relation: SiblingDependent}
	if got := DefaultLinkKindForSibling(dep); got != LinkUpstream {
		t.Fatalf("got %q", got)
	}
	ind := SiblingRelationResult{Relation: SiblingIndependent}
	if got := DefaultLinkKindForSibling(ind); got != LinkFresh {
		t.Fatalf("got %q", got)
	}
}

func TestValidateContextScope(t *testing.T) {
	scope := NewContextScope("s1", "wi1", plan.PersistSession)
	if err := ValidateContextScope(scope); err != nil {
		t.Fatal(err)
	}
	scope.SidechainKey = "bad"
	if err := ValidateContextScope(scope); err == nil {
		t.Fatal("expected sidechain mismatch error")
	}
}

func TestContextScopeSidechainKey(t *testing.T) {
	if got := ContextScopeSidechainKey("abc"); got != "wi_abc" {
		t.Fatalf("got %q", got)
	}
}
