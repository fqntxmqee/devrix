package bash

import (
	"mvdan.cc/sh/v3/syntax"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner/sandboxast"
)

// HeredocAudit 审计 AST 中所有 heredoc 节点。
//
// 检测两类问题 (W4 AC2):
//   - 同 Stmt 内多个 heredoc → 视为嵌套 (FindingNestedHeredoc, critical)
//   - 单 heredoc body 内含 CmdSubst/ProcSubst → 视为注入 (FindingHeredocInjection, critical)
//
// 返回的 finding 已经填好 Severity/RuleID/Fix, 调用方可直接序列化给 LLM。
//
// 纯函数无副作用。sandboxast.Analyzer 实例由调用方复用（zero-cost 共享 zshAttack
// 编译后的 regex set）。
func HeredocAudit(file *syntax.File) []sandboxast.Finding {
	return scanHeredocs(file)
}

// scanHeredocs 实际扫描逻辑 — 抽出来便于测试。
func scanHeredocs(file *syntax.File) []sandboxast.Finding {
	out := []sandboxast.Finding{}
	if file == nil {
		return out
	}
	syntax.Walk(file, func(node syntax.Node) bool {
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
		// 嵌套: 单 Stmt 多 heredoc
		if len(hdocRedirs) >= 2 {
			rule := sandboxast.DefaultRule(sandboxast.FindingNestedHeredoc)
			out = append(out, sandboxast.Finding{
				Kind:     sandboxast.FindingNestedHeredoc,
				RuleID:   rule.ID,
				Severity: rule.Severity,
				Snippet:  heredocSnippet(stmt, len(hdocRedirs)),
				Line:     int(stmt.Pos().Line()),
				Reason:   rule.Description,
				Fix:      rule.Suggestion,
			})
		}
		// 单 heredoc body 审计
		for _, r := range hdocRedirs {
			if r.Hdoc == nil {
				continue
			}
			for _, part := range r.Hdoc.Parts {
				if _, isCmd := part.(*syntax.CmdSubst); isCmd {
					rule := sandboxast.DefaultRule(sandboxast.FindingHeredocInjection)
					out = append(out, sandboxast.Finding{
						Kind:     sandboxast.FindingHeredocInjection,
						RuleID:   rule.ID,
						Severity: rule.Severity,
						Snippet:  "heredoc body has $()",
						Line:     int(r.Pos().Line()),
						Reason:   rule.Description,
						Fix:      rule.Suggestion,
					})
				}
				if _, isProc := part.(*syntax.ProcSubst); isProc {
					rule := sandboxast.DefaultRule(sandboxast.FindingHeredocInjection)
					out = append(out, sandboxast.Finding{
						Kind:     sandboxast.FindingHeredocInjection,
						RuleID:   rule.ID,
						Severity: rule.Severity,
						Snippet:  "heredoc body has <() / >()",
						Line:     int(r.Pos().Line()),
						Reason:   rule.Description,
						Fix:      rule.Suggestion,
					})
				}
			}
		}
		return true
	})
	return out
}

func heredocSnippet(stmt *syntax.Stmt, n int) string {
	return "stmt@" + itoa(int(stmt.Pos().Line())) + " has " + itoa(n) + " heredocs"
}

// itoa 极简 integer-to-string (避免 strconv 导入路径在 heredoc.go 暴露)。
// 实际是 fmt.Sprintf("%d", n) 的内联展开; 当 line/count < 10000 时 ok。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
