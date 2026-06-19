package bash

import (
	"context"
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner/sandboxast"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// Decision bash 审计的最终决策 (W5 high-level API)。
// 比 contracts.Decision 三态多出命中规则列表和修复建议。
type Decision struct {
	Outcome   contracts.Decision      // Allow / Deny / Ask
	Reason    string                  // 第一个命中的 finding reason (或空)
	Findings  []sandboxast.Finding    // 所有命中
	RuleIDs   []string                // 命中规则 ID 列表 (e.g. "ZSH-001", "EVAL-001")
	Severity  sandboxast.Severity     // 最高严重度 (空 = Allow)
	Fixes     []string                // 修复建议 (去重)
}

// Policy bash AST 审计策略 (TOOL-SEC-2-A02-F01~F03 整合)。
//
// 单一入口: Audit(cmd) Decision
//   1) Parse 失败 → Deny (fail-closed, design.md Decision 2)
//   2) AST 走 sandboxast.Analyzer 拿 finding 列表
//   3) 任何 critical/high finding → Deny; medium/low 累积
//   4) medium ≥ 3 个 → Ask (让人复核)
//
// 纯函数 (除复用 sandboxast.Analyzer 内部 regex 编译外)，可安全并发调用。
type Policy struct {
	analyzer *sandboxast.Analyzer
}

// NewPolicy 构造默认 Policy。
func NewPolicy() *Policy {
	return &Policy{analyzer: sandboxast.NewAnalyzer()}
}

// Audit 审计 cmd 返回 Decision。
func (p *Policy) Audit(ctx context.Context, cmd string) Decision {
	_ = ctx // 预留: 后续可加 ctx-bound metric, 暂未使用
	v := p.analyzer.Analyze(cmd)
	d := Decision{Findings: v.Findings}
	if v.Allow {
		d.Outcome = contracts.DecisionAllow
		return d
	}
	d.Reason = v.Reason
	maxSev := sandboxast.SeverityLow
	ruleIDSet := map[string]struct{}{}
	fixSet := map[string]struct{}{}
	for _, f := range v.Findings {
		if rankSeverity(f.Severity) > rankSeverity(maxSev) {
			maxSev = f.Severity
		}
		if f.RuleID != "" {
			ruleIDSet[f.RuleID] = struct{}{}
		}
		if f.Fix != "" {
			fixSet[f.Fix] = struct{}{}
		}
	}
	d.Severity = maxSev
	for id := range ruleIDSet {
		d.RuleIDs = append(d.RuleIDs, id)
	}
	for fix := range fixSet {
		d.Fixes = append(d.Fixes, fix)
	}
	// 决策规则
	mediumCount := countBySeverity(v.Findings, sandboxast.SeverityMedium)
	if maxSev == sandboxast.SeverityCritical || maxSev == sandboxast.SeverityHigh {
		d.Outcome = contracts.DecisionDeny
		return d
	}
	if mediumCount >= 3 {
		d.Outcome = contracts.DecisionAsk
		return d
	}
	d.Outcome = contracts.DecisionDeny
	return d
}

// AuditSimple 是 Audit 的非 ctx 简化版 (供 caller 不需要 ctx 的场景)。
func (p *Policy) AuditSimple(cmd string) Decision {
	return p.Audit(context.TODO(), cmd)
}

func rankSeverity(s sandboxast.Severity) int {
	switch s {
	case sandboxast.SeverityCritical:
		return 4
	case sandboxast.SeverityHigh:
		return 3
	case sandboxast.SeverityMedium:
		return 2
	case sandboxast.SeverityLow:
		return 1
	}
	return 0
}

func countBySeverity(findings []sandboxast.Finding, s sandboxast.Severity) int {
	n := 0
	for _, f := range findings {
		if f.Severity == s {
			n++
		}
	}
	return n
}

// Summary 返回 decision 的人类可读摘要 (含 RuleIDs + 修复建议 + Reason)。
// 用于 surface.Execute 返回的 ToolResult.Error / 数据展示。
func (d Decision) Summary() string {
	if d.Outcome == contracts.DecisionAllow {
		return ""
	}
	var b strings.Builder
	b.WriteString("[" + string(d.Severity) + "] " + d.Reason)
	if len(d.RuleIDs) > 0 {
		b.WriteString(" (rules: ")
		b.WriteString(strings.Join(d.RuleIDs, ", "))
		b.WriteString(")")
	}
	if len(d.Fixes) > 0 {
		b.WriteString("\n  fixes: ")
		b.WriteString(strings.Join(d.Fixes, "; "))
	}
	return b.String()
}
