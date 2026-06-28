package workmodel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatMaterializeContextShow_PartitionAndDownlink(t *testing.T) {
	tm := NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "g")
	children, _ := tm.DecomposeChildren("s1", goal.ID, []ChildSpec{{
		Title: "c", Directive: "d", ExpectedReturn: "patch", ScopeIn: []string{"a.go"},
	}})
	if len(children) == 0 {
		t.Fatal("no child")
	}
	child := children[0]
	out := FormatMaterializeContextShow("s1", tm, child)
	if !strings.Contains(out, "partition: wi:s1:") {
		t.Fatalf("missing partition: %q", out)
	}
	if !strings.Contains(out, "downlink_scope_in: a.go") {
		t.Fatalf("missing downlink: %q", out)
	}
}

func TestMaterializeContextResolveHint_Upstream(t *testing.T) {
	tm := NewTaskManager()
	blocker, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{Kind: WorkKindImplement, Title: "a", Directive: "a"})
	dep, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{Kind: WorkKindImplement, Title: "b", Directive: "b"})
	_ = tm.Tree().AddDependency("s1", dep.ID, blocker.ID)
	dep, _ = tm.GetWorkItem("s1", dep.ID)
	hint := MaterializeContextResolveHint("s1", tm, dep)
	if !strings.Contains(hint, "upstream inject") {
		t.Fatalf("hint = %q", hint)
	}
}

func TestPrivateChainMessageCount(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s1", "wi", "wi1.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"role":"user","content":"a"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if n := PrivateChainMessageCount("s1", "wi1", dir); n != 1 {
		t.Fatalf("count = %d, want 1", n)
	}
}
