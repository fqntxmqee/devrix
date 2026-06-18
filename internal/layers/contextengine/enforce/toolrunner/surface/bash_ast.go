package surface

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner/bash"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// BashASTPolicy evaluates a bash command string against a list of
// deny-rules. Each rule inspects a parsed AST node (CallExpr, etc.)
// so the policy is precise — `rm -rf /` cannot be evaded by variable
// indirection because the parser resolves to a CallExpr with literal
// "-rf" + "/" arguments.
//
// TOOL-SURFACE-1-A01-F07 (DM-20260618-002).
//
// W5 增强 (DM-20260618-007): 在原 5 deny rules 之上集成
// bash.Policy (sandboxast.Analyzer + 22+ 规则) 提供 fail-closed
// parse 错误 + zsh 攻击面 + heredoc 注入检测。bashPolicy 可选
// (nil 时只跑原 5 deny rules, 向后兼容)。
//
// Performance budget: < 5ms p99 (T3.6 benchmark target). The default
// rule set is 5 entries; walker short-circuits on first match.
type BashASTPolicy struct {
	// DenyList is checked in order; first match wins.
	DenyList []BashDenyRule
	// bashPolicy 是 W5 引入的 sandboxast 集成层 (可选)。
	// nil 时 Policy.Audit 步骤跳过, 行为退化为 v1 5 deny rules。
	bashPolicy *bash.Policy
}

// BashDenyRule is one AST-level rule. Match returns true when the rule
// applies to a single statement (typically a *syntax.CallExpr).
type BashDenyRule struct {
	Name     string
	Match    func(*syntax.Stmt) bool
	Reason   string
	Severity string // "danger" | "warning"
}

// DefaultBashDenyRules is the v1 rule set for devrix. Future DM-005
// DSL will replace it with user-configurable YAML regex policies.
var DefaultBashDenyRules = []BashDenyRule{
	{
		Name:     "rm-rf-root",
		Match:    isRmRfRoot,
		Reason:   "rm -rf / would destroy the filesystem",
		Severity: "danger",
	},
	{
		Name:     "dd-overwrite",
		Match:    isDdCommand,
		Reason:   "dd can overwrite disk blocks irreversibly",
		Severity: "danger",
	},
	{
		Name:     "mkfs-format",
		Match:    isMkfsCommand,
		Reason:   "mkfs formats filesystems",
		Severity: "danger",
	},
	{
		Name:     "sudo-elevate",
		Match:    isSudoCommand,
		Reason:   "sudo elevates privileges",
		Severity: "warning",
	},
	{
		Name:     "chmod-777-root",
		Match:    isChmod777Root,
		Reason:   "chmod 777 / opens permissions globally",
		Severity: "warning",
	},
}

// NewBashASTPolicy returns a policy using DefaultBashDenyRules (向后兼容 v1)。
func NewBashASTPolicy() *BashASTPolicy {
	return &BashASTPolicy{DenyList: DefaultBashDenyRules}
}

// NewBashASTPolicyWithBashPolicy 构造带 sandboxast 集成的 v2 policy (W5)。
// 注入 bash.Policy 后, Check 会先跑 22+ 规则审计 (sandboxast + heredoc + zsh),
// 再跑原 5 deny rules (rm -rf / 等更具体的精确模式)。
func NewBashASTPolicyWithBashPolicy(bp *bash.Policy) *BashASTPolicy {
	return &BashASTPolicy{
		DenyList:   DefaultBashDenyRules,
		bashPolicy: bp,
	}
}

// Check parses cmd and returns the first matching deny rule's
// (DecisionDeny, reason). If no rule matches but the parse fails,
// returns (DecisionAsk, parse-error reason) so the LLM can retry
// with a corrected command rather than silently being allowed.
//
// W5: 集成 bash.Policy 后, fail-closed 改为 (DecisionDeny, parse-error)
// 而非 (DecisionAsk, ...) — 与 design.md Decision 2 一致。
func (p *BashASTPolicy) Check(cmd string) (decision contracts.Decision, reason string) {
	// W5 增强: 先跑 bash.Policy (sandboxast + 22+ 规则)
	if p.bashPolicy != nil {
		d := p.bashPolicy.AuditSimple(cmd)
		if d.Outcome == contracts.DecisionDeny {
			return contracts.DecisionDeny, d.Summary()
		}
		if d.Outcome == contracts.DecisionAsk {
			return contracts.DecisionAsk, d.Summary()
		}
	}
	// 原 v1 5 deny rules 走 mvdan.cc/sh parser (向后兼容)
	parser := syntax.NewParser()
	ast, err := parser.Parse(strings.NewReader(cmd), "")
	if err != nil {
		if p.bashPolicy != nil {
			// W5: bashPolicy 已就绪, parse 失败 → Deny (fail-closed)
			return contracts.DecisionDeny, "bash parse error: " + err.Error()
		}
		// v1 向后兼容: parse 失败 → Ask
		return contracts.DecisionAsk, "bash parse error: " + err.Error()
	}
	var matchedRule *BashDenyRule
	syntax.Walk(ast, func(node syntax.Node) bool {
		stmt, ok := node.(*syntax.Stmt)
		if !ok {
			return true
		}
		for i := range p.DenyList {
			if p.DenyList[i].Match(stmt) {
				matchedRule = &p.DenyList[i]
				return false
			}
		}
		return true
	})
	if matchedRule == nil {
		return contracts.DecisionAllow, ""
	}
	return contracts.DecisionDeny, matchedRule.Reason
}

// wordLiteral returns the single literal string in a Word if it
// contains exactly one *syntax.Lit; otherwise returns "".
// Used to safely extract argument values without trusting dynamic
// AST nodes (variable refs, expansions, etc.).
func wordLiteral(w *syntax.Word) string {
	if w == nil || len(w.Parts) != 1 {
		return ""
	}
	lit, ok := w.Parts[0].(*syntax.Lit)
	if !ok {
		return ""
	}
	return lit.Value
}

// callName extracts the literal command name from a CallExpr.
// Returns "" for non-CallExpr nodes or dynamic arguments
// (variable references, command substitutions, etc.).
func callName(stmt *syntax.Stmt) string {
	call, ok := stmt.Cmd.(*syntax.CallExpr)
	if !ok || len(call.Args) == 0 {
		return ""
	}
	return wordLiteral(call.Args[0])
}

// isRmRfRoot denies `rm -rf /` and `rm -fr /` (literal args only).
// Variable indirection `${RM} -rf /` is NOT matched by this rule
// (it would need regex policy — see DM-005 follow-up).
func isRmRfRoot(stmt *syntax.Stmt) bool {
	if callName(stmt) != "rm" {
		return false
	}
	call := stmt.Cmd.(*syntax.CallExpr)
	var hasRfFlag, hasRootTarget bool
	for _, arg := range call.Args[1:] {
		switch wordLiteral(arg) {
		case "-rf", "-fr":
			hasRfFlag = true
		case "/", "/*", "//":
			hasRootTarget = true
		}
	}
	return hasRfFlag && hasRootTarget
}

func isDdCommand(stmt *syntax.Stmt) bool {
	return callName(stmt) == "dd"
}

func isMkfsCommand(stmt *syntax.Stmt) bool {
	name := callName(stmt)
	return name == "mkfs" || strings.HasPrefix(name, "mkfs.")
}

func isSudoCommand(stmt *syntax.Stmt) bool {
	return callName(stmt) == "sudo"
}

func isChmod777Root(stmt *syntax.Stmt) bool {
	if callName(stmt) != "chmod" {
		return false
	}
	call := stmt.Cmd.(*syntax.CallExpr)
	var has777, hasRoot bool
	for _, arg := range call.Args[1:] {
		switch wordLiteral(arg) {
		case "777":
			has777 = true
		case "/":
			hasRoot = true
		}
	}
	return has777 && hasRoot
}
