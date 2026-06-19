// Package bash 提供 Bash AST 安全审计的 high-level API。
//
// 内部依赖 mvdan.cc/sh/v3 + sandboxast.Analyzer。
// 对 surface.BashSurface 暴露:
//   - Parse(cmd) (*syntax.File, error)   AST 解析入口 (fail-closed: ErrParseFailed)
//   - HeredocAudit(file) []Finding       heredoc body 审计 (W4 AC2)
//   - RulesByID                          20+ zsh 攻击规则元数据 (TOOL-SEC-2-A02-T05)
//   - Policy.Audit(cmd) Decision         高阶决策入口 (TOOL-SEC-2-A02-F01~F03)
//
// fail-closed 策略: 解析失败立即返回 error, 由调用方决定如何处理
// (Policy.Audit 转 Deny, 直接 surface.Execute 透传 error)。
package bash

import (
	"errors"
	"fmt"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// ErrParseFailed bash 解析失败的 sentinel error。
// 设计决策 design.md Decision 2: 解析失败 = 拒绝 (fail-closed)。
var ErrParseFailed = errors.New("bash: AST parse failed")

// Parse 解析 cmd 为 *syntax.File。失败时 wrap ErrParseFailed 供 errors.Is 识别。
// TOOL-SEC-2-A02-F01: AST 解析 (mvdan.cc/sh) — design.md §4.1.2 架构入口。
func Parse(cmd string) (*syntax.File, error) {
	parser := syntax.NewParser(syntax.KeepComments(false))
	prog, err := parser.Parse(strings.NewReader(cmd), "")
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrParseFailed, err.Error())
	}
	return prog, nil
}
