package tools

// W2 — D2-S6-A02 (alias A7) ShortStack 包装 sandbox 拒绝错误单元测试。
//
// AC8 (前半):
//   - sandbox 拒绝错误栈 ≤ 5 帧
//   - runtime/testing/reflect 帧被过滤
//   - 原始错误信息 ("sandbox: ast block: ..." 或 "sandbox: dangerous command pattern detected: ...")
//     必须保留在错误字符串首部

import (
	"strings"
	"testing"
)

// T: D2-S6-A02-T01 (AC8 前半)
// 触发 AST 拒绝路径，断言栈 ≤ 5 帧、保留原错误前缀。
func TestSandbox_ASTBlock_ShortStack(t *testing.T) {
	p := DefaultCommandPolicy()
	p.ASTAnalyzer = stubAST{allow: false, reason: "FindingHeredocInjection"}

	err := p.Validate("cat <<EOF\n$(whoami)\nEOF")
	if err == nil {
		t.Fatalf("expected ast block error")
	}

	msg := err.Error()
	if !strings.HasPrefix(msg, "sandbox: ast block: FindingHeredocInjection") {
		t.Errorf("error prefix lost: %q", msg)
	}
	// 错误栈必须以换行符分隔追加在原 error 后面
	idx := strings.Index(msg, "\n")
	if idx < 0 {
		t.Fatalf("expected shortstack appended, got %q", msg)
	}
	stack := msg[idx+1:]
	lines := strings.Split(strings.TrimSpace(stack), "\n")
	if len(lines) > 5 {
		t.Errorf("stack frames = %d, want <= 5; lines=%v", len(lines), lines)
	}
	for _, line := range lines {
		if strings.Contains(line, "runtime.") ||
			strings.Contains(line, "testing.") ||
			strings.Contains(line, "reflect.") {
			t.Errorf("stack frame should be filtered: %q", line)
		}
	}
}

// T: D2-S6-A02-T01 (AC8 前半)
// 触发 DenyPatterns 路径，断言栈 ≤ 5 帧。
func TestSandbox_DangerousPattern_ShortStack(t *testing.T) {
	p := DefaultCommandPolicy()

	err := p.Validate("rm -rf /")
	if err == nil {
		t.Fatalf("expected dangerous pattern error")
	}
	msg := err.Error()
	if !strings.HasPrefix(msg, "sandbox: dangerous command pattern detected:") {
		t.Errorf("error prefix lost: %q", msg)
	}
	idx := strings.Index(msg, "\n")
	if idx < 0 {
		t.Fatalf("expected shortstack appended, got %q", msg)
	}
	stack := msg[idx+1:]
	lines := strings.Split(strings.TrimSpace(stack), "\n")
	if len(lines) > 5 {
		t.Errorf("stack frames = %d, want <= 5; lines=%v", len(lines), lines)
	}
}

// stubAST 实现 sandbox.ASTAnalyzer 接口。
type stubAST struct {
	allow  bool
	reason string
}

func (s stubAST) Analyze(cmd string) (bool, string) { return s.allow, s.reason }
