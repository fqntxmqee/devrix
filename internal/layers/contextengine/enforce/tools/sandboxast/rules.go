package sandboxast

import (
	"regexp"
	"strings"
)

// defaultDangerousWords 9 个高危命令名 + 解释。
// TOOL-SEC-2-A02-F01 AST 解析时检查 CallExpr.Args[0] 是否命中。
//
// DM-20260629-002 devrix-d2-dsaft-restructuring PR-3: extracted from analyzer.go
// to rules.go alongside DefaultRule / AllRules / compileZshAttacks / checkString.
func defaultDangerousWords() map[string]string {
	return map[string]string{
		"eval":   "eval 可执行拼接字符串，常见注入手法；改用数组迭代",
		"exec":   "exec 替换当前 shell 进程，可能绕过 sandbox；改用子 shell 包裹",
		"source": "source 引入外部脚本，绕过当前进程审计；改用 . 显式路径 + 审计",
		".":      "等同于 source；同上",
		"env":    "env 可被滥用为可执行 wrapper 注入 PATH；改用显式路径",
		"xargs":  "xargs 默认执行 exec 模式可注入命令；改用 -I{} + 显式占位符",
		"sudo":   "sudo 提权执行；LSP Agent 上下文禁止使用",
		"chmod":  "chmod 改变文件权限可绕过 sandbox；改用 ACL 或 chattr",
		"chown":  "chown 改变文件属主可绕过 sandbox；改用 ACL",
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
//
// DM-20260629-002 PR-3: extracted from analyzer.go.
func DefaultRule(k FindingKind) Rule {
	return defaultRules[k]
}

// AllRules 返回所有 rule 的快照 (按 FindingKind enum 顺序)。
// 用于 LLM 提示"devrix 拦截的所有规则"展示。
//
// DM-20260629-002 PR-3: extracted from analyzer.go.
func AllRules() []Rule {
	out := make([]Rule, 0, len(defaultRules))
	for _, r := range defaultRules {
		out = append(out, r)
	}
	return out
}

// compileZshAttacks 20+ zsh 攻击面模式 (TOOL-SEC-2-A02-F03 — design.md §8.2 T05)。
// 涵盖 zsh 特有的 module 系统 / glob qualifiers / precommand hooks / autoload / completion。
//
// DM-20260629-002 PR-3: extracted from analyzer.go.
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
		`\bpreexec\b`,          // preexec module
		`\bprecmd\b`,           // precmd module
		`\bchpwd_functions\b`,  // chpwd hook
		`\bperiodic\b`,         // periodic hook
		// --- 新增: autoload + completion ---
		`\bautoload\b`,  // autoload lazy function loader
		`\bcompsys\b`,   // compsys completion system
		`\bcompctl\b`,   // compctl completion control
		`\bcompdef\b`,   // compdef completion definition
		// --- 新增: glob qualifiers (扩展模式) ---
		`\*\([\.\/\*\?ls][^\)]*\)`, // glob qualifier: *(.), *(/), *(*), *(?), *(ls)  — 含 zsh 特有扩展 glob
		`\*\(~[^\)]*\)`,             // glob qualifier: *(~pattern) zsh exclusion
		`\#\([^)]+\)`,               // recursive globbing #(...), ##(...) zsh 特有 (bash globstar 用 ** 不在此)
	}
	out := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		out = append(out, regexp.MustCompile(p))
	}
	return out
}

// fillRule 给 finding 填充 Rule 元数据 (Severity/RuleID/Fix)。
//
// DM-20260629-002 PR-3: extracted from analyzer.go.
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
//
// DM-20260629-002 PR-3: extracted from analyzer.go (was 42 LOC).
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
	if strings.Contains(cmd, `$\(`) {
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