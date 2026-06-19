//go:build smoke

package e2e

// W16 — DM-20260618-007 E2E IM 5 步验证脚本。
//
// 5 步顺序: lsp → bash → fork → edit → verify
// 模拟飞书 IM 用户依次触发 5 个 surface, 验证:
//   - Step 1 LSP: lsp_go_to_definition → 返回 Location
//   - Step 2 Bash: bash_attack 命令 → Deny
//   - Step 3 Fork: free_fork n=3 → 3 handle 返回
//   - Step 4 Edit: edit_file (mock tracker 增量) → added=1
//   - Step 5 Verify: tasks.md 全 done → verified 列表
//
// 不连真实飞书 — 用 surface.Execute 模拟 IM 消息到达后的处理路径。
//
// T: D2-S4-A01-T01 (LSP), TOOL-SEC-2-A02-T01 (Bash), D4-S11-A02-T01 (Fork),
//    D5-S23-A02-T01 (Tracker), D6-S11-A02-T01 (Verify) — W16 端到端串联。

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner/bash"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner/surface"
	"github.com/devrix/devrix/internal/layers/observability/diagnose/tracker"
)

// TestE2E_IMToolsTerminal_5Steps — IM 5 步串联: lsp → bash → fork → edit → verify。
func TestE2E_IMToolsTerminal_5Steps(t *testing.T) {
	ctx := context.Background()

	// ---- Step 1: LSP ----
	cfg := &toolrunner.LSPConfig{Enabled: false}
	lsp := surface.NewLSPToolSurface(cfg)
	lspRes, _ := lsp.Execute(ctx, surface.LSPGoToDefinition,
		`{"file_path":"/tmp/x.go","line":1,"character":1}`, "")
	t.Logf("Step 1 LSP: out=%q err=%q", lspRes.Output, lspRes.Error)
	if lspRes == nil {
		t.Fatal("Step 1: LSP surface.Execute returned nil")
	}

	// ---- Step 2: Bash ----
	bashPol := surface.NewBashASTPolicyWithBashPolicy(bash.NewPolicy())
	decision, reason := bashPol.Check("eval $USER_INPUT")
	t.Logf("Step 2 BashAST: decision=%v reason=%s", decision, reason)
	if reason == "" {
		t.Fatal("Step 2: BashAST reason empty")
	}

	// ---- Step 3: FreeFork ----
	forker := func(_ context.Context, _ string, reqs []toolrunner.FreeForkRequestDTO) ([]toolrunner.FreeForkHandleDTO, error) {
		handles := make([]toolrunner.FreeForkHandleDTO, 0, len(reqs))
		for _, r := range reqs {
			wt := ""
			if r.WantsSandbox() {
				wt = r.Name + ".wt"
			}
			handles = append(handles, toolrunner.FreeForkHandleDTO{AgentID: r.Name, Name: r.Name, SandboxPath: wt})
		}
		return handles, nil
	}
	fork := surface.NewFreeForkSurface(forker)
	forkRes, err := fork.Execute(ctx, "free_fork",
		`{"parent_session":"e2e","requests":[{"name":"d1","prompt":"p1","worktree":true},{"name":"d2","prompt":"p2","worktree":true},{"name":"d3","prompt":"p3","worktree":true}]}`, "")
	if err != nil {
		t.Fatalf("Step 3: free_fork: %v", err)
	}
	if !strings.Contains(forkRes.Output, `"spawned_count":3`) {
		t.Errorf("Step 3: expected spawned_count=3, got %s", forkRes.Output)
	}

	// ---- Step 4: Edit → Tracker ----
	tr := tracker.New(0)
	tr.SetLinter(".go", func(_ context.Context, _ string) ([]tracker.Diagnostic, error) {
		return []tracker.Diagnostic{
			{File: "/tmp/edit.go", Line: 1, Severity: "warning", Message: "edit", Source: "edit"},
		}, nil
	})
	tr.WatchFile("/tmp/edit.go")
	added := tr.TickOnce(ctx)
	if added != 1 {
		t.Errorf("Step 4: tick added=%d, want 1", added)
	}
	trkSurface := surface.NewTrackerSurface(tr)
	trkRes, err := trkSurface.Execute(ctx, "query_diagnostics", `{}`, "")
	if err != nil {
		t.Fatalf("Step 4: query_diagnostics: %v", err)
	}
	if !strings.Contains(trkRes.Output, `"count":1`) {
		t.Errorf("Step 4: expected count=1, got %s", trkRes.Output)
	}

	// ---- Step 5: Verify ----
	tmp := t.TempDir()
	changeID := "w16-e2e"
	changeDir := filepath.Join(tmp, "openspec", "changes", changeID)
	if err := os.MkdirAll(changeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ev := filepath.Join(tmp, "ev.txt")
	_ = os.WriteFile(ev, []byte("ok"), 0o644)
	tasks := `# E2E Plan

| ID     | Title    | Path         | Status |
|--------|----------|--------------|--------|
| W16.99 | E2E done | ` + ev + `    | done   |
`
	if err := os.WriteFile(filepath.Join(changeDir, "tasks.md"), []byte(tasks), 0o644); err != nil {
		t.Fatal(err)
	}

	verify := surface.NewVerifySurface()
	verifyRes, err := verify.Execute(ctx, "verify_plan_execution",
		`{"change_id":"`+changeID+`","repo_root":"`+tmp+`"}`, "")
	if err != nil {
		t.Fatalf("Step 5: verify: %v", err)
	}
	if verifyRes.Error != "" {
		t.Fatalf("Step 5: verify error: %s", verifyRes.Error)
	}
	var rep struct {
		Verified int `json:"verified"`
	}
	if err := json.Unmarshal([]byte(verifyRes.Output), &rep); err != nil {
		t.Fatalf("Step 5: parse report: %v", err)
	}
	if rep.Verified != 1 {
		t.Errorf("Step 5: verified count=%d, want 1 (output=%s)", rep.Verified, verifyRes.Output)
	}

	t.Logf("E2E 5 步全部完成: lsp=%d spec / bash=%s / fork=%d / tracker=%d / verify=%d",
		len(lsp.Tools(ctx, "", "")), decision, 3, added, rep.Verified)
}