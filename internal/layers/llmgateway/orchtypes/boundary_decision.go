// Package orchtypes holds D3-level cross-cutting governance constants.
//
// Boundary Debt Decisions (DM-20260629-003 PR-7, T41-T44):
//
// 4 项跨域边界决策在 D3 域 v3.x 全部 RESOLVED。每项分配 boundary-debt ID
// 以便后续 v4.0+ 重新评估时精准追溯。所有边界由
// openspec/specs/architecture/cross-domain-boundaries.md §2 (D3 跨域边界) 登记。
package orchtypes

const (
	// BoundaryD2D3ImportBan 标记 D2 → D3 任何 import / 调用在 v1.0 起
	// 硬阻断。CI 通过 ./scripts/lint-d1-imports.sh (extends to D2→D3)
	// 守门。归属决策 RESOLVED（DM-020 落地 + v1.0 规格登记 + v2.0 import lint）。
	BoundaryD2D3ImportBan = "boundary-debt:d2-d3-import-ban-v1.0"

	// BoundaryD3S5VsD2S18Grayzone 标记 D3-S5 GuardContent（前置内容过滤）
	// vs D2-S18 PermissionMode（tool execution 权限）灰区。v3.0 R2 命题 E
	// 决议固化：**D3 优先拒**（前置过滤），D2 兜底。归属决策 RESOLVED。
	BoundaryD3S5VsD2S18Grayzone = "boundary-debt:d3-s5-vs-d2-s18-grayzone-v3.0"

	// BoundaryD3S4BudgetSpanInjection 标记 D3-S4 BudgetTokens 不直接 emit
	// span，采用"注入模式"通过 D3_LLM_Stream 顶层 span 暴露 budget.checked
	// attribute + budget.check.exceeded span event。Runtime 字面量稳定
	// 5 active span op + 1 span event (R1 Q3 决议)。归属决策 RESOLVED。
	BoundaryD3S4BudgetSpanInjection = "boundary-debt:d3-s4-budget-span-injection-v3.2"

	// BoundaryD3S6FailFastOnObsNil 标记 D3 启动期 obs == nil 时 fail-fast
	// 返回 ErrObservabilityRequired（不 silent fallback）。R3 P0 #8 实施 +
	// D7-A 决议。归属决策 RESOLVED。
	BoundaryD3S6FailFastOnObsNil = "boundary-debt:d3-s6-fail-fast-on-obs-nil-v1.1"
)
