package surface_test

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner/bash"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner/surface"
)

// T: TOOL-SURFACE-1-A01-T27 — BashASTPolicy denies dangerous commands
// and allows safe ones. Each subtest is one command + expected
// (Decision, reason) pair.
func TestBashASTPolicy_Check(t *testing.T) {
	p := surface.NewBashASTPolicy()
	cases := []struct {
		name     string
		cmd      string
		want     string // expected Decision ("deny" / "allow" / "ask")
		wantHint string // expected reason substring (only checked for deny)
	}{
		// DANGEROUS — rm -rf /
		{"rm -rf /", "rm -rf /", "deny", "rm -rf / would destroy"},
		{"rm -fr /", "rm -fr /", "deny", "rm -rf / would destroy"},
		{"rm -rf /*", "rm -rf /*", "deny", "rm -rf / would destroy"},
		// DANGEROUS — dd
		{"dd if=/dev/zero of=/dev/sda", "dd if=/dev/zero of=/dev/sda", "deny", "dd can overwrite"},
		// DANGEROUS — mkfs
		{"mkfs /dev/sda1", "mkfs /dev/sda1", "deny", "mkfs formats"},
		{"mkfs.ext4 /dev/sda1", "mkfs.ext4 /dev/sda1", "deny", "mkfs formats"},
		// WARNING — sudo
		{"sudo apt-get update", "sudo apt-get update", "deny", "sudo elevates"},
		// WARNING — chmod 777 /
		{"chmod 777 /", "chmod 777 /", "deny", "chmod 777 / opens"},
		// SAFE
		{"ls -la", "ls -la", "allow", ""},
		{"echo hello", "echo hello", "allow", ""},
		{"cat /etc/hostname", "cat /etc/hostname", "allow", ""},
		{"rm file.txt", "rm file.txt", "allow", ""},             // no -rf, no /
		{"rm -rf /home/user", "rm -rf /home/user", "allow", ""}, // /home/user not root
		// Parse error
		{"unterminated quote", "echo 'unterminated", "ask", "bash parse error"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			decision, reason := p.Check(c.cmd)
			if string(decision) != c.want {
				t.Errorf("Check(%q) decision = %q, want %q (reason=%q)",
					c.cmd, decision, c.want, reason)
			}
			if c.wantHint != "" && !strings.Contains(reason, c.wantHint) {
				t.Errorf("Check(%q) reason = %q, want hint %q",
					c.cmd, reason, c.wantHint)
			}
		})
	}
}

// T: TOOL-SURFACE-1-A01-T27 — NewBashASTPolicy uses the default rule
// set, so a freshly constructed policy has 5 rules.
func TestNewBashASTPolicy_HasDefaults(t *testing.T) {
	p := surface.NewBashASTPolicy()
	if len(p.DenyList) != 5 {
		t.Errorf("default policy has %d rules, want 5", len(p.DenyList))
	}
}

// T: TOOL-SURFACE-1-A01-T27 — Multi-statement input: a safe command
// followed by a dangerous one still gets denied.
func TestBashASTPolicy_MultiStatement_DenyStillApplies(t *testing.T) {
	p := surface.NewBashASTPolicy()
	decision, reason := p.Check("ls -la; rm -rf /")
	if string(decision) != "deny" {
		t.Errorf("multi-statement deny: decision = %q, want deny (reason=%q)", decision, reason)
	}
}

// T: W5-TOOL-SEC-2-A02-T06 — BashASTPolicy v2 (with bash.Policy) 拒绝 zsh 攻击面。
// 验证集成 sandboxast 后能拦 zmodload / preexec / autoload 等。
func TestBashASTPolicy_V2_DenyZshAttack(t *testing.T) {
	bp := bash.NewPolicy()
	p := surface.NewBashASTPolicyWithBashPolicy(bp)
	for _, cmd := range []string{
		"zmodload zsh/sys",
		"preexec() { echo running }",
		"autoload -U compinit",
		"echo *(.)",
	} {
		decision, reason := p.Check(cmd)
		if string(decision) != "deny" {
			t.Errorf("zsh attack not denied: %q; decision=%q reason=%q", cmd, decision, reason)
		}
	}
}

// T: W5 — BashASTPolicy v2 拒绝危险词 (eval / exec / sudo / chmod / xargs)。
func TestBashASTPolicy_V2_DenyDangerousWords(t *testing.T) {
	bp := bash.NewPolicy()
	p := surface.NewBashASTPolicyWithBashPolicy(bp)
	for _, cmd := range []string{
		"eval $user_input",
		"exec /bin/sh",
		"sudo cat /etc/shadow",
		"chmod 777 /",
		"xargs rm -rf",
	} {
		decision, _ := p.Check(cmd)
		if string(decision) != "deny" {
			t.Errorf("dangerous word not denied: %q; decision=%q", cmd, decision)
		}
	}
}

// T: W5 — BashASTPolicy v2 fail-closed (parse error → Deny, 非 Ask)。
func TestBashASTPolicy_V2_FailClosedOnParseError(t *testing.T) {
	bp := bash.NewPolicy()
	p := surface.NewBashASTPolicyWithBashPolicy(bp)
	decision, reason := p.Check("echo 'unterminated")
	if string(decision) != "deny" {
		t.Errorf("v2 fail-closed: decision = %q, want deny (reason=%q)", decision, reason)
	}
	if !strings.Contains(reason, "parse") {
		t.Errorf("reason should mention parse, got %q", reason)
	}
}

// T: W5 — BashASTPolicy v2 良性命令仍 Allow (向后兼容 v1)。
func TestBashASTPolicy_V2_AllowsBenign(t *testing.T) {
	bp := bash.NewPolicy()
	p := surface.NewBashASTPolicyWithBashPolicy(bp)
	for _, cmd := range []string{
		"ls -la /tmp",
		"cat README.md | head -20",
		"go test ./...",
	} {
		decision, _ := p.Check(cmd)
		if string(decision) != "allow" {
			t.Errorf("benign cmd denied: %q; decision=%q", cmd, decision)
		}
	}
}

// T: W5 — BashASTPolicy v2 包含 5 deny rules (v1 + v2 行为都生效)。
func TestBashASTPolicy_V2_HasBothRuleSets(t *testing.T) {
	bp := bash.NewPolicy()
	p := surface.NewBashASTPolicyWithBashPolicy(bp)
	if len(p.DenyList) != 5 {
		t.Errorf("DenyList = %d, want 5 (v1 + v2 backward compat)", len(p.DenyList))
	}
}
