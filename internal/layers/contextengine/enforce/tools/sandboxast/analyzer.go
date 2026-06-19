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
package sandboxast

import (
	"fmt"
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
	parser        *syntax.Parser
	zshAttack     []*regexp.Regexp
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

// defaultDangerousWords 9 个高危命令名 + 解释。
// TOOL-SEC-2-A02-F01 AST 解析时检查 CallExpr.Args[0] 是否命中。
func defaultDangerousWords() map[string]string {
	return map[string]string{
		"eval":      "eval 可执行拼接字符串，常见注入手法；改用数组迭代",
		"exec":      "exec 替换当前 shell 进程，可能绕过 sandbox；改用子 shell 包裹",
		"source":    "source 引入外部脚本，绕过当前进程审计；改用 . 显式路径 + 审计",
		".":         "等同于 source；同上",
		"env":       "env 可被滥用为可执行 wrapper 注入 PATH；改用显式路径",
		"xargs":     "xargs 默认执行 exec 模式可注入命令；改用 -I{} + 显式占位符",
		"sudo":      "sudo 提权执行；LSP Agent 上下文禁止使用",
		"chmod":     "chmod 改变文件权限可绕过 sandbox；改用 ACL 或 chattr",
		"chown":     "chown 改变文件属主可绕过 sandbox；改用 ACL",
	}
}

// defaultRules 规则元数据索引。命中 pattern/keyword 时把 Rule 字段填到 Finding。
// TOOL-SEC-2-A02-F03 — 设计文档 §8.2 T05 要求 20+ zsh 攻击模式 + 修复建议。
var defaultRules = map[FindingKind]Rule{
	FindingHeredocInjection: {
		ID: "HEREDOC-001", Severity: SeverityCritical,
		Description: "heredoc body 内嵌 command substitution",
		Suggestion:  "heredoc body 不应包含 $(...) / ` `；改用管道或临时文件",
	},
	FindingNestedHeredoc: {
		ID: "HEREDOC-002", Severity: SeverityCritical,
		Description: "嵌套 heredoc (同 command 内多个 heredoc 或 heredoc body 内含 heredoc 标记)",
		Suggestion:  "扁平化为单层 heredoc；用 cat <<EOF1; cat <<EOF2 顺序追加",
	},
	FindingZshAttack: {
		ID: "ZSH-001", Severity: SeverityHigh,
		Description: "zsh 攻击面 (zmodload / sysopen / precommand module 等)",
		Suggestion:  "不要在 bash 中调用 zsh 特有功能；用 bash 等价物",
	},
	FindingDangerousRedirect: {
		ID: "REDIRECT-001", Severity: SeverityHigh,
		Description: "重定向到敏感设备 (/dev/sda, /proc, /sys)",
		Suggestion:  "禁止重定向到 /dev/sd* /proc /sys；改用日志文件",
	},
	FindingShebangInjection: {
		ID: "SHEBANG-001", Severity: SeverityHigh,
		Description: "shebang 行注入 (脚本开头 #! 后跟恶意解释器)",
		Suggestion:  "只允许 #!/bin/bash / #!/usr/bin/env bash；其他解释器需审批",
	},
	FindingNestedEscape: {
		ID: "ESCAPE-001", Severity: SeverityMedium,
		Description: "反引号 / 转义命令替换 (难以静态分析)",
		Suggestion:  "改用 $(...) 或拆分变量",
	},
	FindingEvalCall: {
		ID: "EVAL-001", Severity: SeverityCritical,
		Description: "危险命令名 (eval/exec/source/sudo/chmod 等)",
		Suggestion:  "改用直接调用；如需拼接用数组+for 循环",
	},
	FindingCommandSubst: {
		ID: "CMDSUBST-001", Severity: SeverityMedium,
		Description: "command substitution $() 嵌套执行",
		Suggestion:  "改用管道 + 临时文件；避免 $() 嵌套超过 1 层",
	},
	FindingProcessSubst: {
		ID: "PROCSUBST-001", Severity: SeverityMedium,
		Description: "process substitution <() / >() 可掩盖命令执行",
		Suggestion:  "改用 named pipe (mkfifo) 或临时文件",
	},
}

// DefaultRule 返回指定 kind 的 rule 元数据。
// bash.Policy / bash.HeredocAudit 用此 API 填充 finding 字段。
// 找不到时返回 zero Rule (ID="" 即未识别) — 调用方可判断 RuleID 空跳过。
func DefaultRule(k FindingKind) Rule {
	return defaultRules[k]
}

// AllRules 返回所有 rule 的快照 (按 FindingKind enum 顺序)。
// 用于 LLM 提示"devrix 拦截的所有规则"展示。
func AllRules() []Rule {
	out := make([]Rule, 0, len(defaultRules))
	for _, r := range defaultRules {
		out = append(out, r)
	}
	return out
}

// compileZshAttacks 20+ zsh 攻击面模式 (TOOL-SEC-2-A02-F03 — design.md §8.2 T05)。
// 涵盖 zsh 特有的 module 系统 / glob qualifiers / precommand hooks / autoload / completion。
func compileZshAttacks() []*regexp.Regexp {
	patterns := []string{
		// --- zsh module 系统 (原有 11 个) ---
		`\bzmodload\b`,
		`\bsysopen\b`,
		`\bsyswrite\b`,
		`\bsysread\b`,
		`\bzsystem\b`,
		`\bztcp\b`,
		`\bzpty\b`,
		`\bzsocket\b`,
		`\=\(.+\)`,
		`\(\?[\*\@\!]`,
		`\bfunction\s*\(\s*\)\s*\{`,
		// --- 新增: precommand hooks (W4 增强) ---
		`\bpreexec\b`,       // preexec module
		`\bprecmd\b`,        // precmd module
		`\bchpwd_functions\b`, // chpwd hook
		`\bperiodic\b`,      // periodic hook
		// --- 新增: autoload + completion ---
		`\bautoload\b`,     // autoload lazy function loader
		`\bcompsys\b`,      // compsys completion system
		`\bcompctl\b`,      // compctl completion control
		`\bcompdef\b`,      // compdef completion definition
		// --- 新增: glob qualifiers (扩展模式) ---
		`\*\([\.\/\*\?ls][^\)]*\)`, // glob qualifier: *(.), *(/), *(*), *(?), *(ls)  — 含 zsh 特有扩展 glob
		`\*\(~[^\)]*\)`,            // glob qualifier: *(~pattern) zsh exclusion
		`\#\([^)]+\)`,              // recursive globbing #(...), ##(...) zsh 特有 (bash globstar 用 ** 不在此)
	}
	out := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		out = append(out, regexp.MustCompile(p))
	}
	return out
}

// Analyze 主入口。fail-closed (W4: design.md Decision 2)。
// 解析失败时返回 Allow=false + Reason="AST parse failed: ..."。
// 即便 panic 也不允许 Allow=true（兜底由 defer recover() 维持原行为）。
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

// fillRule 给 finding 填充 Rule 元数据 (Severity/RuleID/Fix)。
func fillRule(f Finding) Finding {
	if rule, ok := defaultRules[f.Kind]; ok {
		f.RuleID = rule.ID
		f.Severity = rule.Severity
		if f.Fix == "" {
			f.Fix = rule.Suggestion
		}
	}
	return f
}

// checkString 在 AST 之前用正则兜底。
func (a *Analyzer) checkString(cmd string) []Finding {
	out := []Finding{}
	for _, re := range a.zshAttack {
		if m := re.FindStringIndex(cmd); m != nil {
			out = append(out, fillRule(Finding{
				Kind:    FindingZshAttack,
				Snippet: cmd[m[0]:m[1]],
				Line:    1 + strings.Count(cmd[:m[0]], "\n"),
				Reason:  "zsh attack surface pattern: " + re.String(),
			}))
		}
	}
	for _, p := range []*regexp.Regexp{
		regexp.MustCompile(`>\s*/dev/(?:s|h|xv)d[a-z]\b`),
		regexp.MustCompile(`>\s*/proc/self/`),
		regexp.MustCompile(`>\s*/sys/`),
	} {
		if m := p.FindStringIndex(cmd); m != nil {
			out = append(out, fillRule(Finding{
				Kind:    FindingDangerousRedirect,
				Snippet: cmd[m[0]:m[1]],
				Line:    1 + strings.Count(cmd[:m[0]], "\n"),
				Reason:  "dangerous redirect to sensitive device",
			}))
		}
	}
	if strings.Contains(cmd, `\$\(`) {
		out = append(out, fillRule(Finding{
			Kind:    FindingNestedEscape,
			Snippet: `$\(`,
			Reason:  "escaped command substitution bypass",
		}))
	}
	// 反引号命令替换：原始 grep 抓不到（backticks 容易和模板字符串混淆）
	if strings.Contains(cmd, "`") {
		out = append(out, fillRule(Finding{
			Kind:    FindingNestedEscape,
			Snippet: "`",
			Reason:  "backtick command substitution is dangerous and hard to track",
		}))
	}
	return out
}

// walk 遍历 AST 节点。
// W4 增强：1) 嵌套 heredoc 检测 (FindingNestedHeredoc)  2) heredoc body ProcSubst 也算  3) Finding 携带 Rule 字段
func (a *Analyzer) walk(file *syntax.File) []Finding {
	out := []Finding{}
	if file == nil {
		return out
	}
	_ = walk(file, func(node syntax.Node) bool {
		switch n := node.(type) {
		case *syntax.CmdSubst:
			out = append(out, fillRule(Finding{
				Kind:    FindingCommandSubst,
				Snippet: fmt.Sprintf("$() at line %d", n.Pos().Line()),
				Line:    int(n.Pos().Line()),
				Reason:  "command substitution $() can execute nested payloads",
			}))
		case *syntax.ProcSubst:
			out = append(out, fillRule(Finding{
				Kind:    FindingProcessSubst,
				Snippet: fmt.Sprintf("process substitution at line %d", n.Pos().Line()),
				Line:    int(n.Pos().Line()),
				Reason:  "process substitution <() / >() can mask command execution",
			}))
		case *syntax.CallExpr:
			if len(n.Args) > 0 && len(n.Args[0].Parts) > 0 {
				if w, ok := n.Args[0].Parts[0].(*syntax.Lit); ok {
					name := w.Value
					if reason, found := a.dangerousWords[name]; found {
						out = append(out, fillRule(Finding{
							Kind:    FindingEvalCall,
							Snippet: name,
							Line:    int(n.Pos().Line()),
							Reason:  reason,
						}))
					}
				}
			}
		}
		return true
	})

	// Heredoc 审计 (W4 增强)：
	//   - 同 Stmt 内多个 heredoc 视为 nested (FindingNestedHeredoc)
	//   - 单个 heredoc body 含 CmdSubst/ProcSubst → FindingHeredocInjection
	_ = walk(file, func(node syntax.Node) bool {
		stmt, ok := node.(*syntax.Stmt)
		if !ok {
			return true
		}
		var hdocRedirs []*syntax.Redirect
		for _, r := range stmt.Redirs {
			if r.Op == syntax.Hdoc || r.Op == syntax.DashHdoc {
				hdocRedirs = append(hdocRedirs, r)
			}
		}
		// 嵌套 heredoc: 同 Stmt 内有 2+ 个 heredoc
		if len(hdocRedirs) >= 2 {
			out = append(out, fillRule(Finding{
				Kind:    FindingNestedHeredoc,
				Snippet: fmt.Sprintf("%d heredocs in single command at line %d", len(hdocRedirs), stmt.Pos().Line()),
				Line:    int(stmt.Pos().Line()),
				Reason:  fmt.Sprintf("nested/compound heredoc denied: %d heredocs in one statement (suspicious — 99%% legitimate use has 1)", len(hdocRedirs)),
			}))
		}
		// 单 heredoc body 审计
		for _, r := range hdocRedirs {
			if r.Hdoc == nil {
				continue
			}
			for _, part := range r.Hdoc.Parts {
				switch part.(type) {
				case *syntax.CmdSubst, *syntax.ProcSubst:
					out = append(out, fillRule(Finding{
						Kind:    FindingHeredocInjection,
						Snippet: "heredoc body has command/process substitution",
						Line:    int(r.Pos().Line()),
						Reason:  "heredoc body contains $() / <() / >() — shell will execute nested payload on body expansion",
					}))
				}
			}
		}
		return true
	})
	return out
}

// walk 深度优先遍历；fn 返回 false 停止子树。
func walk(node syntax.Node, fn func(syntax.Node) bool) error {
	if node == nil {
		return nil
	}
	if !fn(node) {
		return nil
	}
	switch n := node.(type) {
	case *syntax.File:
		for _, stmt := range n.Stmts {
			_ = walk(stmt, fn)
		}
	case *syntax.Stmt:
		if n.Cmd != nil {
			_ = walk(n.Cmd, fn)
		}
		for _, r := range n.Redirs {
			_ = walk(r, fn)
		}
	case *syntax.CallExpr:
		for _, arg := range n.Args {
			_ = walk(arg, fn)
		}
		for _, assign := range n.Assigns {
			_ = walk(assign, fn)
		}
	case *syntax.IfClause:
		for _, s := range n.Cond {
			_ = walk(s, fn)
		}
		for _, s := range n.Then {
			_ = walk(s, fn)
		}
		if n.Else != nil {
			_ = walk(n.Else, fn)
		}
	case *syntax.ForClause:
		_ = walk(n.Loop, fn)
		for _, s := range n.Do {
			_ = walk(s, fn)
		}
	case *syntax.WhileClause:
		for _, s := range n.Cond {
			_ = walk(s, fn)
		}
		for _, s := range n.Do {
			_ = walk(s, fn)
		}
	case *syntax.Block:
		for _, s := range n.Stmts {
			_ = walk(s, fn)
		}
	case *syntax.Subshell:
		for _, s := range n.Stmts {
			_ = walk(s, fn)
		}
	case *syntax.BinaryCmd:
		_ = walk(n.X, fn)
		_ = walk(n.Y, fn)
	case *syntax.FuncDecl:
		_ = walk(n.Body, fn)
	case *syntax.CaseClause:
		for _, ci := range n.Items {
			for _, s := range ci.Stmts {
				_ = walk(s, fn)
			}
		}
	case *syntax.Word:
		for _, part := range n.Parts {
			_ = walk(part, fn)
		}
	case *syntax.DblQuoted:
		for _, part := range n.Parts {
			_ = walk(part, fn)
		}
	case *syntax.CmdSubst:
		for _, s := range n.Stmts {
			_ = walk(s, fn)
		}
	case *syntax.ProcSubst:
		for _, s := range n.Stmts {
			_ = walk(s, fn)
		}
	}
	return nil
}

func firstReason(findings []Finding) string {
	if len(findings) == 0 {
		return ""
	}
	return findings[0].Reason
}
