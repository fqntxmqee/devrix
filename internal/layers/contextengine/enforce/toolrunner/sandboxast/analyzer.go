// Package sandboxast — G2 Bash AST 安全分析器，对标 clawcode src/tools/BashTool/bashSecurity.ts。
//
// 基于 mvdan.cc/sh/v3 纯 Go shell parser，检测：
//   - heredoc body 内嵌危险命令
//   - process substitution <( ) / >( )
//   - command substitution $( ) / ` `
//   - zsh 攻击面 (zmodload, sysopen, =cmd 等)
//   - 危险重定向 (>/dev/sda, >&跨进程)
//   - 嵌套反斜杠 / 引号深度异常
//
// 解析失败时返回 Allow=true（fallback 到现有 regex 层）—— 遵循 R3 风险缓解。
package sandboxast

import (
	"fmt"
	"regexp"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// FindingKind AST finding 类别。
type FindingKind string

const (
	FindingHeredocInjection  FindingKind = "heredoc_injection"
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
	Kind    FindingKind
	Snippet string
	Line    int
	Column  int
	Reason  string
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

func defaultDangerousWords() map[string]string {
	return map[string]string{
		"eval":   "eval 可执行拼接字符串，常见注入手法",
		"exec":   "exec 替换当前 shell 进程，可能绕过 sandbox",
		"source": "source 引入外部脚本",
		".":      "等同于 source",
	}
}

func compileZshAttacks() []*regexp.Regexp {
	patterns := []string{
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
	}
	out := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		out = append(out, regexp.MustCompile(p))
	}
	return out
}

// Analyze 主入口。
func (a *Analyzer) Analyze(cmd string) Verdict {
	defer func() {
		// R3: panic 兜底 → Allow=true fallback regex
		_ = recover()
	}()

	stringFindings := a.checkString(cmd)

	reader := strings.NewReader(cmd)
	prog, err := a.parser.Parse(reader, "")
	if err != nil {
		v := Verdict{Allow: true, Findings: stringFindings}
		if len(stringFindings) > 0 {
			v.Allow = false
			v.Reason = "AST parse failed; matched string-level patterns"
		}
		return v
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

// checkString 在 AST 之前用正则兜底。
func (a *Analyzer) checkString(cmd string) []Finding {
	out := []Finding{}
	for _, re := range a.zshAttack {
		if m := re.FindStringIndex(cmd); m != nil {
			out = append(out, Finding{
				Kind:    FindingZshAttack,
				Snippet: cmd[m[0]:m[1]],
				Line:    1 + strings.Count(cmd[:m[0]], "\n"),
				Reason:  "zsh attack surface pattern: " + re.String(),
			})
		}
	}
	for _, p := range []*regexp.Regexp{
		regexp.MustCompile(`>\s*/dev/(?:s|h|xv)d[a-z]\b`),
		regexp.MustCompile(`>\s*/proc/self/`),
		regexp.MustCompile(`>\s*/sys/`),
	} {
		if m := p.FindStringIndex(cmd); m != nil {
			out = append(out, Finding{
				Kind:    FindingDangerousRedirect,
				Snippet: cmd[m[0]:m[1]],
				Line:    1 + strings.Count(cmd[:m[0]], "\n"),
				Reason:  "dangerous redirect to sensitive device",
			})
		}
	}
	if strings.Contains(cmd, `\$\(`) {
		out = append(out, Finding{
			Kind:    FindingNestedEscape,
			Snippet: `$\(`,
			Reason:  "escaped command substitution bypass",
		})
	}
	// 反引号命令替换：原始 grep 抓不到（backticks 容易和模板字符串混淆）
	if strings.Contains(cmd, "`") {
		out = append(out, Finding{
			Kind:    FindingNestedEscape,
			Snippet: "`",
			Reason:  "backtick command substitution is dangerous and hard to track",
		})
	}
	return out
}

// walk 遍历 AST 节点。
func (a *Analyzer) walk(file *syntax.File) []Finding {
	out := []Finding{}
	if file == nil {
		return out
	}
	_ = walk(file, func(node syntax.Node) bool {
		switch n := node.(type) {
		case *syntax.CmdSubst:
			out = append(out, Finding{
				Kind:    FindingCommandSubst,
				Snippet: fmt.Sprintf("$() at line %d", n.Pos().Line()),
				Line:    int(n.Pos().Line()),
				Reason:  "command substitution $() can execute nested payloads",
			})
		case *syntax.ProcSubst:
			out = append(out, Finding{
				Kind:    FindingProcessSubst,
				Snippet: fmt.Sprintf("process substitution at line %d", n.Pos().Line()),
				Line:    int(n.Pos().Line()),
				Reason:  "process substitution <() / >() can mask command execution",
			})
		case *syntax.CallExpr:
			if len(n.Args) > 0 && len(n.Args[0].Parts) > 0 {
				if w, ok := n.Args[0].Parts[0].(*syntax.Lit); ok {
					name := w.Value
					if reason, found := a.dangerousWords[name]; found {
						out = append(out, Finding{
							Kind:    FindingEvalCall,
							Snippet: name,
							Line:    int(n.Pos().Line()),
							Reason:  reason,
						})
					}
				}
			}
		}
		return true
	})

	// Heredoc body 单独审计：检查 Hdoc.Parts 含 CmdSubst → 提升为 HeredocInjection
	_ = walk(file, func(node syntax.Node) bool {
		r, ok := node.(*syntax.Redirect)
		if !ok {
			return true
		}
		if r.Op != syntax.Hdoc && r.Op != syntax.DashHdoc {
			return true
		}
		if r.Hdoc == nil {
			return false
		}
		for _, part := range r.Hdoc.Parts {
			if _, isCmd := part.(*syntax.CmdSubst); isCmd {
				out = append(out, Finding{
					Kind:    FindingHeredocInjection,
					Snippet: "heredoc body has command substitution",
					Line:    int(r.Pos().Line()),
					Reason:  "heredoc body contains $(...) — shell will execute nested command on body expansion",
				})
				return false
			}
		}
		return false
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
