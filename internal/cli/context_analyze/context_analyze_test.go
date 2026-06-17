package contextanalyze_test

// W10 — D2-S6-A03 (alias A5) /context analyze CLI 子命令 单元测试。
//
// AC2:
//   - messages.jsonl 输入 → 5 类 category 计数正确
//   - 缺省参数 → 错误

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contextanalyze "github.com/devrix/devrix/internal/cli/context_analyze"
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

func writeJSONL(t *testing.T, lines []string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "messages.jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write jsonl: %v", err)
	}
	return p
}

func TestContextAnalyze_CategorizesMessages(t *testing.T) {
	// 5 行: system / user / assistant / tool / 含 thinking
	lines := []string{
		`{"role":"system","content":"you are a helpful assistant"}`,
		`{"role":"user","content":"hi"}`,
		`{"role":"assistant","content":"hello there"}`,
		`{"role":"tool","content":"{\"k\":\"v\"}"}`,
		`{"role":"assistant","content":"<thinking>reasoning</thinking> answer"}`,
	}
	p := writeJSONL(t, lines)
	out, err := captureStdout(t, func() error {
		return contextanalyze.Run([]string{"--messages-file=" + p})
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, cat := range []string{"system", "messages", "tools", "thinking", "reminders", "total"} {
		if !strings.Contains(out, cat) {
			t.Errorf("table missing category %q\n--- output ---\n%s", cat, out)
		}
	}
}

func TestContextAnalyze_JSONFormat(t *testing.T) {
	lines := []string{
		`{"role":"system","content":"sys"}`,
		`{"role":"user","content":"hi"}`,
	}
	p := writeJSONL(t, lines)
	out, err := captureStdout(t, func() error {
		return contextanalyze.Run([]string{"--json", "--messages-file=" + p})
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var bd struct {
		System    int `json:"system"`
		Tools     int `json:"tools"`
		Messages  int `json:"messages"`
		Thinking  int `json:"thinking"`
		Reminders int `json:"reminders"`
		Total     int `json:"total"`
	}
	if err := json.Unmarshal([]byte(out), &bd); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if bd.System <= 0 {
		t.Errorf("expected system > 0, got %d", bd.System)
	}
	if bd.Messages <= 0 {
		t.Errorf("expected messages > 0, got %d", bd.Messages)
	}
	if bd.Total != bd.System+bd.Tools+bd.Messages+bd.Thinking+bd.Reminders {
		t.Errorf("total %d != sum of categories", bd.Total)
	}
}

func TestContextAnalyze_MissingArgsReturnsError(t *testing.T) {
	_, err := captureStdout(t, func() error {
		return contextanalyze.Run([]string{})
	})
	if err == nil {
		t.Errorf("expected error for missing args")
	}
	if !strings.Contains(err.Error(), "is required") {
		t.Errorf("expected 'is required' error, got %q", err.Error())
	}
}

func TestContextAnalyze_EmptyFileReturnsError(t *testing.T) {
	// 空 messages 文件 → readMessagesJSONL 返回空列表, 触发 "no messages" 错误。
	dir := t.TempDir()
	p := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(p, []byte(""), 0o644); err != nil {
		t.Fatalf("write empty: %v", err)
	}
	_, err := captureStdout(t, func() error {
		return contextanalyze.Run([]string{"--messages-file=" + p})
	})
	if err == nil {
		t.Errorf("expected error for empty file")
	}
	if !strings.Contains(err.Error(), "no messages") {
		t.Errorf("expected 'no messages' error, got %q", err.Error())
	}
}

func TestContextAnalyze_InvalidFileReturnsError(t *testing.T) {
	_, err := captureStdout(t, func() error {
		return contextanalyze.Run([]string{"--messages-file=/nonexistent/path/xyz.jsonl"})
	})
	if err == nil {
		t.Errorf("expected error for invalid file path")
	}
}
