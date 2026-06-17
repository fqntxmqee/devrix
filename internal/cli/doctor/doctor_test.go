package doctorcli_test

// W9 — D5-S23-A03 (alias A1) /doctor CLI 子命令 单元测试。
//
// AC1:
//   - doctor.Run 返回 7 项 check (install_paths, config_yaml_valid,
//     lsp_servers_reachable, workdir_writable, observability_ready,
//     tool_count, transcript_dir_ok)
//   - lsp_fake → lsp_servers_reachable 返回 fail

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	doctorcli "github.com/devrix/devrix/internal/cli/doctor"
)

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()
	fnErr := fn()
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String(), fnErr
}

func TestDoctor_RunReturnsSevenChecks(t *testing.T) {
	// 切到 repo root, 让 workdir_writable / config_yaml_valid 都有意义。
	wd, _ := os.Getwd()
	// 创建临时 workdir。
	tmp := t.TempDir()
	_ = wd

	out, err := captureStdout(t, func() error {
		return doctorcli.Run([]string{"--workdir=" + tmp})
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// table 模式 → 7 行 status 列加 summary 行。
	checks := []string{
		"install_paths",
		"config_yaml_valid",
		"lsp_servers_reachable",
		"workdir_writable",
		"observability_ready",
		"tool_count",
		"transcript_dir_ok",
	}
	for _, name := range checks {
		if !strings.Contains(out, name) {
			t.Errorf("table output missing check %q\n--- output ---\n%s", name, out)
		}
	}
}

func TestDoctor_JsonFormatReturnsSevenChecks(t *testing.T) {
	tmp := t.TempDir()
	out, err := captureStdout(t, func() error {
		return doctorcli.Run([]string{"--json", "--workdir=" + tmp})
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var checks []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal([]byte(out), &checks); err != nil {
		t.Fatalf("unmarshal: %v\n--- output ---\n%s", err, out)
	}
	if len(checks) != 7 {
		t.Errorf("got %d checks, want 7: %+v", len(checks), checks)
	}
}

func TestDoctor_LspFakeReturnsFail(t *testing.T) {
	tmp := t.TempDir()
	out, err := captureStdout(t, func() error {
		return doctorcli.Run([]string{"--json", "--workdir=" + tmp, "--lsp-fake"})
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var checks []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal([]byte(out), &checks); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var lspCheck *struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Detail string `json:"detail"`
	}
	for i := range checks {
		if checks[i].Name == "lsp_servers_reachable" {
			lspCheck = &checks[i]
			break
		}
	}
	if lspCheck == nil {
		t.Fatalf("lsp_servers_reachable check missing: %+v", checks)
	}
	if lspCheck.Status != "fail" {
		t.Errorf("lsp_servers_reachable status = %q, want fail (--lsp-fake)", lspCheck.Status)
	}
	if !strings.Contains(lspCheck.Detail, "fake-lsp-test") {
		t.Errorf("lsp_servers_reachable detail missing fake marker: %q", lspCheck.Detail)
	}
}
