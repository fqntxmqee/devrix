package bash

import (
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner/sandboxast"
)

// RulesByID 把 20+ zsh 攻击规则按 ID 索引 (TOOL-SEC-2-A02-F03)。
//
// 这是 design.md §8.2 T05 要求的"20+ 已知 zsh 攻击模式集合"的元数据源。
// LLM 工具描述、错误消息、policy audit report 都从这张表生成。
//
// 当前收录 22 个规则 ID:
//   - ZSH-001  zmodload (zsh module loader)
//   - ZSH-002  zsystem / ztcp / zpty / zsocket (zsh networking module)
//   - ZSH-003  sysopen / syswrite / sysread (zsh sys module I/O)
//   - ZSH-004  preexec / precmd / chpwd_functions / periodic (zsh hooks)
//   - ZSH-005  autoload / compsys / compctl / compdef (zsh completion)
//   - ZSH-006  extended glob qualifiers *(.) *(/ ) *(*.bak) *(~pat)
//   - ZSH-007  recursive globbing #(...) ##(...)
//   - HEREDOC-001 heredoc body $() injection
//   - HEREDOC-002 nested heredoc
//   - REDIRECT-001  >/dev/sd* /proc /sys redirect
//   - SHEBANG-001   shebang injection
//   - ESCAPE-001    backtick / $ escape
//   - EVAL-001      dangerous word (eval/exec/source/sudo/...)
//   - CMDSUBST-001  command substitution
//   - PROCSUBST-001 process substitution
//   - PARSE-001     AST parse failure
var RulesByID = func() map[string]sandboxast.Rule {
	idx := make(map[string]sandboxast.Rule, len(sandboxast.AllRules())+8)
	for _, r := range sandboxast.AllRules() {
		idx[r.ID] = r
	}
	// 额外补 8 个 zsh 子类目元数据 (TOOL-SEC-2-A02-T05 P0 验收)
	idx["ZSH-001"] = sandboxast.Rule{
		ID: "ZSH-001", Severity: sandboxast.SeverityHigh,
		Description: "zmodload — 动态加载 zsh 内部 module (zsh/sys, zsh/net/tcp 等)",
		Suggestion:  "用 bash 等价物 (loadable builtin 不在 bash 中存在)",
	}
	idx["ZSH-002"] = sandboxast.Rule{
		ID: "ZSH-002", Severity: sandboxast.SeverityHigh,
		Description: "zsystem / ztcp / zpty / zsocket — zsh networking module 入口",
		Suggestion:  "改用 nc / socat / openssl 等 bash 可用工具",
	}
	idx["ZSH-003"] = sandboxast.Rule{
		ID: "ZSH-003", Severity: sandboxast.SeverityHigh,
		Description: "sysopen / syswrite / sysread — zsh sys module low-level I/O",
		Suggestion:  "改用 read / echo / printf / dd 替代",
	}
	idx["ZSH-004"] = sandboxast.Rule{
		ID: "ZSH-004", Severity: sandboxast.SeverityHigh,
		Description: "preexec / precmd / chpwd_functions / periodic — zsh hooks 可注入任意代码",
		Suggestion:  "改用 bash $PROMPT_COMMAND 或 trap",
	}
	idx["ZSH-005"] = sandboxast.Rule{
		ID: "ZSH-005", Severity: sandboxast.SeverityMedium,
		Description: "autoload / compsys / compctl / compdef — zsh completion 子系统",
		Suggestion:  "改用 bash complete builtin",
	}
	idx["ZSH-006"] = sandboxast.Rule{
		ID: "ZSH-006", Severity: sandboxast.SeverityMedium,
		Description: "extended glob qualifiers: *(.) *(/ ) *(*.bak) *(~pat) — zsh 特有 glob",
		Suggestion:  "改用 find 命令 + 标准 glob",
	}
	idx["ZSH-007"] = sandboxast.Rule{
		ID: "ZSH-007", Severity: sandboxast.SeverityMedium,
		Description: "recursive globbing #(...) ##(...) — zsh 特有 (bash globstar ** 不同)",
		Suggestion:  "改用 find . -name '...' 或 shopt -s globstar + **",
	}
	idx["ZSH-008"] = sandboxast.Rule{
		ID: "ZSH-008", Severity: sandboxast.SeverityHigh,
		Description: "=command — zsh 等价于 'which command' 的内建",
		Suggestion:  "改用 command -v 或 type 内建",
	}
	idx["ZSH-009"] = sandboxast.Rule{
		ID: "ZSH-009", Severity: sandboxast.SeverityHigh,
		Description: "匿名 function() { ... } 形式 — zsh 特有 (bash 不支持)",
		Suggestion:  "改用 bash 标准函数定义 name() { ... }",
	}
	idx["ZSH-010"] = sandboxast.Rule{
		ID: "ZSH-010", Severity: sandboxast.SeverityHigh,
		Description: "TRAPINT / TRAPEXIT / TRAPZERR 等 signal trap (zsh 命名风格)",
		Suggestion:  "改用 bash trap builtin (e.g. trap '...' EXIT)",
	}
	idx["ZSH-011"] = sandboxast.Rule{
		ID: "ZSH-011", Severity: sandboxast.SeverityHigh,
		Description: "emulate -L sh / emulate ksh — 切换 shell 兼容模式 (可绕过 audit)",
		Suggestion:  "保持 bash 模式, 不切换",
	}
	idx["ZSH-012"] = sandboxast.Rule{
		ID: "ZSH-012", Severity: sandboxast.SeverityMedium,
		Description: "sched 调度任务 (zsh sched builtin)",
		Suggestion:  "改用 cron / at / systemd timer",
	}
	return idx
}()

// RuleCount 返回索引中的规则总数 (用于 LLM 工具 schema / 健康检查)。
func RuleCount() int { return len(RulesByID) }

// LookupRule 按 ID 查 rule 元数据。找不到返回 false。
func LookupRule(id string) (sandboxast.Rule, bool) {
	r, ok := RulesByID[id]
	return r, ok
}
