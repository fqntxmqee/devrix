package tools

// W4 — TOOL-SEC-2-A02 (alias G2) Bash AST 注入 bootstrap 单元测试。
//
// AC10:
//   - devrix.yaml 配置 sandbox.ast_enabled=true → bootstrap 注入 ASTAnalyzer
//   - sandbox.ast_enabled=false → ASTAnalyzer 为 nil（不启用 AST）
//   - bash heredoc body 内 $(whoami) 被拦
//   - bash zmodload zsh 攻击被拦

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/shared/config"
)

// T: TOOL-SEC-2-A02-T01
// 默认 ToolConfig (ASTEnabled 未显式设置) 应当返回 true，
// 保证 sandboxast 默认开启、heredoc/zsh attack 被拦。
func TestNewToolExecConfig_ASTEnabled_Default(t *testing.T) {
	cfg := config.DefaultToolConfig()
	cfg.Sandbox.ASTEnabled = nil // 默认 nil
	cfgE := newToolExecConfig(cfg)
	if cfgE.policy == nil || cfgE.policy.ASTAnalyzer == nil {
		t.Fatalf("expected ASTAnalyzer injected by default")
	}
}

// T: TOOL-SEC-2-A02-T01
// 显式设置 ast_enabled=false 时不注入 ASTAnalyzer。
func TestNewToolExecConfig_ASTDisabled(t *testing.T) {
	disabled := false
	cfg := config.DefaultToolConfig()
	cfg.Sandbox.ASTEnabled = &disabled
	cfgE := newToolExecConfig(cfg)
	if cfgE.policy == nil {
		t.Fatalf("policy should exist")
	}
	if cfgE.policy.ASTAnalyzer != nil {
		t.Errorf("expected ASTAnalyzer nil when ast_enabled=false")
	}
}

// T: TOOL-SEC-2-A02-T01 (heredoc 子能力)
// bash heredoc body 含 $(whoami) 应被 sandbox 拒绝。
func TestSandbox_ASTEnabled_BlockHeredocInjection(t *testing.T) {
	cfg := config.DefaultToolConfig()
	cfgE := newToolExecConfig(cfg)
	if cfgE.policy.ASTAnalyzer == nil {
		t.Fatalf("ASTAnalyzer must be injected for this test")
	}
	err := cfgE.policy.Validate("cat <<EOF\n$(whoami)\nEOF")
	if err == nil {
		t.Fatalf("expected heredoc injection blocked")
	}
	if !strings.Contains(err.Error(), "sandbox: ast block") {
		t.Errorf("expected 'sandbox: ast block' in err, got %q", err.Error())
	}
}

// T: TOOL-SEC-2-A02-T01 (zsh 子能力)
// bash zmodload zsh 攻击应被 sandbox 拒绝。
func TestSandbox_ASTEnabled_BlockZshAttack(t *testing.T) {
	cfg := config.DefaultToolConfig()
	cfgE := newToolExecConfig(cfg)
	if cfgE.policy.ASTAnalyzer == nil {
		t.Fatalf("ASTAnalyzer must be injected for this test")
	}
	err := cfgE.policy.Validate("zmodload zsh/sys")
	if err == nil {
		t.Fatalf("expected zsh attack blocked")
	}
	if !strings.Contains(err.Error(), "sandbox: ast block") {
		t.Errorf("expected 'sandbox: ast block' in err, got %q", err.Error())
	}
}

// T: TOOL-SEC-2-A02-T02
// 合法命令 (ls -la .) 不被 AST 拦，允许执行。
func TestSandbox_ASTEnabled_AllowLegitCommand(t *testing.T) {
	cfg := config.DefaultToolConfig()
	cfgE := newToolExecConfig(cfg)
	if cfgE.policy.ASTAnalyzer == nil {
		t.Fatalf("ASTAnalyzer must be injected for this test")
	}
	if err := cfgE.policy.Validate("ls -la ."); err != nil {
		t.Errorf("expected ls allow, got %v", err)
	}
}
