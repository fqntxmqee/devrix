package toolrunner_test

// W6 — D6-S11-A02 (alias G4) verify_plan_execution LLM tool 单元测试。
//
// AC4:
//   - done items verified → JSON 含 verified 计数
//   - missing file → unverified 含 reason="file not found"
//   - _test.go without func TestXxx → unverified

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
)

// T: D6-S11-A02-T01
// 构造一个 tasks.md 含 3 个 done item：
//   - W1 引用存在的 .go 文件 → verified
//   - W2 引用存在的 _test.go 文件且含 func TestXxx → verified
//   - W3 引用不存在的文件 → unverified
func TestVerifyTool_DoneItemsVerified(t *testing.T) {
	dir := t.TempDir()
	// repo 根目录结构：<dir>/openspec/changes/test-change/tasks.md + <dir>/file*.go
	changeDir := filepath.Join(dir, "openspec", "changes", "test-change")
	if err := os.MkdirAll(changeDir, 0755); err != nil {
		t.Fatalf("mkdir changeDir: %v", err)
	}
	// W1 file
	mustWriteFile(t, dir, "file1.go", "package x\n")
	// W2 _test.go with func TestXxx
	mustWriteFile(t, dir, "file2_test.go", "package x\n\nfunc TestFoo(t *testing.T) {}\n")
	// W3 missing file
	tasksMD := strings.Join([]string{
		"| ID | Title | File | Status |",
		"|----|-------|------|--------|",
		"| W1.1 | item1 | file1.go | done |",
		"| W1.2 | item2 | file2_test.go | done |",
		"| W1.3 | item3 | missing.go | done |",
		"| W1.4 | item4 | file1.go | pending |",
	}, "\n")
	if err := os.WriteFile(filepath.Join(changeDir, "tasks.md"), []byte(tasksMD), 0644); err != nil {
		t.Fatalf("write tasks.md: %v", err)
	}

	reg := toolrunner.NewToolRegistry()
	if err := toolrunner.RegisterVerifyTool(reg); err != nil {
		t.Fatalf("RegisterVerifyTool: %v", err)
	}

	input, _ := json.Marshal(map[string]string{
		"change_id": "test-change",
		"repo_root": dir,
	})

	res, err := reg.Execute(context.Background(), toolrunner.ToolCall{
		Name:  "verify_plan_execution",
		Input: string(input),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Error != "" {
		t.Fatalf("tool error: %s", res.Error)
	}
	if !strings.Contains(res.Output, `"verified": 2`) {
		t.Errorf("expected verified=2, got: %s", res.Output)
	}
	if !strings.Contains(res.Output, `"unverified"`) {
		t.Errorf("expected unverified list, got: %s", res.Output)
	}
	if !strings.Contains(res.Output, "missing.go") {
		t.Errorf("expected missing.go in unverified, got: %s", res.Output)
	}
	if !strings.Contains(res.Output, `"skipped": 1`) {
		t.Errorf("expected skipped=1 (pending item), got: %s", res.Output)
	}
}

// T: D6-S11-A02-T02
// _test.go 文件存在但不含 func TestXxx → unverified
func TestVerifyTool_TestFileMissingFunc(t *testing.T) {
	dir := t.TempDir()
	changeDir := filepath.Join(dir, "openspec", "changes", "test-change")
	if err := os.MkdirAll(changeDir, 0755); err != nil {
		t.Fatalf("mkdir changeDir: %v", err)
	}
	mustWriteFile(t, dir, "noop_test.go", "package x\n// no test func here\n")
	tasksMD := strings.Join([]string{
		"| ID | Title | File | Status |",
		"|----|-------|------|--------|",
		"| W1.1 | item | noop_test.go | done |",
	}, "\n")
	if err := os.WriteFile(filepath.Join(changeDir, "tasks.md"), []byte(tasksMD), 0644); err != nil {
		t.Fatalf("write tasks.md: %v", err)
	}

	reg := toolrunner.NewToolRegistry()
	if err := toolrunner.RegisterVerifyTool(reg); err != nil {
		t.Fatalf("RegisterVerifyTool: %v", err)
	}

	input, _ := json.Marshal(map[string]string{
		"change_id": "test-change",
		"repo_root": dir,
	})
	res, _ := reg.Execute(context.Background(), toolrunner.ToolCall{
		Name:  "verify_plan_execution",
		Input: string(input),
	})
	if res.Error != "" {
		t.Fatalf("tool error: %s", res.Error)
	}
	if !strings.Contains(res.Output, `"verified": 0`) {
		t.Errorf("expected verified=0, got: %s", res.Output)
	}
	if !strings.Contains(res.Output, "noop_test.go") {
		t.Errorf("expected noop_test.go in unverified, got: %s", res.Output)
	}
}

// T: change_id 为空 → 工具返回 error，不 panic
func TestVerifyTool_ChangeIDRequired(t *testing.T) {
	reg := toolrunner.NewToolRegistry()
	if err := toolrunner.RegisterVerifyTool(reg); err != nil {
		t.Fatalf("RegisterVerifyTool: %v", err)
	}
	input, _ := json.Marshal(map[string]string{"change_id": ""})
	res, _ := reg.Execute(context.Background(), toolrunner.ToolCall{
		Name:  "verify_plan_execution",
		Input: string(input),
	})
	if res == nil || !strings.Contains(res.Error, "change_id is required") {
		t.Errorf("expected change_id required error, got %+v", res)
	}
}

func mustWriteFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
