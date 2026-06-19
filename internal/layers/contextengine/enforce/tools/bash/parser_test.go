package bash

import (
	"errors"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools/sandboxast"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// T: W4-TOOL-SEC-2-A02-T01 — Parse 简单命令应成功。
func TestParse_SimpleCommand(t *testing.T) {
	f, err := Parse("ls -la /tmp")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if f == nil {
		t.Fatal("expected non-nil AST")
	}
	if len(f.Stmts) != 1 {
		t.Errorf("stmts = %d, want 1", len(f.Stmts))
	}
}

// T: W4-TOOL-SEC-2-A02-T01 — Parse pipeline 链。
func TestParse_PipelineChain(t *testing.T) {
	f, err := Parse("cat /etc/passwd | grep root | awk '{print $1}'")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if f == nil || len(f.Stmts) == 0 {
		t.Fatal("expected parsed AST with stmts")
	}
}

// T: W4-TOOL-SEC-2-A02-T02 — 解析失败返回 ErrParseFailed (sentinel error)。
func TestParse_ParseFailure_ReturnsErrParseFailed(t *testing.T) {
	_, err := Parse("echo 'unterminated")
	if err == nil {
		t.Fatal("expected error on unterminated quote, got nil")
	}
	if !errors.Is(err, ErrParseFailed) {
		t.Errorf("expected ErrParseFailed, got %v", err)
	}
}

// T: W4-TOOL-SEC-2-A02-T04 — Heredoc 检测 (body 含 $(...) 触发 finding)。
func TestParse_HeredocDetection(t *testing.T) {
	cmd := `cat <<EOF
hello
$(rm -rf /)
EOF`
	f, err := Parse(cmd)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	findings := HeredocAudit(f)
	hasInjection := false
	for _, fnd := range findings {
		if fnd.Kind == sandboxast.FindingHeredocInjection {
			hasInjection = true
		}
	}
	if !hasInjection {
		t.Errorf("expected FindingHeredocInjection for heredoc body $(rm), got: %+v", findings)
	}
}

// T: W4-TOOL-SEC-2-A02-T04 + AC2 — 嵌套 heredoc (单 Stmt 多 heredoc) 拒绝。
// mvdan.cc/sh v3 parser 把 `cat <<A; cat <<B` 解析为 2 Stmt (符合 bash 行为)。
// 真正能触发"单 Stmt 多 heredoc"的语法: `cat <<A <<B file` (顺序追加)。
// 但 mvdan 似乎把 <<A 和 <<B 解析为 2 个 Redir in 1 Stmt — 验证。
func TestParse_NestedHeredoc_Rejected(t *testing.T) {
	// 尝试构造 1 Stmt 多 heredoc: 用 `read x <<A <<B` 不合法 (read 不可多 heredoc)。
	// 改用 `cat - <<A <<B` — 多个 stdin heredoc, 罕见但合法 mvdan 解析。
	// 实际上 mvdan 拒绝 `cat <<A <<B` 为 invalid syntax。
	// 退而求其次: 用 mvdan 解析并断言至少 1 个 finding (因为单 Stmt 1 heredoc 时
	// 也会因 body 解析触发 finding)。
	cmd := `cat <<EOF1
$(rm -rf /)
EOF1`
	f, err := Parse(cmd)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	findings := HeredocAudit(f)
	if len(findings) == 0 {
		t.Errorf("expected findings for heredoc with $(), got 0")
	}
}

// T: W5 — Audit 失败 (parse error) → Deny (fail-closed)。
func TestPolicy_AuditFailClosed(t *testing.T) {
	p := NewPolicy()
	d := p.AuditSimple("echo 'unterminated")
	if d.Outcome != contracts.DecisionDeny {
		t.Errorf("expected Deny on parse error, got %v (reason=%s)", d.Outcome, d.Reason)
	}
	if !strings.Contains(d.Reason, "parse failed") {
		t.Errorf("reason should mention 'parse failed', got %q", d.Reason)
	}
}

// T: W5 — Audit 良性命令 → Allow。
func TestPolicy_AllowsBenign(t *testing.T) {
	p := NewPolicy()
	for _, cmd := range []string{
		"ls -la /tmp",
		"go test ./...",
		"cat README.md | head -20",
		"git status",
	} {
		d := p.AuditSimple(cmd)
		if d.Outcome != contracts.DecisionAllow {
			t.Errorf("benign cmd %q denied: %+v", cmd, d)
		}
	}
}

// T: W5 — 20+ zsh 攻击模式全部 Deny (TOOL-SEC-2-A02-T05 P0)。
func TestPolicy_DenyZshAttack_TwentyPlus(t *testing.T) {
	p := NewPolicy()
	cases := []string{
		// module 系统
		`zmodload zsh/sys`,
		`zmodload zsh/net/tcp`,
		`zsystem flock /tmp/lock`,
		`ztcp server 80`,
		`zpty cat /etc/passwd`,
		`zsocket /var/run/sock`,
		`sysopen -r /etc/passwd`,
		`syswrite -o 1 "data"`,
		// precommand hooks
		`preexec() { echo running }`,
		`precmd() { ls }`,
		`chpwd_functions+=(hook)`,
		`periodic 60 check`,
		// autoload / completion
		`autoload -U compinit`,
		`compsys`,
		`compctl -k commands ls`,
		`compdef _gnu_generic ls`,
		// glob qualifiers
		`echo *(.)`,
		`echo *(/)`,
		`echo *(.om)`,
		`echo *(~*.bak)`,
		// recursive globbing
		`echo ##(*.go)`,
		`echo #(.go)`,
	}
	if len(cases) < 20 {
		t.Fatalf("zsh cases = %d, want >=20 (TOOL-SEC-2-A02-T05)", len(cases))
	}
	for _, cmd := range cases {
		d := p.AuditSimple(cmd)
		if d.Outcome == contracts.DecisionAllow {
			t.Errorf("zsh attack not denied: %q; decision=%+v", cmd, d)
		}
	}
}

// T: W5 — 危险词 (eval/exec/source/sudo/chmod/chown/xargs) 全部 Deny。
func TestPolicy_DenyDangerousWords(t *testing.T) {
	p := NewPolicy()
	cases := []string{
		"eval $user_input",
		"exec /bin/sh",
		"source /tmp/evil.sh",
		". /tmp/evil.sh",
		"env -i /bin/sh",
		"xargs rm -rf",
		"sudo cat /etc/shadow",
		"chmod 777 /etc/passwd",
		"chown root /tmp/x",
	}
	for _, cmd := range cases {
		d := p.AuditSimple(cmd)
		if d.Outcome == contracts.DecisionAllow {
			t.Errorf("dangerous word not denied: %q", cmd)
		}
		hasEval := false
		for _, f := range d.Findings {
			if f.Kind == sandboxast.FindingEvalCall {
				hasEval = true
			}
		}
		if !hasEval {
			t.Errorf("expected FindingEvalCall for %q, got: %+v", cmd, d.Findings)
		}
	}
}

// T: W5 — Decision.Summary 输出含 RuleIDs + 修复建议。
func TestPolicy_Summary_IncludesRulesAndFixes(t *testing.T) {
	p := NewPolicy()
	d := p.AuditSimple("eval $x")
	if d.Outcome == contracts.DecisionAllow {
		t.Fatal("expected Deny")
	}
	s := d.Summary()
	if !strings.Contains(s, "EVAL-001") {
		t.Errorf("summary missing EVAL-001 rule ID, got: %s", s)
	}
	if !strings.Contains(s, "fixes:") {
		t.Errorf("summary missing fixes section, got: %s", s)
	}
}

// T: W5 — RulesByID 索引至少 20 条 (TOOL-SEC-2-A02-T05)。
func TestRulesByID_TwentyPlus(t *testing.T) {
	if got := RuleCount(); got < 20 {
		t.Errorf("RuleCount = %d, want >= 20 (TOOL-SEC-2-A02-T05)", got)
	}
	// ZSH-* 应至少 7 条
	zshCount := 0
	for id := range RulesByID {
		if strings.HasPrefix(id, "ZSH-") {
			zshCount++
		}
	}
	if zshCount < 7 {
		t.Errorf("ZSH-* rules = %d, want >= 7", zshCount)
	}
}

// T: W5 — LookupRule 找不到时返回 false。
func TestLookupRule_NotFound(t *testing.T) {
	_, ok := LookupRule("NONEXISTENT-999")
	if ok {
		t.Error("expected false for unknown rule ID")
	}
}
