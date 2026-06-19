package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// T: W15 — ci-lint-invariant 找到所有 5 个 _invariant.go 文件。
func TestScan_FindsAllInvariantFiles(t *testing.T) {
	roots := []string{
		"./internal/layers/contextengine/enforce/tools/surface",
		"./internal/layers/multiagent/provision/freefork",
		"./internal/layers/observability/diagnose/tracker",
		"./internal/layers/evolution/verify",
	}
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	// 切到 devrix 根 (test 在 tools/ci-lint-invariant/ 下, root 在 ..)
	if err := os.Chdir(filepath.Join(wd, "..", "..")); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	rep, err := scan(roots, false)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if rep.FilesScanned < 5 {
		t.Errorf("FilesScanned = %d, want >= 5 (LSP + bash + tracker + freefork + verify)", rep.FilesScanned)
	}
	if got := len(rep.Invariants); got < 16 {
		t.Errorf("total invariants = %d, want >= 16 (4+4+4+4+4 across 5 surfaces)", got)
	}
	if rep.ErrorCount() > 0 {
		t.Errorf("unexpected errors: %v", rep.Errors)
	}
}

// T: W15 — 解析失败 (缺 =>) 时报告 error。
func TestParse_MalformedTag_ReportsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "_invariant.go")
	bad := `package x
type Bad struct {
	F string ` + "`invariant:\"malformed_no_arrow\"`" + `
}
`
	if err := os.WriteFile(path, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	_, errs := parseInvariantFile(path)
	if len(errs) == 0 {
		t.Fatal("expected error for missing '=>' operator")
	}
	if !strings.Contains(errs[0], "missing") {
		t.Errorf("error should mention 'missing', got %q", errs[0])
	}
}

// T: W15 — 空 pre/post 报告 error。
func TestParse_EmptyPreOrPost_ReportsError(t *testing.T) {
	cases := []struct {
		name string
		tag  string
	}{
		{"empty pre", "`invariant:\" => something\"`"},
		{"empty post", "`invariant:\"something => \"`"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "_invariant.go")
			src := `package x
type Bad struct {
	F string ` + c.tag + `
}
`
			if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			_, errs := parseInvariantFile(path)
			if len(errs) == 0 {
				t.Errorf("expected error for %s", c.name)
			}
		})
	}
}

// T: W15 — 合法 invariant 解析成功。
func TestParse_ValidInvariant_NoError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "_invariant.go")
	src := `package x
type Good struct {
	F string ` + "`invariant:\"a => b\"`" + `
}
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, errs := parseInvariantFile(path)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if entries[0].Pre != "a" || entries[0].Post != "b" {
		t.Errorf("pre/post = (%q,%q), want (a,b)", entries[0].Pre, entries[0].Post)
	}
}

// T: W15 — 跨 surface 同名 invariant 不同 post 报告 conflict。
func TestDetectConflicts_DivergentPosts(t *testing.T) {
	entries := []InvariantEntry{
		{Source: "S1", Field: "Shared", Pre: "p", Post: "x"},
		{Source: "S2", Field: "Shared", Pre: "p", Post: "y"}, // divergent
	}
	groups := detectConflicts(entries)
	if len(groups) != 1 {
		t.Fatalf("conflict groups = %d, want 1", len(groups))
	}
	if len(groups[0].Posts) != 2 {
		t.Errorf("posts = %d, want 2", len(groups[0].Posts))
	}
}

// T: W15 — 跨 surface 同名 invariant 相同 post 不报 conflict。
func TestDetectConflicts_SamePost_NoConflict(t *testing.T) {
	entries := []InvariantEntry{
		{Source: "S1", Field: "Shared", Pre: "p", Post: "x"},
		{Source: "S2", Field: "Shared", Pre: "p", Post: "x"},
	}
	groups := detectConflicts(entries)
	if len(groups) != 0 {
		t.Errorf("expected 0 conflict groups, got %d", len(groups))
	}
}

// T: W15 — 缺 _invariant.go 时报告 warning。
func TestScan_MissingInvariantFile_Warning(t *testing.T) {
	dir := t.TempDir()
	rep, err := scan([]string{dir}, false)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if rep.WarningCount() == 0 {
		t.Error("expected warning for empty root dir")
	}
}

// T: W15 — reflectTag 解析多个 tag key。
func TestReflectTag_ParsesMultipleKeys(t *testing.T) {
	tag := `json:"field" yaml:"f" invariant:"a => b"`
	if got := reflectTag(tag, "json"); got != "field" {
		t.Errorf("json = %q, want field", got)
	}
	if got := reflectTag(tag, "yaml"); got != "f" {
		t.Errorf("yaml = %q, want f", got)
	}
	if got := reflectTag(tag, "invariant"); got != "a => b" {
		t.Errorf("invariant = %q, want 'a => b'", got)
	}
	if got := reflectTag(tag, "missing"); got != "" {
		t.Errorf("missing = %q, want empty", got)
	}
}
