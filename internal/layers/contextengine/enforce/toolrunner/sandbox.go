package toolrunner

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

const sandboxPolicyHint = "This is a sandbox policy (not permission/YOLO); use relative paths under WorkDir or read_file/glob/list_dir for files."

// CommandPolicy validates bash commands against allow/deny rules and workspace constraints.
type CommandPolicy struct {
	Enabled      bool
	Allowlist    []string
	DenyPatterns []*regexp.Regexp
	WorkDirLock  bool
	// ASTAnalyzer 可选：G2 Bash AST 二次审计，在 regex 之前生效。
	// nil 时跳过 AST 检查（生产环境可通过 WireASTAnalyzer 注入）。
	ASTAnalyzer ASTAnalyzer
}

// ASTAnalyzer 由 sandboxast.Analyzer 实现的接口，便于 sandbox.go 不直接依赖 sandboxast。
type ASTAnalyzer interface {
	Analyze(cmd string) (allow bool, reason string)
}

var defaultAllowlist = []string{
	"ls", "cat", "head", "tail", "wc", "grep", "find",
	"git", "go", "python", "python3", "node", "npm",
	"echo", "printf", "date", "env", "pwd", "which",
	"mkdir", "cp", "mv", "touch", "chmod", "chown",
	"diff", "sort", "uniq", "cut", "tr", "sed", "awk",
}

var defaultDenyPatternStrings = []string{
	`\brm\s+(-[a-zA-Z]+\s+)*[/~]`,
	`\bsudo\b`,
	`\bcurl\b.*\|.*\b(?:sh|bash|python3?|perl|node|ruby)\b`,
	`\bwget\b.*\|.*\b(?:sh|bash|python3?|perl|node|ruby)\b`,
	`>[>]?\s*/dev/[a-z]`,
	`\bmkfifo\b`,
	`\bnc\s+-[lL]`,
	`\bchmod\s+.*[0-7]*7[0-7]*[0-7]*`,
	`:\(\)\s*\{\s*:`,
	`\breboot\b`,
	`\bshutdown\b`,
	`\bdd\s+if=`,
	`\bchroot\b`,
	`\$\(`,
	"`[^`]+`",
}

// DefaultCommandPolicy returns the production sandbox policy.
func DefaultCommandPolicy() *CommandPolicy {
	return NewCommandPolicy(true, nil, nil)
}

// NewCommandPolicy builds a policy from sandbox settings.
func NewCommandPolicy(enabled bool, allowlistExtra, denyPatternsExtra []string) *CommandPolicy {
	if !enabled {
		return &CommandPolicy{Enabled: false}
	}

	allow := append([]string{}, defaultAllowlist...)
	allow = append(allow, allowlistExtra...)

	patterns := compileDenyPatterns(append(defaultDenyPatternStrings, denyPatternsExtra...))
	return &CommandPolicy{
		Enabled:      true,
		Allowlist:    allow,
		DenyPatterns: patterns,
		WorkDirLock:  true,
	}
}

func compileDenyPatterns(patterns []string) []*regexp.Regexp {
	out := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if strings.TrimSpace(p) == "" {
			continue
		}
		out = append(out, regexp.MustCompile(p))
	}
	return out
}

// Validate checks whether a bash command is allowed to run.
func (p *CommandPolicy) Validate(command string) error {
	if p == nil || !p.Enabled {
		return nil
	}

	// G2 AST 前置：先经 mvdan.cc/sh 解析
	if p.ASTAnalyzer != nil {
		if allow, reason := p.ASTAnalyzer.Analyze(command); !allow {
			return fmt.Errorf("sandbox: ast block: %s. %s", reason, sandboxPolicyHint)
		}
	}

	cmdName := extractCommandName(command)
	if cmdName == "" {
		return fmt.Errorf("empty command")
	}

	// Denylist-only: block known-dangerous patterns; do not require yaml allowlist entries.
	scrubbed := scrubBenignDevNullRedirects(command)
	for _, pattern := range p.DenyPatterns {
		if pattern.MatchString(scrubbed) {
			return fmt.Errorf("sandbox: dangerous command pattern detected: %s. %s", pattern.String(), sandboxPolicyHint)
		}
	}

	if p.WorkDirLock && containsAbsPath(command) {
		return fmt.Errorf("sandbox: absolute paths outside workspace are not allowed in bash. %s", sandboxPolicyHint)
	}

	return nil
}

// isAllowed reports whether cmdName is on the optional extra allowlist (legacy tightening).
// Default sandbox policy does not use a command whitelist.
func (p *CommandPolicy) isAllowed(cmdName string) bool {
	cmdName = strings.ToLower(strings.TrimSpace(cmdName))
	for _, allowed := range p.Allowlist {
		if cmdName == strings.ToLower(strings.TrimSpace(allowed)) {
			return true
		}
	}
	return false
}

func extractCommandName(command string) string {
	cmd := strings.TrimSpace(command)
	for _, sep := range []string{"|", "&&", "||", ";"} {
		if idx := strings.Index(cmd, sep); idx >= 0 {
			cmd = cmd[:idx]
		}
	}
	cmd = strings.TrimSpace(cmd)
	if idx := strings.Index(cmd, " "); idx >= 0 {
		return cmd[:idx]
	}
	return cmd
}

func containsAbsPath(command string) bool {
	for _, field := range strings.Fields(command) {
		field = strings.Trim(field, `"'`)
		if strings.HasPrefix(field, "/") || strings.HasPrefix(field, "~/") {
			return true
		}
	}
	return strings.Contains(command, " /") || strings.Contains(command, "\t/")
}

// NormalizeWorkspacePaths rewrites absolute paths under workDir to workspace-relative paths.
func NormalizeWorkspacePaths(workDir, command string) string {
	workDir = filepath.Clean(workDir)
	if workDir == "" {
		return command
	}

	childPrefix := workDir + string(filepath.Separator)
	if strings.Contains(command, childPrefix) {
		command = strings.ReplaceAll(command, childPrefix, "")
	}

	return replaceAbsolutePathToken(command, workDir, ".")
}

// scrubBenignDevNullRedirects removes common stderr-null redirects before deny-pattern checks.
func scrubBenignDevNullRedirects(command string) string {
	for _, token := range []string{"2>/dev/null", "1>/dev/null", ">/dev/null"} {
		command = strings.ReplaceAll(command, token, "")
	}
	return command
}

func replaceAbsolutePathToken(command, absPath, rel string) string {
	if absPath == "" || !strings.Contains(command, absPath) {
		return command
	}

	delims := []string{" ", "\t", ";", "|", "&&", "||", "\"", "'", ")", ">", "\n"}
	for _, d := range delims {
		command = strings.ReplaceAll(command, absPath+d, rel+d)
	}
	if strings.HasPrefix(command, absPath) {
		command = rel + command[len(absPath):]
	}
	return command
}
