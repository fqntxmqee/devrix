package contextengine

import (
	"fmt"
	"regexp"
	"strings"
)

// CommandPolicy validates bash commands against allow/deny rules and workspace constraints.
type CommandPolicy struct {
	Enabled      bool
	Allowlist    []string
	DenyPatterns []*regexp.Regexp
	WorkDirLock  bool
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
	`\bcurl\b.*\|.*\b(?:sh|bash|python|perl)\b`,
	`\bwget\b.*\|.*\b(?:sh|bash|python|perl)\b`,
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

	cmdName := extractCommandName(command)
	if cmdName == "" {
		return fmt.Errorf("empty command")
	}

	if !p.isAllowed(cmdName) {
		return fmt.Errorf("command not allowed: %s (add to tool.allowlist in config)", cmdName)
	}

	for _, pattern := range p.DenyPatterns {
		if pattern.MatchString(command) {
			return fmt.Errorf("dangerous command pattern detected: %s", pattern.String())
		}
	}

	if p.WorkDirLock && containsAbsPath(command) {
		return fmt.Errorf("absolute paths are not allowed")
	}

	return nil
}

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
