package sandboxast

import (
	"fmt"

	"mvdan.cc/sh/v3/syntax"
)

// walk 遍历 AST 节点。
// W4 增强：1) 嵌套 heredoc 检测 (FindingNestedHeredoc)  2) heredoc body ProcSubst 也算  3) Finding 携带 Rule 字段
//
// DM-20260629-002 devrix-d2-dsaft-restructuring PR-3: extracted from analyzer.go
// (was an 82-LOC method on *Analyzer; the package-level walk helper function
// also moved here as it is structurally a single recursive descent over the
// mvdan.cc/sh AST).
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
//
// DM-20260629-002 PR-3: kept here next to (*Analyzer).walk so the recursive
// visitor and the AST-method visitor are co-located. The package-level walk
// is the only AST traversal helper the analyzer relies on.
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