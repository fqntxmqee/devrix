package verify

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTasksFile 在 t.TempDir() 写一个示例 tasks.md。
func writeTasksFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "tasks.md")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const sampleTasks = `# Tasks: Example

## W1 — Phase One

| ID | Description | File | Status |
|----|-------------|------|--------|
| W1.1 | Add field A | internal/foo/a.go | done |
| W1.2 | Add field B | internal/foo/b.go | pending |
| W1.3 | Update spec |  | done |

## W2 — Tests

| ID | Description | File | Status |
|----|-------------|------|--------|
| W2.1 | Cover A test | internal/foo/a_test.go | done |
`

// TestLoadPlan_ParsesRows — 标准 4 列表格解析。
func TestLoadPlan_ParsesRows(t *testing.T) {
	p := writeTasksFile(t, sampleTasks)
	items, err := NewFileVerifier().LoadPlan(p)
	if err != nil {
		t.Fatalf("LoadPlan: %v", err)
	}
	if len(items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(items))
	}
	if items[0].ID != "W1.1" || !items[0].Done {
		t.Errorf("W1.1 wrong: %+v", items[0])
	}
	if items[1].ID != "W1.2" || items[1].Done {
		t.Errorf("W1.2 wrong: %+v", items[1])
	}
	if items[2].ID != "W1.3" || !items[2].Done {
		t.Errorf("W1.3 wrong: %+v", items[2])
	}
	if items[3].ID != "W2.1" || !items[3].Done {
		t.Errorf("W2.1 wrong: %+v", items[3])
	}
}

// TestLoadPlan_AutoTestEvidence — _test.go 文件自动加 Test evidence。
func TestLoadPlan_AutoTestEvidence(t *testing.T) {
	p := writeTasksFile(t, sampleTasks)
	items, _ := NewFileVerifier().LoadPlan(p)
	w2 := items[3]
	hasTest := false
	for _, ev := range w2.Evidence {
		if ev.Kind == EvidenceTest {
			hasTest = true
		}
	}
	if !hasTest {
		t.Errorf("W2.1 should have Test evidence, got: %+v", w2.Evidence)
	}
}

// TestLoadPlan_IgnoresHeaderRows — header + separator 行跳过。
func TestLoadPlan_IgnoresHeaderRows(t *testing.T) {
	p := writeTasksFile(t, sampleTasks)
	items, _ := NewFileVerifier().LoadPlan(p)
	for _, it := range items {
		if it.ID == "ID" || it.ID == "Task" {
			t.Errorf("header row leaked: %+v", it)
		}
	}
}

// TestVerify_AllPass — 所有 done item 文件存在时,全 verify。
func TestVerify_AllPass(t *testing.T) {
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "internal/foo/a.go"), "package foo")
	mustWrite(t, filepath.Join(repo, "internal/foo/b.go"), "package foo")
	mustWrite(t, filepath.Join(repo, "internal/foo/a_test.go"), "package foo\n\nfunc TestA(t *testing.T) {}\n")
	tasksPath := writeTasksFile(t, sampleTasks)
	// 把 tasks.md 中 path 改成相对 repo 根
	items, _ := NewFileVerifier().LoadPlan(tasksPath)
	report, err := NewFileVerifier().Verify(context.Background(), items, repo)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	// 3 done: W1.1 (file), W1.3 (no file = pass), W2.1 (file+test)
	if report.Verified != 3 {
		t.Errorf("expected 3 verified, got %d; unverified=%+v", report.Verified, report.Unverified)
	}
	if len(report.Unverified) != 0 {
		t.Errorf("expected 0 unverified, got %+v", report.Unverified)
	}
	if report.Skipped != 1 { // W1.2 pending
		t.Errorf("expected 1 skipped, got %d", report.Skipped)
	}
}

// TestVerify_MissingFile — Done=true 但文件不存在 → unverified。
func TestVerify_MissingFile(t *testing.T) {
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "internal/foo/a.go"), "package foo")
	tasksPath := writeTasksFile(t, sampleTasks)
	items, _ := NewFileVerifier().LoadPlan(tasksPath)
	report, _ := NewFileVerifier().Verify(context.Background(), items, repo)
	if len(report.Unverified) == 0 {
		t.Fatal("expected at least 1 unverified")
	}
	gotReason := report.Unverified[0].Reason
	if !strings.Contains(gotReason, "file not found") {
		t.Errorf("expected 'file not found' reason, got %q", gotReason)
	}
}

// TestVerify_TestFileMissingFunc — _test.go 但没有 func TestXxx → fail。
func TestVerify_TestFileMissingFunc(t *testing.T) {
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "internal/foo/a.go"), "package foo")
	mustWrite(t, filepath.Join(repo, "internal/foo/a_test.go"), "package foo // no test func")
	tasksPath := writeTasksFile(t, sampleTasks)
	items, _ := NewFileVerifier().LoadPlan(tasksPath)
	report, _ := NewFileVerifier().Verify(context.Background(), items, repo)
	if len(report.Unverified) == 0 {
		t.Fatal("expected W2.1 to be unverified (no func Test)")
	}
	if !strings.Contains(report.Unverified[0].Reason, "no func Test") {
		t.Errorf("expected test-missing reason, got %q", report.Unverified[0].Reason)
	}
}

// TestVerify_EmptyEvidencePasses — Done=true + 无 file 列 → 视为 spec-only 任务,pass。
func TestVerify_EmptyEvidencePasses(t *testing.T) {
	repo := t.TempDir()
	tasks := `# T
| ID | Desc | File | Status |
|----|------|------|--------|
| W1.1 | doc |  | done |
`
	p := writeTasksFile(t, tasks)
	items, _ := NewFileVerifier().LoadPlan(p)
	report, _ := NewFileVerifier().Verify(context.Background(), items, repo)
	if len(report.Unverified) != 0 {
		t.Errorf("spec-only item should pass, got %+v", report.Unverified)
	}
	if report.Verified != 1 {
		t.Errorf("expected 1 verified, got %d", report.Verified)
	}
}

// TestFormatJSON — 序列化为合法 JSON 且含 change_id。
func TestFormatJSON(t *testing.T) {
	r := Report{ChangeID: "demo", Total: 1, Verified: 1}
	data, err := FormatJSON(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"change_id": "demo"`) {
		t.Errorf("JSON missing change_id: %s", data)
	}
}

// TestVerify_ContextCanceled — 取消 ctx 后立即返回。
func TestVerify_ContextCanceled(t *testing.T) {
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "internal/foo/a.go"), "package foo")
	mustWrite(t, filepath.Join(repo, "internal/foo/a_test.go"), "package foo\nfunc TestA(t *testing.T) {}\n")
	p := writeTasksFile(t, sampleTasks)
	items, _ := NewFileVerifier().LoadPlan(p)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消
	report, _ := NewFileVerifier().Verify(ctx, items, repo)
	// 第一条 evidence 失败,后续不会再走 ctx.Err()。
	// 关键点:即便 ctx canceled,不能 panic,必须返回有效 report。
	if report.Total != len(items) {
		t.Errorf("expected total=%d, got %d", len(items), report.Total)
	}
}

func mustWrite(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
