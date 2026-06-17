package surface

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"

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
// Performance budget: < 5ms p99 (T3.6 benchmark target). The default
// rule set is 5 entries; walker short-circuits on first match.
type BashASTPolicy struct {
	// DenyList is checked in order; first match wins.
	DenyList []BashDenyRule
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

// NewBashASTPolicy returns a policy using DefaultBashDenyRules.
func NewBashASTPolicy() *BashASTPolicy {
	return &BashASTPolicy{DenyList: DefaultBashDenyRules}
}

// Check parses cmd and returns the first matching deny rule's
// (DecisionDeny, reason). If no rule matches but the parse fails,
// returns (DecisionAsk, parse-error reason) so the LLM can retry
// with a corrected command rather than silently being allowed.
func (p *BashASTPolicy) Check(cmd string) (decision contracts.Decision, reason string) {
	parser := syntax.NewParser()
	ast, err := parser.Parse(strings.NewReader(cmd), "")
	if err != nil {
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
