package surface_test

import (
	"strings"
	"testing"

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
