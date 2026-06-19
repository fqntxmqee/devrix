package surface

import (
	"github.com/devrix/devrix/internal/shared/ltllite"
)

// LTL-Lite invariant declarations for LSPToolSurface (DM-20260618-007 W15)。
//
// 这些 invariant 表达"5 typed LSP method 应当满足的跨切面约束"。
// runtime 校验由 ltllite.Check 触发; static 扫描由 ci-lint-invariant
// (W15 tools/ci-lint-invariant/main.go) 在 PR pipeline 中执行。
//
// invariant tag 语法: "pre => post"。
// 评估规则: pre=true 时要求 post=true; pre=false 时跳过。
// State 提供 Eval(prop) bool — 实现方负责把"surface 状态"映射到命题。
//
// 4 条核心 invariant:
//   - is_typed_method: lsp_go_to_definition 等 5 个 method 都是 typed
//   - read_only: 所有 5 个 method 都是只读 (ReadOnly=true)
//   - concurrency_safe: 单个 method 调用可并发 (ConcurrencySafe=true)
//   - low_risk: 风险等级 LOW (无副作用)
type lspSurfaceInvariants struct {
	TypedMethod       string `invariant:"is_typed_method => typed_only"`
	ReadOnlyFlag      string `invariant:"read_only => no_destructive"`
	ConcurrencySafety string `invariant:"is_concurrent_safe => single_call_idempotent"`
	LowRisk           string `invariant:"low_risk => no_destructive"`
}

// lspSurfaceInvariantSet 是 LSPToolSurface 的 4 条 invariant 集合 (compile-time parsed)。
//
// 编译期: ltllite.ParseStruct 在 init 阶段执行, 任何 ErrInvalidInvariant 会让
// binary build 失败 (而非 silent 跳过)。这是 W15 设计的关键保证:
// "invariant 必须总是 valid, 否则视为严重 regression"。
var lspSurfaceInvariantSet = mustParseLSPInvariants()

func mustParseLSPInvariants() ltllite.InvariantSet {
	set, err := ltllite.ParseStruct(lspSurfaceInvariants{})
	if err != nil {
		panic("ltllite: LSPToolSurface invariant parse failed: " + err.Error())
	}
	return set
}

// CheckLSPInvariants 在 runtime 验证 LSPToolSurface 状态是否满足所有 invariant。
// 调用方传入 MapState (key = prop name, value = true/false)。
// 返回 violations 列表 (空 = 全部成立)。
func CheckLSPInvariants(state ltllite.State) []ltllite.Violation {
	return ltllite.Check(lspSurfaceInvariantSet, state)
}
