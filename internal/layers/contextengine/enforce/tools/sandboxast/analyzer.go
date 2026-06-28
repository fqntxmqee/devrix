// Package sandboxast — G2 Bash AST 安全分析器，对标 clawcode src/tools/BashTool/bashSecurity.ts。
//
// 基于 mvdan.cc/sh/v3 纯 Go shell parser，检测：
//   - heredoc body 内嵌危险命令 + 嵌套 heredoc (W4 AC2)
//   - process substitution <( ) / >( )
//   - command substitution $( ) / ` `
//   - zsh 攻击面 (zmodload, sysopen, =cmd, precommand modules 等 20+ 模式)
//   - 危险重定向 (>/dev/sda, >&跨进程)
//   - 嵌套反斜杠 / 引号深度异常
//
// 解析失败时 fail-closed (Allow=false)，遵循 design.md Decision 2。
// 每个 finding 携带 Severity (Critical/High/Medium/Low) + Rule ID + 修复建议。
//
// DM-20260629-002 devrix-d2-dsaft-restructuring PR-3: types + Analyze
// orchestrator kept here. Helper modules:
//   - rules.go: defaultRules + DefaultRule + AllRules + compileZshAttacks +
//     defaultDangerousWords + checkString + fillRule (rule registry + string pre-pass)
//   - ast_walker.go: (*Analyzer).walk + package walk function (AST traversal)
package sandboxast

import (
	"regexp"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// Severity 严重度等级 (TOOL-SEC-2-A02-F03 规则元数据)。
type Severity string

const (
	SeverityCritical Severity = "critical" // 立即拒绝 (eval/exec, 嵌套 heredoc)
	SeverityHigh     Severity = "high"     // 拒绝 + 高优先级 (zsh 攻击面, /dev/sda)
	SeverityMedium   Severity = "medium"   // 拒绝 + 警告 (command/process subst, backtick)
	SeverityLow      Severity = "low"      // 拒绝 + 提示 (嵌套反斜杠)
)

// Rule 规则元数据：用于工具调用者解释"为什么被拒 + 怎么改"。
type Rule struct {
	ID          string   // e.g. "ZSH-001", "HEREDOC-002"
	Severity    Severity // Critical/High/Medium/Low
	Description string   // 一句话描述规则
	Suggestion  string   // 修复建议 (e.g. "use read -p instead of eval")
}

// FindingKind AST finding 类别。
type FindingKind string

const (
	FindingHeredocInjection  FindingKind = "heredoc_injection"
	FindingNestedHeredoc     FindingKind = "nested_heredoc"
	FindingProcessSubst      FindingKind = "process_substitution"
	FindingCommandSubst      FindingKind = "command_substitution"
	FindingZshAttack         FindingKind = "zsh_attack_surface"
	FindingDangerousRedirect FindingKind = "dangerous_redirect"
	FindingShebangInjection  FindingKind = "shebang_injection"
	FindingNestedEscape      FindingKind = "nested_escape"
	FindingDangerousWord     FindingKind = "dangerous_word"
	FindingEvalCall          FindingKind = "eval_call"
)

// Finding 单条 AST finding。
type Finding struct {
	Kind     FindingKind
	RuleID   string   // 关联的 Rule ID, e.g. "ZSH-001"
	Severity Severity // Critical/High/Medium/Low
	Snippet  string
	Line     int
	Column   int
	Reason   string
	Fix      string // 修复建议 (来自 Rule.Suggestion)
}

// Verdict 分析结果。
type Verdict struct {
	Allow    bool
	Reason   string
	Findings []Finding
}

// Analyzer 可配置分析器。
type Analyzer struct {
	parser         *syntax.Parser
	zshAttack      []*regexp.Regexp
	dangerousWords map[string]string
}

// NewAnalyzer 构造带默认规则的 analyzer。
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		parser:         syntax.NewParser(syntax.KeepComments(false)),
		zshAttack:      compileZshAttacks(),
		dangerousWords: defaultDangerousWords(),
	}
}

// Analyze 主入口。fail-closed (W4: design.md Decision 2)。
//
// DM-20260629-002 PR-3: orchestrator-only after the god-fn split. The actual
// rule checks live in rules.go (checkString pre-pass) and ast_walker.go
// (per-AST-node rules). Analyze owns the parse + verdict combination.
func (a *Analyzer) Analyze(cmd string) Verdict {
	defer func() {
		// R3: panic 兜底 → Deny with reason
		if r := recover(); r != nil {
			_ = r // swallow but final Allow already set false below
		}
	}()

	stringFindings := a.checkString(cmd)

	reader := strings.NewReader(cmd)
	prog, err := a.parser.Parse(reader, "")
	if err != nil {
		// W4: fail-closed — parse 失败立即拒绝。
		// stringFindings 仍保留供调用者诊断。
		parseErrFinding := Finding{
			Kind:     FindingDangerousWord, // 复用 enum (无专门 kind)
			Severity: SeverityHigh,
			RuleID:   "PARSE-001",
			Reason:   "AST parse failed: " + err.Error(),
			Fix:      "检查 bash 语法 (引号闭合、heredoc 终止符、escaping)",
		}
		all := append([]Finding{parseErrFinding}, stringFindings...)
		return Verdict{
			Allow:    false,
			Reason:   "AST parse failed: " + err.Error(),
			Findings: all,
		}
	}

	astFindings := a.walk(prog)
	all := append(astFindings, stringFindings...)
	v := Verdict{Allow: true, Findings: all}
	if len(all) > 0 {
		v.Allow = false
		v.Reason = firstReason(all)
	}
	return v
}

// firstReason returns the first finding's reason (used as the canonical
// deny reason when one or more findings exist).
func firstReason(findings []Finding) string {
	if len(findings) == 0 {
		return ""
	}
	return findings[0].Reason
}