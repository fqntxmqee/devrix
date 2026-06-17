package doctor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDoctor_RunAllChecks — 默认 doctor 跑完 7 项 check。
func TestDoctor_RunAllChecks(t *testing.T) {
	d := NewDefaultDoctor(t.TempDir(), "", "", nil)
	checks := d.Run(context.Background())
	if len(checks) != 7 {
		t.Errorf("expected 7 default checks, got %d", len(checks))
	}
	for _, c := range checks {
		if c.Name == "" {
			t.Errorf("check missing name: %+v", c)
		}
		if c.Status == "" {
			t.Errorf("check %q missing status", c.Name)
		}
	}
}

// TestDoctor_InstallPaths_MissingBinary — devrix binary 缺失 → warn。
func TestDoctor_InstallPaths_MissingBinary(t *testing.T) {
	d := NewDefaultDoctor("", "definitely-not-a-real-binary-xyzzy", "", nil)
	c := d.checkInstallPaths(context.Background())
	if c.Status != StatusWarn {
		t.Errorf("expected warn for missing binary, got %s: %s", c.Status, c.Detail)
	}
	if !strings.Contains(c.Detail, "definitely-not-a-real-binary-xyzzy") {
		t.Errorf("detail should mention missing tool, got %q", c.Detail)
	}
}

// TestDoctor_LSPServers_AllMissing — LSP 全找不到 → fail。
func TestDoctor_LSPServers_AllMissing(t *testing.T) {
	d := NewDefaultDoctor("", "", "", []LSPServer{
		{Name: "gopls", Command: "gopls-fake-12345"},
		{Name: "tsserver", Command: "tsserver-fake-67890"},
	})
	c := d.checkLSPServers(context.Background())
	if c.Status != StatusFail {
		t.Errorf("expected fail when all lsp servers missing, got %s: %s", c.Status, c.Detail)
	}
}

// TestDoctor_LSPServers_NoneConfigured — 零配置 → warn。
func TestDoctor_LSPServers_NoneConfigured(t *testing.T) {
	d := NewDefaultDoctor("", "", "", nil)
	c := d.checkLSPServers(context.Background())
	if c.Status != StatusWarn {
		t.Errorf("expected warn for zero lsp config, got %s: %s", c.Status, c.Detail)
	}
}

// TestDoctor_LSPServers_PassWhenReachable — 指向已存在的命令 → pass。
func TestDoctor_LSPServers_PassWhenReachable(t *testing.T) {
	d := NewDefaultDoctor("", "", "", []LSPServer{
		{Name: "sh", Command: "sh"}, // sh 一定存在
	})
	c := d.checkLSPServers(context.Background())
	if c.Status != StatusPass {
		t.Errorf("expected pass for reachable sh, got %s: %s", c.Status, c.Detail)
	}
}

// TestDoctor_WorkdirWritable_Pass — workdir 可写 → pass。
func TestDoctor_WorkdirWritable_Pass(t *testing.T) {
	d := NewDefaultDoctor(t.TempDir(), "", "", nil)
	c := d.checkWorkdirWritable(context.Background())
	if c.Status != StatusPass {
		t.Errorf("expected pass, got %s: %s", c.Status, c.Detail)
	}
}

// TestDoctor_WorkdirWritable_EmptyFail — 空 workdir → fail。
func TestDoctor_WorkdirWritable_EmptyFail(t *testing.T) {
	d := NewDefaultDoctor("", "", "", nil)
	c := d.checkWorkdirWritable(context.Background())
	if c.Status != StatusFail {
		t.Errorf("expected fail for empty WorkDir, got %s", c.Status)
	}
}

// TestDoctor_ConfigYAML_PassWhenExists — 写一个 config.yaml → pass。
func TestDoctor_ConfigYAML_PassWhenExists(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "config.yaml"), "x: 1\n")
	d := NewDefaultDoctor(dir, "", "", nil)
	c := d.checkConfigYAML(context.Background())
	if c.Status != StatusPass {
		t.Errorf("expected pass, got %s: %s", c.Status, c.Detail)
	}
}

// TestDoctor_ConfigYAML_WarnWhenMissing — 无 yaml → warn。
func TestDoctor_ConfigYAML_WarnWhenMissing(t *testing.T) {
	d := NewDefaultDoctor(t.TempDir(), "", "", nil)
	c := d.checkConfigYAML(context.Background())
	if c.Status != StatusWarn {
		t.Errorf("expected warn, got %s", c.Status)
	}
}

// TestDoctor_TranscriptDir_Pass — transcript dir 不可写则 fail,否则 pass。
func TestDoctor_TranscriptDir_Pass(t *testing.T) {
	d := NewDefaultDoctor("", "", t.TempDir(), nil)
	c := d.checkTranscriptDir(context.Background())
	if c.Status != StatusPass {
		t.Errorf("expected pass, got %s: %s", c.Status, c.Detail)
	}
}

// TestDoctor_TranscriptDir_Empty — 未配置 → warn。
func TestDoctor_TranscriptDir_Empty(t *testing.T) {
	d := NewDefaultDoctor("", "", "", nil)
	c := d.checkTranscriptDir(context.Background())
	if c.Status != StatusWarn {
		t.Errorf("expected warn, got %s", c.Status)
	}
}

// TestFormatJSON_Valid — JSON 可解析。
func TestFormatJSON_Valid(t *testing.T) {
	checks := []Check{{Name: "x", Status: StatusPass, Detail: "ok"}}
	data, err := FormatJSON(checks)
	if err != nil {
		t.Fatal(err)
	}
	var out []Check
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out) != 1 || out[0].Name != "x" {
		t.Errorf("round-trip failed: %+v", out)
	}
}

// TestFormatTable_IncludesAll — table 渲染含所有 check。
func TestFormatTable_IncludesAll(t *testing.T) {
	checks := []Check{
		{Name: "a", Status: StatusPass, Detail: "ok"},
		{Name: "b", Status: StatusFail, Detail: "broken"},
	}
	out := FormatTable(checks)
	for _, want := range []string{"Doctor Report", "a", "pass", "b", "fail"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q in:\n%s", want, out)
		}
	}
}

// TestSummary_AllPass → pass。
func TestSummary_AllPass(t *testing.T) {
	got := Summary([]Check{
		{Status: StatusPass},
		{Status: StatusPass},
	})
	if got != StatusPass {
		t.Errorf("expected pass, got %s", got)
	}
}

// TestSummary_AnyWarn → warn。
func TestSummary_AnyWarn(t *testing.T) {
	got := Summary([]Check{
		{Status: StatusPass},
		{Status: StatusWarn},
	})
	if got != StatusWarn {
		t.Errorf("expected warn, got %s", got)
	}
}

// TestSummary_AnyFail → fail 优先。
func TestSummary_AnyFail(t *testing.T) {
	got := Summary([]Check{
		{Status: StatusPass},
		{Status: StatusWarn},
		{Status: StatusFail},
	})
	if got != StatusFail {
		t.Errorf("expected fail (highest priority), got %s", got)
	}
}

// TestDoctor_ExtraChecks — 自定义 check 也会被运行。
func TestDoctor_ExtraChecks(t *testing.T) {
	d := NewDefaultDoctor(t.TempDir(), "", "", nil)
	d.ExtraChecks = []CheckFunc{
		func(_ context.Context) Check { return Check{Name: "extra1", Status: StatusPass, Detail: "yes"} },
		func(_ context.Context) Check { return Check{Name: "extra2", Status: StatusFail, Detail: "no"} },
	}
	got := d.Run(context.Background())
	if len(got) != 9 { // 7 默认 + 2 extra
		t.Errorf("expected 9 checks, got %d", len(got))
	}
}

// TestDoctor_InterfaceConformance — 满足 Doctor 接口。
func TestDoctor_InterfaceConformance(t *testing.T) {
	var _ Doctor = NewDefaultDoctor("", "", "", nil)
}

func mustWrite(t *testing.T, p, content string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
